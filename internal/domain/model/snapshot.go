// Package model 定义 snapshot/report 等模块复用的数据模型。
package model

import "time"

// Snapshot 表示一次系统状态快照。
// P0 阶段先聚焦“可落盘、可查看、可对比”的最小结构，模块数据用 map 承载。
type Snapshot struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	Host        string         `json:"host"`
	Platform    string         `json:"platform"`
	Modules     map[string]any `json:"modules"`
	Warnings    []string       `json:"warnings,omitempty"`
}

// SnapshotSummary 是快照列表和删除预览使用的轻量结构。
type SnapshotSummary struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	Host        string    `json:"host"`
	Platform    string    `json:"platform"`
	Modules     []string  `json:"modules"`
	SizeBytes   int64     `json:"size_bytes"`
}
