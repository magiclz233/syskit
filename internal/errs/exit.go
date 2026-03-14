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

type ExitError struct {
	CodeValue int
	Message   string
	Err       error
}

func (e *ExitError) Error() string {
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "command failed"
}

func (e *ExitError) Unwrap() error {
	return e.Err
}

func New(code int, message string) error {
	return &ExitError{
		CodeValue: code,
		Message:   message,
	}
}

func Wrap(code int, err error, message string) error {
	if err == nil {
		return nil
	}

	return &ExitError{
		CodeValue: code,
		Message:   message,
		Err:       err,
	}
}

func Code(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var exitErr *ExitError
	if errors.As(err, &exitErr) {
		return exitErr.CodeValue
	}

	return ExitExecutionFailed
}
