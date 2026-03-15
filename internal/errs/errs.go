package errs

import "errors"

const (
	ExitSuccess          = 0
	ExitWarning          = 1
	ExitFailOnMatched    = 2
	ExitInvalidArgument  = 3
	ExitPermissionDenied = 4
	ExitExecutionFailed  = 5
	ExitPartialSuccess   = 6
)

const (
	CodeInvalidArgument     = "ERR_INVALID_ARGUMENT"
	CodePermissionDenied    = "ERR_PERMISSION_DENIED"
	CodePlatformUnsupported = "ERR_PLATFORM_UNSUPPORTED"
	CodeExecutionFailed     = "ERR_EXECUTION_FAILED"
	CodeConfigInvalid       = "ERR_CONFIG_INVALID"
	CodePolicyInvalid       = "ERR_POLICY_INVALID"
	CodeTimeout             = "ERR_TIMEOUT"
	CodeNotFound            = "ERR_NOT_FOUND"
	CodeAlreadyExists       = "ERR_ALREADY_EXISTS"
	CodeStorageFull         = "ERR_STORAGE_FULL"
	CodeDependencyMissing   = "ERR_DEPENDENCY_MISSING"
)

type CLIError struct {
	ExitCode   int
	ErrCode    string
	Message    string
	Suggestion string
	Err        error
}

func (e *CLIError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "command failed"
}

func (e *CLIError) Unwrap() error {
	return e.Err
}

func New(exitCode int, errCode string, message string) error {
	return &CLIError{
		ExitCode:   exitCode,
		ErrCode:    normalizeCode(exitCode, errCode),
		Message:    message,
		Suggestion: defaultSuggestion(exitCode, errCode),
	}
}

func NewWithSuggestion(exitCode int, errCode string, message string, suggestion string) error {
	return &CLIError{
		ExitCode:   exitCode,
		ErrCode:    normalizeCode(exitCode, errCode),
		Message:    message,
		Suggestion: suggestion,
	}
}

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

func InvalidArgument(message string) error {
	return New(ExitInvalidArgument, CodeInvalidArgument, message)
}

func PermissionDenied(message string, suggestion string) error {
	return NewWithSuggestion(ExitPermissionDenied, CodePermissionDenied, message, suggestion)
}

func ExecutionFailed(message string, err error) error {
	return Wrap(ExitExecutionFailed, CodeExecutionFailed, err, message)
}

func ConfigInvalid(message string, err error) error {
	if err == nil {
		return New(ExitInvalidArgument, CodeConfigInvalid, message)
	}
	return Wrap(ExitInvalidArgument, CodeConfigInvalid, err, message)
}

func PolicyInvalid(message string, err error) error {
	if err == nil {
		return New(ExitInvalidArgument, CodePolicyInvalid, message)
	}
	return Wrap(ExitInvalidArgument, CodePolicyInvalid, err, message)
}

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
