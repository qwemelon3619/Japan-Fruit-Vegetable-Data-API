package xerror

import "fmt"

type Code string

const (
	CodeInvalidArgument Code = "INVALID_ARGUMENT"
	CodeInternal        Code = "INTERNAL"
	CodeDB              Code = "DB_ERROR"
)

type AppError struct {
	Code    Code
	Message string
	Err     error
}

func (e *AppError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("[%s] %s", e.Code, e.Message)
	}
	return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
}

func (e *AppError) Unwrap() error { return e.Err }

func New(code Code, message string) error {
	return &AppError{Code: code, Message: message}
}

func Wrap(code Code, message string, err error) error {
	return &AppError{Code: code, Message: message, Err: err}
}
