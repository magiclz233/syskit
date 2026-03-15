//go:build !windows

package disk

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const statReadonlyFlag = 1

type unixMount struct {
	device     string
	mountPoint string
	fileSystem string
}

// collectPartitions 在非 Windows 平台采集挂载点容量信息。
func collectPartitions() ([]Partition, []string, error) {
	mounts, warnings := discoverUnixMounts()
	partitions := make([]Partition, 0, len(mounts))

	for _, mount := range mounts {
		var stat syscall.Statfs_t
		if err := syscall.Statfs(mount.mountPoint, &stat); err != nil {
			warnings = append(warnings, mount.mountPoint+" 读取容量失败: "+err.Error())
			continue
		}

		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bavail * uint64(stat.Bsize)
		if totalBytes == 0 {
			continue
		}

		usedBytes := totalBytes - freeBytes
		partitions = append(partitions, Partition{
			Device:       mount.device,
			MountPoint:   mount.mountPoint,
			FileSystem:   mount.fileSystem,
			TotalBytes:   totalBytes,
			UsedBytes:    usedBytes,
			FreeBytes:    freeBytes,
			UsagePercent: usagePercent(totalBytes, usedBytes),
			ReadOnly:     stat.Flags&statReadonlyFlag != 0,
		})
	}

	if len(partitions) == 0 {
		return nil, warnings, errors.New("未发现可读取的磁盘分区")
	}

	return partitions, warnings, nil
}

// discoverUnixMounts 从 /proc/self/mounts 读取挂载点；若不可用则回退到根分区。
func discoverUnixMounts() ([]unixMount, []string) {
	defaultMount := unixMount{
		device:     "/",
		mountPoint: "/",
		fileSystem: "",
	}

	file, err := os.Open("/proc/self/mounts")
	if err != nil {
		// 在 macOS 等没有 /proc 的平台上，至少保证能看到根分区信息。
		return []unixMount{defaultMount}, nil
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	mounts := make([]unixMount, 0, 16)
	seen := map[string]struct{}{
		"/": {},
	}
	mounts = append(mounts, defaultMount)

	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 3 {
			continue
		}

		device := fields[0]
		mountPoint := decodeMountPath(fields[1])
		fileSystem := strings.TrimSpace(fields[2])
		if mountPoint == "" || !filepath.IsAbs(mountPoint) {
			continue
		}
		if shouldSkipFileSystem(fileSystem) {
			continue
		}
		if _, ok := seen[mountPoint]; ok {
			continue
		}

		seen[mountPoint] = struct{}{}
		mounts = append(mounts, unixMount{
			device:     device,
			mountPoint: mountPoint,
			fileSystem: fileSystem,
		})
	}

	if scanErr := scanner.Err(); scanErr != nil {
		return mounts, []string{"/proc/self/mounts 读取失败: " + scanErr.Error()}
	}

	return mounts, nil
}

func decodeMountPath(raw string) string {
	replacer := strings.NewReplacer(
		"\\040", " ",
		"\\011", "\t",
		"\\012", "\n",
		"\\134", "\\",
	)
	return replacer.Replace(raw)
}

func shouldSkipFileSystem(fileSystem string) bool {
	switch fileSystem {
	case "proc", "sysfs", "devtmpfs", "devfs", "tmpfs", "cgroup", "cgroup2", "mqueue", "pstore", "securityfs", "debugfs", "tracefs", "fusectl", "autofs":
		return true
	default:
		return false
	}
}
