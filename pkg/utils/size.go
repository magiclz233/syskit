// Package utils 提供通用工具函数
package utils

import (
	"fmt"
	"strings"
)

// FormatBytes 将字节数格式化为人类可读的字符串
// 例如：1024 -> "1.00 KB", 1048576 -> "1.00 MB"
//
// Go 语言知识点：
// 1. 函数可以返回多个值（这里只返回一个）
// 2. const 定义常量
// 3. switch 语句不需要 break（Go 自动 break）
func FormatBytes(bytes int64) string {
	// 定义单位常量
	// Go 语言知识点：const 可以定义无类型常量，编译时计算
	const (
		B  = 1
		KB = 1024 * B
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	// 将 int64 转换为 float64 以便进行除法运算
	// Go 语言知识点：类型转换必须显式进行
	value := float64(bytes)

	// 根据大小选择合适的单位
	// Go 语言知识点：switch 可以不带表达式，相当于 if-else 链
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

// FormatNumber 格式化数字，添加千位分隔符
// 例如：1234567 -> "1,234,567"
//
// Go 语言知识点：
// 1. 字符串是不可变的
// 2. 使用 rune 处理 Unicode 字符
func FormatNumber(n int) string {
	// 将数字转换为字符串
	str := fmt.Sprintf("%d", n)

	// 如果数字小于 1000，直接返回
	if len(str) <= 3 {
		return str
	}

	// 从右向左每三位添加逗号
	// Go 语言知识点：使用 []byte 或 []rune 来构建字符串更高效
	result := ""
	for i, digit := range str {
		if i > 0 && (len(str)-i)%3 == 0 {
			result += ","
		}
		result += string(digit)
	}

	return result
}

// ParseSize 解析大小字符串（如 "100MB", "1GB"）为字节数
// 支持的单位：B, KB, MB, GB, TB（不区分大小写）
//
// Go 语言知识点：
// 1. 函数可以返回多个值（值和错误）
// 2. error 是 Go 的内置接口类型
func ParseSize(sizeStr string) (int64, error) {
	if sizeStr == "" {
		return 0, fmt.Errorf("大小字符串不能为空")
	}

	// 转换为大写以便统一处理
	sizeStr = strings.ToUpper(strings.TrimSpace(sizeStr))

	// 定义单位映射
	units := map[string]int64{
		"B":  1,
		"KB": 1024,
		"MB": 1024 * 1024,
		"GB": 1024 * 1024 * 1024,
		"TB": 1024 * 1024 * 1024 * 1024,
	}

	// 尝试匹配每个单位
	for unit, multiplier := range units {
		if strings.HasSuffix(sizeStr, unit) {
			// 提取数字部分
			numStr := strings.TrimSuffix(sizeStr, unit)
			numStr = strings.TrimSpace(numStr)

			// 解析数字（支持整数和浮点数）
			var value float64
			_, err := fmt.Sscanf(numStr, "%f", &value)
			if err != nil {
				return 0, fmt.Errorf("无效的数字: %s", numStr)
			}

			if value < 0 {
				return 0, fmt.Errorf("大小不能为负数")
			}

			return int64(value * float64(multiplier)), nil
		}
	}

	// 如果没有单位，尝试解析为纯数字（字节）
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
