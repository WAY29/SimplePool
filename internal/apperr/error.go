package apperr

import "fmt"

type Code string

const (
	CodeConfig   Code = "config_invalid"
	CodeRuntime  Code = "runtime_error"
	CodeSecurity Code = "security_error"
	CodeStore    Code = "store_error"
	CodeInternal Code = "internal_error"
)

type Error struct {
	Code    Code
	Op      string
	Message string
	Err     error
}

func (e *Error) Error() string {
	switch {
	case e == nil:
		return ""
	case e.Message != "" && e.Op != "":
		return fmt.Sprintf("%s: %s", e.Op, e.Message)
	case e.Message != "":
		return e.Message
	case e.Op != "" && e.Err != nil:
		return fmt.Sprintf("%s: %v", e.Op, e.Err)
	case e.Err != nil:
		return e.Err.Error()
	default:
		return string(e.Code)
	}
}

func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Err
}

func Wrap(code Code, op string, err error) error {
	if err == nil {
		return nil
	}

	return &Error{
		Code: code,
		Op:   op,
		Err:  err,
	}
}

func New(code Code, op, message string) error {
	return &Error{
		Code:    code,
		Op:      op,
		Message: message,
	}
}
