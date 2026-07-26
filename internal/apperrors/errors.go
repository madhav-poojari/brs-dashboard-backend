package apperrors

import "fmt"

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	return fmt.Sprintf("Error occured with code %d: %v", e.Code, e.Message)
}

func NewAppError(code int, message string) *AppError {
	return &AppError{
		Code:    code,
		Message: message,
	}
}
