package domain

import "fmt"

type AppError struct {
	Code    int
	Message string
}

func (e *AppError) Error() string {
	return e.Message
}

func ErrInvalidInput(msg string) *AppError {
	return &AppError{Code: 400, Message: msg}
}

func ErrNotFound(entity string) *AppError {
	return &AppError{Code: 404, Message: fmt.Sprintf("%s not found", entity)}
}

func ErrConflict(msg string) *AppError {
	return &AppError{Code: 409, Message: msg}
}

func ErrInternal(msg string) *AppError {
	return &AppError{Code: 500, Message: msg}
}
