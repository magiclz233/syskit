package policy

import (
	"os"
	"strings"
	"syskit/internal/errs"

	"gopkg.in/yaml.v3"
)

type LoadOptions struct {
	ExplicitPath string
}

type LoadResult struct {
	Policy *Policy
	Paths  []string
}

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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
