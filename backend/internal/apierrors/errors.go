// Package apierrors provides structured API error handling.
package apierrors

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/finopsmind/backend/internal/correlation"
)

// APIError represents a structured API error.
type APIError struct {
	Code       string `json:"code"`
	Message    string `json:"message"`
	StatusCode int    `json:"-"`
	Details    any    `json:"details,omitempty"`
	RequestID  string `json:"request_id,omitempty"`
}

func (e *APIError) Error() string {
	return e.Message
}

// Write writes the error response.
func (e *APIError) Write(w http.ResponseWriter, r *http.Request) {
	e.RequestID = correlation.GetID(r.Context())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(e)
}

// Common errors

func NewBadRequestError(message string) *APIError {
	return &APIError{
		Code:       "BAD_REQUEST",
		Message:    message,
		StatusCode: http.StatusBadRequest,
	}
}

func NewUnauthorizedError(message string) *APIError {
	return &APIError{
		Code:       "UNAUTHORIZED",
		Message:    message,
		StatusCode: http.StatusUnauthorized,
	}
}

func NewForbiddenError(message string) *APIError {
	return &APIError{
		Code:       "FORBIDDEN",
		Message:    message,
		StatusCode: http.StatusForbidden,
	}
}

func NewNotFoundError(resource, id string) *APIError {
	return &APIError{
		Code:       "NOT_FOUND",
		Message:    resource + " not found",
		StatusCode: http.StatusNotFound,
		Details:    map[string]string{"resource": resource, "id": id},
	}
}

func NewConflictError(message string) *APIError {
	return &APIError{
		Code:       "CONFLICT",
		Message:    message,
		StatusCode: http.StatusConflict,
	}
}

func NewValidationError(message string, details any) *APIError {
	return &APIError{
		Code:       "VALIDATION_ERROR",
		Message:    message,
		StatusCode: http.StatusUnprocessableEntity,
		Details:    details,
	}
}

func NewInternalError(message string) *APIError {
	return &APIError{
		Code:       "INTERNAL_ERROR",
		Message:    message,
		StatusCode: http.StatusInternalServerError,
	}
}

func NewServiceUnavailableError(service string) *APIError {
	return &APIError{
		Code:       "SERVICE_UNAVAILABLE",
		Message:    service + " is temporarily unavailable",
		StatusCode: http.StatusServiceUnavailable,
	}
}

func NewRateLimitError() *APIError {
	return &APIError{
		Code:       "RATE_LIMIT_EXCEEDED",
		Message:    "Rate limit exceeded, please try again later",
		StatusCode: http.StatusTooManyRequests,
	}
}

// FromError converts a standard error to an APIError.
func FromError(err error) *APIError {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr
	}

	return NewInternalError("An unexpected error occurred")
}

// ErrorHandler is middleware that handles panics and errors.
func ErrorHandler(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				NewInternalError("Internal server error").Write(w, r)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
