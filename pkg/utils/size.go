// Package utils 提供和扫描流程无关的通用辅助函数。
// 这些函数主要服务于结果展示和参数解析。
package utils

import (
	"fmt"
	"strings"
)

// FormatBytes 把原始字节数格式化成人类更容易阅读的形式。
// 例如：
//
//	1024      -> 1.00 KB
//	1048576   -> 1.00 MB
//	1073741824 -> 1.00 GB
func FormatBytes(bytes int64) string {
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	// 转成浮点数后再做除法，避免整数除法丢失小数部分。
	value := float64(bytes)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", value/TB)
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", value/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", value/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", value/KB)
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

// FormatNumber 给整数添加千分位分隔符，方便终端阅读大数字。
// 例如：
//
//	1234567 -> 1,234,567
func FormatNumber(n int) string {
	str := fmt.Sprintf("%d", n)
	if len(str) <= 3 {
		return str
	}

	// 这里从左到右扫描字符串。
	// 每当剩余位数是 3 的整数倍时，就在当前位置前插入一个逗号。
	result := ""
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(digit)
	}

	return result
}

// ParseSize 把用户输入的大小字符串解析成字节数。
// 支持：
// - 纯数字，例如 1024
// - 带单位，例如 100MB、1.5GB
// 单位不区分大小写。
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("大小字符串不能为空")
	}

	// 先清理输入，避免大小写和前后空格影响后续匹配。
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// 单位必须按“从长到短”的顺序匹配。
	// 否则像 "100MB" 这样的输入，有可能先命中 "B"，把数字部分错误地截成 "100M"。
	units := []struct {
		suffix     string
		multiplier int64
	}{
		{suffix: "TB", multiplier: 1024 * 1024 * 1024 * 1024},
		{suffix: "GB", multiplier: 1024 * 1024 * 1024},
		{suffix: "MB", multiplier: 1024 * 1024},
		{suffix: "KB", multiplier: 1024},
		{suffix: "B", multiplier: 1},
	}

	for _, unit := range units {
		if strings.HasSuffix(sizeStr, unit.suffix) {
			numStr := strings.TrimSpace(strings.TrimSuffix(sizeStr, unit.suffix))

			var value float64
			_, err := fmt.Sscanf(numStr, "%f", &value)
			if err != nil {
				return 0, fmt.Errorf("无效的数字: %s", numStr)
			}

			if value < 0 {
				return 0, fmt.Errorf("大小不能为负数")
			}

			return int64(value * float64(unit.multiplier)), nil
		}
	}

	// 如果没有单位，就按“字节数”理解。
	var value int64
	_, err := fmt.Sscanf(sizeStr, "%d", &value)
	if err != nil {
		return 0, fmt.Errorf("无效的大小格式: %s（支持格式：100MB, 1GB, 1024）", sizeStr)
	}

	if value < 0 {
		return 0, fmt.Errorf("大小不能为负数")
	}

	return value, nil
}
