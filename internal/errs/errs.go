// Package errs 提供 CLI 统一错误协议。
// 这个包的目标不是简单包一层 error，而是把“退出码、错误码、对用户的提示”绑定在一起，
// 这样所有命令都能输出一致的错误结构和一致的进程退出行为。
package errs

import "errors"

const (
	// ExitSuccess 表示命令完全成功。
	ExitSuccess = 0
	// ExitWarning 预留给未来“成功但存在告警”的场景。
	ExitWarning = 1
	// ExitFailOnMatched 表示命中了 --fail-on 阈值，供后续 doctor/CI 使用。
	ExitFailOnMatched = 2
	// ExitInvalidArgument 表示参数、配置或策略不合法。
	ExitInvalidArgument = 3
	// ExitPermissionDenied 表示权限不足。
	ExitPermissionDenied = 4
	// ExitExecutionFailed 表示命令执行失败，但不属于参数错误或权限错误。
	ExitExecutionFailed = 5
	// ExitPartialSuccess 预留给部分成功、部分跳过的场景。
	ExitPartialSuccess = 6
)

const (
	// CodeInvalidArgument 对应参数或输入协议错误。
	CodeInvalidArgument = "ERR_INVALID_ARGUMENT"
	// CodePermissionDenied 对应权限不足。
	CodePermissionDenied = "ERR_PERMISSION_DENIED"
	// CodePlatformUnsupported 对应平台不支持。
	CodePlatformUnsupported = "ERR_PLATFORM_UNSUPPORTED"
	// CodeExecutionFailed 对应通用执行失败。
	CodeExecutionFailed = "ERR_EXECUTION_FAILED"
	// CodeConfigInvalid 对应配置文件无效。
	CodeConfigInvalid = "ERR_CONFIG_INVALID"
	// CodePolicyInvalid 对应策略文件无效。
	CodePolicyInvalid = "ERR_POLICY_INVALID"
	// CodeTimeout 对应超时。
	CodeTimeout = "ERR_TIMEOUT"
	// CodeNotFound 对应对象不存在。
	CodeNotFound = "ERR_NOT_FOUND"
	// CodeAlreadyExists 对应资源已存在。
	CodeAlreadyExists = "ERR_ALREADY_EXISTS"
	// CodeStorageFull 对应存储空间不足。
	CodeStorageFull = "ERR_STORAGE_FULL"
	// CodeDependencyMissing 对应外部依赖缺失。
	CodeDependencyMissing = "ERR_DEPENDENCY_MISSING"
)

// CLIError 是 syskit 内部统一使用的错误载体。
// 它同时保存“进程退出码”和“结构化错误码”，避免调用方重复做映射。
type CLIError struct {
	// ExitCode 是进程最终退出码，供 main 包直接 os.Exit 使用。
	ExitCode int
	// ErrCode 是结构化错误码，供 JSON/Markdown/plain 输出使用。
	ErrCode string
	// Message 是直接面向用户的错误信息。
	Message string
	// Suggestion 是可选修复建议，用于帮助用户快速定位下一步动作。
	Suggestion string
	// Err 保存底层原始错误，便于 errors.Is / errors.As 继续工作。
	Err error
}

// Error 实现 error 接口。
// 优先返回面向用户的 Message；如果 Message 为空，则退回底层错误文本。
func (e *CLIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "command failed"
}

// Unwrap 允许上层继续通过 errors.Is / errors.As 识别底层错误。
func (e *CLIError) Unwrap() error {
	return e.Err
}

// New 创建一个不携带底层 error 的 CLIError。
// 适合“已经知道错误类型，不需要保留底层调用栈”的场景。
func New(exitCode int, errCode string, message string) error {
	return &CLIError{
		ExitCode:   exitCode,
		ErrCode:    normalizeCode(exitCode, errCode),
		Message:    message,
		Suggestion: defaultSuggestion(exitCode, errCode),
	}
}

// NewWithSuggestion 在 New 的基础上允许显式指定建议文本。
func NewWithSuggestion(exitCode int, errCode string, message string, suggestion string) error {
	return &CLIError{
		ExitCode:   exitCode,
		ErrCode:    normalizeCode(exitCode, errCode),
		Message:    message,
		Suggestion: suggestion,
	}
}

// Wrap 用已有 error 包装出 CLIError。
// 当底层 err 为 nil 时直接返回 nil，方便调用方少写一层判断。
func Wrap(exitCode int, errCode string, err error, message string) error {
	if err == nil {
		return nil
	}

	return &CLIError{
		ExitCode:   exitCode,
		ErrCode:    normalizeCode(exitCode, errCode),
		Message:    message,
		Suggestion: defaultSuggestion(exitCode, errCode),
		Err:        err,
	}
}

// WrapWithSuggestion 是带建议文本的 Wrap 版本。
func WrapWithSuggestion(exitCode int, errCode string, err error, message string, suggestion string) error {
	if err == nil {
		return nil
	}

	return &CLIError{
		ExitCode:   exitCode,
		ErrCode:    normalizeCode(exitCode, errCode),
		Message:    message,
		Suggestion: suggestion,
		Err:        err,
	}
}

// InvalidArgument 快速构造“参数无效”错误。
func InvalidArgument(message string) error {
	return New(ExitInvalidArgument, CodeInvalidArgument, message)
}

// PermissionDenied 快速构造“权限不足”错误。
func PermissionDenied(message string, suggestion string) error {
	return NewWithSuggestion(ExitPermissionDenied, CodePermissionDenied, message, suggestion)
}

// ExecutionFailed 快速构造“执行失败”错误，并保留底层 error。
func ExecutionFailed(message string, err error) error {
	return Wrap(ExitExecutionFailed, CodeExecutionFailed, err, message)
}

// ConfigInvalid 快速构造“配置无效”错误。
// 这里要兼容没有底层 err 的场景，所以不能简单依赖 Wrap。
func ConfigInvalid(message string, err error) error {
	if err == nil {
		return New(ExitInvalidArgument, CodeConfigInvalid, message)
	}
	return Wrap(ExitInvalidArgument, CodeConfigInvalid, err, message)
}

// PolicyInvalid 快速构造“策略无效”错误。
func PolicyInvalid(message string, err error) error {
	if err == nil {
		return New(ExitInvalidArgument, CodePolicyInvalid, message)
	}
	return Wrap(ExitInvalidArgument, CodePolicyInvalid, err, message)
}

// Code 返回 error 对应的进程退出码。
// 对于非 CLIError，默认按通用执行失败处理。
func Code(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.ExitCode
	}

	return ExitExecutionFailed
}

// ErrorCode 返回结构化错误码。
func ErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return normalizeCode(cliErr.ExitCode, cliErr.ErrCode)
	}

	return CodeExecutionFailed
}

// Message 返回最终面向用户的错误文案。
func Message(err error) string {
	if err == nil {
		return ""
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) && cliErr.Message != "" {
		return cliErr.Message
	}

	return err.Error()
}

// Suggestion 返回建议文案。
// 如果原始错误没有建议，则按错误码推导默认建议。
func Suggestion(err error) string {
	if err == nil {
		return ""
	}

	var cliErr *CLIError
	if errors.As(err, &cliErr) {
		return cliErr.Suggestion
	}

	return defaultSuggestion(Code(err), ErrorCode(err))
}

// normalizeCode 保证每个 CLIError 至少有一个稳定的结构化错误码。
func normalizeCode(exitCode int, errCode string) string {
	if errCode != "" {
		return errCode
	}

	switch exitCode {
	case ExitInvalidArgument:
		return CodeInvalidArgument
	case ExitPermissionDenied:
		return CodePermissionDenied
	default:
		return CodeExecutionFailed
	}
}

// defaultSuggestion 为常见错误码提供默认建议。
// 这样即便调用方没有显式传 suggestion，结构化输出也不会完全缺少下一步提示。
func defaultSuggestion(exitCode int, errCode string) string {
	switch normalizeCode(exitCode, errCode) {
	case CodeInvalidArgument:
		return "请使用 --help 查看正确用法"
	case CodePermissionDenied:
		return "请提升权限后重试"
	case CodeConfigInvalid:
		return "请检查配置文件格式和字段取值"
	case CodePolicyInvalid:
		return "请检查策略文件格式和字段取值"
	case CodeTimeout:
		return "请调大 --timeout 后重试"
	default:
		return ""
	}
}
