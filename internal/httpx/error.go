package httpx

import (
	"errors"
	"fmt"
)

// Issue kinds used in the `errors[].issue` field.
const (
	IssueRequired   = "REQUIRED"
	IssueInvalid    = "INVALID"
	IssueDuplicate  = "DUPLICATE"
	IssueNotFound   = "NOT_FOUND"
	IssueXOR        = "XOR"
	IssueTooLong    = "TOO_LONG"
	IssueOutOfRange = "OUT_OF_RANGE"
)

// FieldError is one entry of the `errors[]` array.
type FieldError struct {
	Field   string `json:"field"`
	Issue   string `json:"issue"`
	Message string `json:"message"`
}

// AppError is the single error type that travels from the repository up to the
// handler. Everything the envelope needs is attached to it.
type AppError struct {
	Code    ErrorCode
	Message string
	Fields  []FieldError
	// Err is the underlying cause. It is logged, never serialised.
	Err error
}

func (e *AppError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = e.Code.Message
	}
	if e.Err != nil {
		return fmt.Sprintf("%s (%s): %v", msg, e.Code.Code, e.Err)
	}
	return fmt.Sprintf("%s (%s)", msg, e.Code.Code)
}

func (e *AppError) Unwrap() error { return e.Err }

// ResolvedMessage returns the override message when set, otherwise the default
// message of the code.
func (e *AppError) ResolvedMessage() string {
	if e.Message != "" {
		return e.Message
	}
	return e.Code.Message
}

// Err builds an AppError from a code.
func Err(code ErrorCode) *AppError {
	return &AppError{Code: code}
}

// Errf builds an AppError with a formatted message overriding the default one.
func Errf(code ErrorCode, format string, args ...any) *AppError {
	return &AppError{Code: code, Message: fmt.Sprintf(format, args...)}
}

// WithField appends a field-level detail.
func (e *AppError) WithField(field, issue, message string) *AppError {
	e.Fields = append(e.Fields, FieldError{Field: field, Issue: issue, Message: message})
	return e
}

// WithFields appends several field-level details at once.
func (e *AppError) WithFields(fields ...FieldError) *AppError {
	e.Fields = append(e.Fields, fields...)
	return e
}

// WithCause attaches the underlying error for logging.
func (e *AppError) WithCause(err error) *AppError {
	e.Err = err
	return e
}

// AsAppError extracts an *AppError from an error chain, falling back to
// E500_001 so a handler always has something to send.
func AsAppError(err error) *AppError {
	var appErr *AppError
	if errors.As(err, &appErr) {
		return appErr
	}
	return &AppError{Code: ErrInternal, Err: err}
}
