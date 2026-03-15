// Package policy 也负责策略文件的加载和自动发现。
package policy

import (
	"os"
	"strings"
	"syskit/internal/errs"

	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	// ExplicitPath 表示命令行显式指定的策略文件路径。
	ExplicitPath string
}

// LoadResult 返回最终生效策略以及本次参与合并的路径。
type LoadResult struct {
	Policy *Policy
	Paths  []string
}

// Load 按“默认模板 -> 显式路径/环境变量 -> 系统级 -> 用户级”的顺序构建最终策略。
func Load(opts LoadOptions) (*LoadResult, error) {
	cfg := DefaultPolicy()
	paths := make([]string, 0, 2)

	explicitPath := strings.TrimSpace(opts.ExplicitPath)
	envPolicyPath := strings.TrimSpace(os.Getenv("SYSKIT_POLICY"))

	switch {
	case explicitPath != "":
		if err := mergeFile(cfg, explicitPath, true); err != nil {
			return nil, err
		}
		paths = append(paths, explicitPath)
	case envPolicyPath != "":
		if err := mergeFile(cfg, envPolicyPath, true); err != nil {
			return nil, err
		}
		paths = append(paths, envPolicyPath)
	default:
		systemPath := SystemPolicyPath()
		if err := mergeFile(cfg, systemPath, false); err != nil {
			return nil, err
		}
		if fileExists(systemPath) {
			paths = append(paths, systemPath)
		}

		userPath := UserPolicyPath()
		if err := mergeFile(cfg, userPath, false); err != nil {
			return nil, err
		}
		if fileExists(userPath) {
			paths = append(paths, userPath)
		}
	}

	if err := Validate(cfg); err != nil {
		return nil, err
	}

	return &LoadResult{
		Policy: cfg,
		Paths:  paths,
	}, nil
}

// mergeFile 把单个策略 YAML 文件叠加到现有策略对象上。
func mergeFile(cfg *Policy, path string, required bool) error {
	if !fileExists(path) {
		if required {
			return errs.PolicyInvalid("策略文件不存在: "+path, os.ErrNotExist)
		}
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return errs.PolicyInvalid("读取策略文件失败: "+path, err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return errs.PolicyInvalid("解析策略文件失败: "+path, err)
	}

	return nil
}

// fileExists 用于判断某一层策略文件是否存在。
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
