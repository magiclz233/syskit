//go:build windows

package disk

import (
	"errors"
	"fmt"
	"syscall"
	"unsafe"
)

const (
	driveTypeUnknown   = 0
	driveTypeNoRootDir = 1
	driveTypeCDROM     = 5

	fileReadOnlyVolume = 0x00080000
	volumeNameMaxLen   = 261
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procGetLogicalDrives     = kernel32.NewProc("GetLogicalDrives")
	procGetDriveTypeW        = kernel32.NewProc("GetDriveTypeW")
	procGetDiskFreeSpaceExW  = kernel32.NewProc("GetDiskFreeSpaceExW")
	procGetVolumeInformation = kernel32.NewProc("GetVolumeInformationW")
)

// collectPartitions 在 Windows 上采集各盘符容量信息。
func collectPartitions() ([]Partition, []string, error) {
	driveMask, err := getLogicalDrives()
	if err != nil {
		return nil, nil, err
	}

	partitions := make([]Partition, 0, 8)
	warnings := make([]string, 0)

	for i := 0; i < 26; i++ {
		if driveMask&(1<<uint(i)) == 0 {
			continue
		}

		mountPoint := fmt.Sprintf("%c:\\", rune('A'+i))

		driveType, driveTypeErr := getDriveType(mountPoint)
		if driveTypeErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s 获取驱动器类型失败: %v", mountPoint, driveTypeErr))
			continue
		}
		if driveType == driveTypeNoRootDir {
			continue
		}
		if driveType == driveTypeUnknown {
			warnings = append(warnings, fmt.Sprintf("%s 驱动器类型未知，已跳过", mountPoint))
			continue
		}

		totalBytes, freeBytes, diskErr := getDiskFreeSpace(mountPoint)
		if diskErr != nil {
			warnings = append(warnings, fmt.Sprintf("%s 读取容量失败: %v", mountPoint, diskErr))
			continue
		}
		if totalBytes == 0 {
			continue
		}

		volumeName, fileSystem, readOnly := getVolumeInformation(mountPoint)
		usedBytes := totalBytes - freeBytes
		partitions = append(partitions, Partition{
			Device:       mountPoint,
			VolumeName:   volumeName,
			MountPoint:   mountPoint,
			FileSystem:   fileSystem,
			TotalBytes:   totalBytes,
			UsedBytes:    usedBytes,
			FreeBytes:    freeBytes,
			UsagePercent: usagePercent(totalBytes, usedBytes),
			ReadOnly:     readOnly || driveType == driveTypeCDROM,
		})
	}

	if len(partitions) == 0 {
		return nil, warnings, errors.New("未发现可读取的磁盘分区")
	}

	return partitions, warnings, nil
}

func getLogicalDrives() (uint32, error) {
	ret, _, callErr := procGetLogicalDrives.Call()
	if ret == 0 {
		return 0, normalizeSyscallErr("调用 GetLogicalDrives 失败", callErr)
	}
	return uint32(ret), nil
}

func getDriveType(root string) (uint32, error) {
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, err
	}

	ret, _, callErr := procGetDriveTypeW.Call(uintptr(unsafe.Pointer(rootPtr)))
	if ret == 0 {
		return 0, normalizeSyscallErr("调用 GetDriveTypeW 失败", callErr)
	}

	return uint32(ret), nil
}

func getDiskFreeSpace(root string) (uint64, uint64, error) {
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return 0, 0, err
	}

	var freeBytesAvailable uint64
	var totalNumberOfBytes uint64
	var totalNumberOfFreeBytes uint64

	ret, _, callErr := procGetDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalNumberOfBytes)),
		uintptr(unsafe.Pointer(&totalNumberOfFreeBytes)),
	)
	if ret == 0 {
		return 0, 0, normalizeSyscallErr("调用 GetDiskFreeSpaceExW 失败", callErr)
	}

	return totalNumberOfBytes, totalNumberOfFreeBytes, nil
}

func getVolumeInformation(root string) (string, string, bool) {
	rootPtr, err := syscall.UTF16PtrFromString(root)
	if err != nil {
		return "", "", false
	}

	volumeNameBuffer := make([]uint16, volumeNameMaxLen)
	fileSystemNameBuffer := make([]uint16, volumeNameMaxLen)
	var serialNumber uint32
	var maxComponentLength uint32
	var fileSystemFlags uint32

	ret, _, _ := procGetVolumeInformation.Call(
		uintptr(unsafe.Pointer(rootPtr)),
		uintptr(unsafe.Pointer(&volumeNameBuffer[0])),
		uintptr(volumeNameMaxLen),
		uintptr(unsafe.Pointer(&serialNumber)),
		uintptr(unsafe.Pointer(&maxComponentLength)),
		uintptr(unsafe.Pointer(&fileSystemFlags)),
		uintptr(unsafe.Pointer(&fileSystemNameBuffer[0])),
		uintptr(volumeNameMaxLen),
	)
	if ret == 0 {
		return "", "", false
	}

	return syscall.UTF16ToString(volumeNameBuffer), syscall.UTF16ToString(fileSystemNameBuffer), fileSystemFlags&fileReadOnlyVolume != 0
}

func normalizeSyscallErr(fallback string, err error) error {
	errno, ok := err.(syscall.Errno)
	if ok && errno != 0 {
		return errno
	}

	return errors.New(fallback)
}
