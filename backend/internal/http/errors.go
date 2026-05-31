package http

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"runtime"
	"strings"
)

// ErrorCode represents standardized error codes
type ErrorCode string

const (
	// Authentication errors
	ErrUnauthorized      ErrorCode = "unauthorized"
	ErrForbidden         ErrorCode = "forbidden"
	ErrInvalidToken      ErrorCode = "invalid_token"
	ErrTokenExpired      ErrorCode = "token_expired"
	ErrInvalidPassword   ErrorCode = "invalid_password"
	ErrAccountLocked     ErrorCode = "account_locked"
	ErrTwoFactorRequired ErrorCode = "2fa_required"
	ErrInvalid2FACode    ErrorCode = "invalid_2fa_code"

	// Validation errors
	ErrInvalidInput  ErrorCode = "invalid_input"
	ErrMissingField  ErrorCode = "missing_field"
	ErrInvalidFormat ErrorCode = "invalid_format"
	ErrTooLong       ErrorCode = "too_long"
	ErrTooShort      ErrorCode = "too_short"
	ErrInvalidEmail  ErrorCode = "invalid_email"
	ErrInvalidUUID   ErrorCode = "invalid_uuid"
	ErrInvalidJSON   ErrorCode = "invalid_json"

	// Resource errors
	ErrNotFound      ErrorCode = "not_found"
	ErrAlreadyExists ErrorCode = "already_exists"
	ErrConflict      ErrorCode = "conflict"
	ErrResourceLimit ErrorCode = "resource_limit"

	// System errors
	ErrInternal           ErrorCode = "internal"
	ErrDatabase           ErrorCode = "database"
	ErrNetwork            ErrorCode = "network"
	ErrTimeout            ErrorCode = "timeout"
	ErrRateLimited        ErrorCode = "rate_limited"
	ErrServiceUnavailable ErrorCode = "service_unavailable"

	// Business logic errors
	ErrChatFull                ErrorCode = "chat_full"
	ErrCannotDeleteOwner       ErrorCode = "cannot_delete_owner"
	ErrInsufficientPermissions ErrorCode = "insufficient_permissions"
	ErrMessageTooLarge         ErrorCode = "message_too_large"
	ErrFileTypeNotAllowed      ErrorCode = "file_type_not_allowed"
)

// AppError represents an application error with context
type AppError struct {
	Code       ErrorCode              `json:"code"`
	Message    string                 `json:"message"`
	Details    map[string]interface{} `json:"details,omitempty"`
	StatusCode int                    `json:"-"`
	Cause      error                  `json:"-"`
	StackTrace string                 `json:"stack_trace,omitempty"`
}

// Error implements the error interface
func (e *AppError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("%s: %s", e.Code, e.Message)
	}
	return string(e.Code)
}

// Unwrap returns the underlying cause
func (e *AppError) Unwrap() error {
	return e.Cause
}

// NewAppError creates a new application error
func NewAppError(code ErrorCode, message string, statusCode int) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
	}
}

// NewAppErrorWithCause creates a new application error with cause
func NewAppErrorWithCause(code ErrorCode, message string, statusCode int, cause error) *AppError {
	return &AppError{
		Code:       code,
		Message:    message,
		StatusCode: statusCode,
		Cause:      cause,
	}
}

// WithDetails adds details to the error
func (e *AppError) WithDetails(key string, value interface{}) *AppError {
	if e.Details == nil {
		e.Details = make(map[string]interface{})
	}
	e.Details[key] = value
	return e
}

// WithStackTrace adds stack trace to the error (for debugging)
func (e *AppError) WithStackTrace() *AppError {
	buf := make([]byte, 1024)
	for {
		n := runtime.Stack(buf, false)
		if n < len(buf) {
			e.StackTrace = string(buf[:n])
			break
		}
		buf = make([]byte, 2*len(buf))
	}
	return e
}

// ErrorHandler is a centralized error handler
type ErrorHandler struct {
	logger *slog.Logger
}

// NewErrorHandler creates a new error handler
func NewErrorHandler(logger *slog.Logger) *ErrorHandler {
	return &ErrorHandler{logger: logger}
}

// HandleError handles an error and writes appropriate HTTP response
func (h *ErrorHandler) HandleError(w http.ResponseWriter, err error) {
	var appErr *AppError

	// Check if it's already an AppError
	if errors.As(err, &appErr) {
		h.logError(appErr)
		h.writeErrorResponse(w, appErr)
		return
	}

	// Convert to AppError based on error type
	appErr = h.classifyError(err)
	h.logError(appErr)
	h.writeErrorResponse(w, appErr)
}

// classifyError converts generic errors to AppError
func (h *ErrorHandler) classifyError(err error) *AppError {
	errMsg := err.Error()

	// Database errors
	if strings.Contains(errMsg, "database") || strings.Contains(errMsg, "sql") {
		return NewAppErrorWithCause(ErrDatabase, "Database operation failed", http.StatusInternalServerError, err)
	}

	// Network errors
	if strings.Contains(errMsg, "connection") || strings.Contains(errMsg, "timeout") {
		return NewAppErrorWithCause(ErrNetwork, "Network error occurred", http.StatusServiceUnavailable, err)
	}

	// JSON errors
	if strings.Contains(errMsg, "json") || strings.Contains(errMsg, "unmarshal") {
		return NewAppErrorWithCause(ErrInvalidJSON, "Invalid JSON format", http.StatusBadRequest, err)
	}

	// Validation errors
	if strings.Contains(errMsg, "validation") || strings.Contains(errMsg, "invalid") {
		return NewAppErrorWithCause(ErrInvalidInput, "Invalid input data", http.StatusBadRequest, err)
	}

	// Default to internal error
	return NewAppErrorWithCause(ErrInternal, "Internal server error", http.StatusInternalServerError, err).
		WithStackTrace()
}

// logError logs the error with appropriate level
func (h *ErrorHandler) logError(err *AppError) {
	attrs := []any{
		slog.String("code", string(err.Code)),
		slog.String("message", err.Message),
	}

	if err.Cause != nil {
		attrs = append(attrs, slog.String("cause", err.Cause.Error()))
	}

	if err.Details != nil {
		attrs = append(attrs, slog.Any("details", err.Details))
	}

	// Log with appropriate level based on status code
	if err.StatusCode >= 500 {
		h.logger.Error("Application error", attrs...)
	} else if err.StatusCode >= 400 {
		h.logger.Warn("Client error", attrs...)
	} else {
		h.logger.Info("Request error", attrs...)
	}
}

// writeErrorResponse writes the error response
func (h *ErrorHandler) writeErrorResponse(w http.ResponseWriter, err *AppError) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
		"success": false,
	}

	if err.Details != nil {
		response["error"].(map[string]interface{})["details"] = err.Details
	}

	json.NewEncoder(w).Encode(response)
}

// WriteError is a convenience function for writing errors
func WriteError(w http.ResponseWriter, code ErrorCode, message string) {
	err := NewAppError(code, message, getStatusCodeForCode(code))

	// Simple write without logging (for backward compatibility)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
		"success": false,
	})
}

// WriteErrorCode writes an error with explicit status code (backward compatibility)
func WriteErrorCode(w http.ResponseWriter, statusCode int, code ErrorCode, message string) {
	err := NewAppError(code, message, statusCode)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)

	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
		"success": false,
	})
}

// WriteErrorWithDetails writes an error with details
func WriteErrorWithDetails(w http.ResponseWriter, code ErrorCode, message string, details map[string]interface{}) {
	err := NewAppError(code, message, getStatusCodeForCode(code)).WithDetails("details", details)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(err.StatusCode)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    err.Code,
			"message": err.Message,
		},
		"success": false,
	}

	if err.Details != nil {
		response["error"].(map[string]interface{})["details"] = err.Details
	}

	json.NewEncoder(w).Encode(response)
}

// getStatusCodeForCode returns appropriate HTTP status code for error code
func getStatusCodeForCode(code ErrorCode) int {
	switch code {
	case ErrUnauthorized, ErrInvalidToken, ErrTokenExpired:
		return http.StatusUnauthorized
	case ErrForbidden, ErrInsufficientPermissions:
		return http.StatusForbidden
	case ErrNotFound:
		return http.StatusNotFound
	case ErrAlreadyExists, ErrConflict:
		return http.StatusConflict
	case ErrInvalidInput, ErrMissingField, ErrInvalidFormat, ErrInvalidEmail, ErrInvalidUUID, ErrInvalidJSON:
		return http.StatusBadRequest
	case ErrRateLimited:
		return http.StatusTooManyRequests
	case ErrServiceUnavailable, ErrNetwork, ErrDatabase:
		return http.StatusServiceUnavailable
	case ErrTwoFactorRequired:
		return http.StatusPreconditionRequired
	default:
		return http.StatusInternalServerError
	}
}

// RecoveryMiddleware recovers from panics and converts them to errors
func RecoveryMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					logger.Error("Panic recovered",
						slog.Any("error", err),
						slog.String("path", r.URL.Path),
						slog.String("method", r.Method),
					)

					appErr := NewAppError(ErrInternal, "Internal server error", http.StatusInternalServerError).
						WithDetails("recovered_from_panic", true).
						WithStackTrace()

					errorHandler := NewErrorHandler(logger)
					errorHandler.HandleError(w, appErr)
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
