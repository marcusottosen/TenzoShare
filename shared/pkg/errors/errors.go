// Package errors defines typed application errors used across all services.
// Each error has a machine-readable Code, an HTTP Status, and a human Message.
// Use errors.As to unwrap AppError from any error chain.
package errors

import (
	"errors"
	"fmt"
	"net/http"
)

// Code is a machine-readable error identifier included in API responses.
type Code string

const (
	CodeNotFound     Code = "NOT_FOUND"
	CodeUnauthorized Code = "UNAUTHORIZED"
	CodeForbidden    Code = "FORBIDDEN"
	CodeValidation   Code = "VALIDATION_ERROR"
	CodeConflict     Code = "CONFLICT"
	CodeInternal     Code = "INTERNAL_ERROR"
	CodeRateLimit    Code = "RATE_LIMIT_EXCEEDED"
	CodeBadRequest   Code = "BAD_REQUEST"
)

// AppError is a structured application-level error.
type AppError struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"status,omitempty"`
	cause   error
}

func (e *AppError) Error() string {
	if e.cause != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.cause)
	}
	return e.Message
}

// Unwrap enables errors.Is/As to inspect the underlying cause.
func (e *AppError) Unwrap() error { return e.cause }

// New constructs an AppError directly.
func New(code Code, status int, message string, cause error) *AppError {
	return &AppError{Code: code, Status: status, Message: message, cause: cause}
}

// ── Constructors ──────────────────────────────────────────────────────────────

func NotFound(msg string) *AppError {
	return New(CodeNotFound, http.StatusNotFound, msg, nil)
}

func Unauthorized(msg string) *AppError {
	return New(CodeUnauthorized, http.StatusUnauthorized, msg, nil)
}

func Forbidden(msg string) *AppError {
	return New(CodeForbidden, http.StatusForbidden, msg, nil)
}

func Validation(msg string) *AppError {
	return New(CodeValidation, http.StatusBadRequest, msg, nil)
}

func Conflict(msg string) *AppError {
	return New(CodeConflict, http.StatusConflict, msg, nil)
}

func Internal(msg string, cause error) *AppError {
	return New(CodeInternal, http.StatusInternalServerError, msg, cause)
}

func RateLimit(msg string) *AppError {
	return New(CodeRateLimit, http.StatusTooManyRequests, msg, nil)
}

func BadRequest(msg string) *AppError {
	return New(CodeBadRequest, http.StatusBadRequest, msg, nil)
}

// ── Sentinel checks ───────────────────────────────────────────────────────────

func IsNotFound(err error) bool     { return hasCode(err, CodeNotFound) }
func IsUnauthorized(err error) bool { return hasCode(err, CodeUnauthorized) }
func IsForbidden(err error) bool    { return hasCode(err, CodeForbidden) }
func IsConflict(err error) bool     { return hasCode(err, CodeConflict) }

func hasCode(err error, code Code) bool {
	var e *AppError
	return errors.As(err, &e) && e.Code == code
}
