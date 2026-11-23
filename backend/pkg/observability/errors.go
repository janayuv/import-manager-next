package observability

import (
	"fmt"
)

// ErrorCode defines strongly-typed, standard application error categories.
type ErrorCode string

const (
	ErrInvalidInput       ErrorCode = "INVALID_INPUT"
	ErrNotFound           ErrorCode = "NOT_FOUND"
	ErrUnauthorized       ErrorCode = "UNAUTHORIZED"
	ErrForbidden          ErrorCode = "FORBIDDEN"
	ErrConflict           ErrorCode = "CONFLICT"
	ErrDatabaseFailure    ErrorCode = "DATABASE_FAILURE"
	ErrCalculationError   ErrorCode = "CALCULATION_ERROR"
	ErrIntegrityViolation ErrorCode = "INTEGRITY_VIOLATION"
	ErrTimeout            ErrorCode = "TIMEOUT"
	ErrInternal           ErrorCode = "INTERNAL_ERROR"
)

// AppError is an architectural error container separating safe user presentation from internal root causes.
type AppError struct {
	Code     ErrorCode      `json:"code"`
	Message  string         `json:"message"`  // Safe presentation string
	Internal string         `json:"internal"` // Technical diagnostic info
	Cause    error          `json:"-"`        // Underlying wrapped error
	Fields   map[string]any `json:"fields,omitempty"`
}

// Error implements the standard error interface.
func (e *AppError) Error() string {
	if e.Internal != "" {
		return fmt.Sprintf("[%s] %s: %s", e.Code, e.Message, e.Internal)
	}
	if e.Cause != nil {
		return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Cause)
	}
	return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// Unwrap supports Go 1.13+ error unwrapping.
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError constructs a typed application error.
func NewAppError(code ErrorCode, userMessage string, internalDetail string, cause error) *AppError {
	return &AppError{
		Code:     code,
		Message:  userMessage,
		Internal: internalDetail,
		Cause:    cause,
		Fields:   make(map[string]any),
	}
}

// WithField adds contextual diagnostic data to the error.
func (e *AppError) WithField(key string, value any) *AppError {
	if e.Fields == nil {
		e.Fields = make(map[string]any)
	}
	e.Fields[key] = value
	return e
}
