package http

import (
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ValidationMiddleware provides request validation
func ValidationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Validate Content-Type for POST/PUT requests
		if r.Method == "POST" || r.Method == "PUT" {
			contentType := r.Header.Get("Content-Type")
			if !strings.Contains(contentType, "application/json") &&
				!strings.Contains(contentType, "multipart/form-data") &&
				!strings.Contains(contentType, "application/x-www-form-urlencoded") {
				WriteErrorCode(w, http.StatusBadRequest, "invalid_content_type", "Invalid Content-Type header")
				return
			}
		}

		// Validate request size
		if r.ContentLength > 100*1024*1024 { // 100MB limit
			WriteErrorCode(w, http.StatusRequestEntityTooLarge, "request_too_large", "Request entity too large")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// ValidateUUID validates UUID parameter
func ValidateUUID(paramName string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			idStr := chi.URLParam(r, paramName)
			if idStr == "" {
				WriteErrorCode(w, http.StatusBadRequest, "missing_param", "Missing "+paramName+" parameter")
				return
			}

			if _, err := uuid.Parse(idStr); err != nil {
				WriteErrorCode(w, http.StatusBadRequest, "invalid_uuid", "Invalid "+paramName+" format")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ValidateEmail validates email format
func ValidateEmail(email string) bool {
	if email == "" {
		return false
	}

	// Basic email regex - more comprehensive validation should be done at application level
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email) && len(email) <= 254
}

// ValidatePhone validates phone number format
func ValidatePhone(phone string) bool {
	if phone == "" {
		return false
	}

	// Phone regex: allows +, digits, spaces, dashes, parentheses, dots
	phoneRegex := regexp.MustCompile(`^[+]?[\s\d\-\(\)\.]{10,25}$`)
	if !phoneRegex.MatchString(phone) {
		return false
	}

	// Count digits - must be at least 10
	digitCount := 0
	for _, ch := range phone {
		if ch >= '0' && ch <= '9' {
			digitCount++
		}
	}
	return digitCount >= 10
}

// ValidatePassword validates password strength
func ValidatePassword(password string) error {
	if len(password) < 8 {
		return &ValidationError{Field: "password", Message: "Password must be at least 8 characters long"}
	}

	if len(password) > 128 {
		return &ValidationError{Field: "password", Message: "Password must be less than 128 characters"}
	}

	// Check for common passwords (top most common)
	commonPasswords := map[string]bool{
		"password": true, "123456": true, "12345678": true, "qwerty": true,
		"abc123": true, "password123": true, "admin": true, "welcome": true,
		"monkey": true, "letmein": true, "dragon": true, "master": true,
		"hello": true, "login": true, "football": true, "iloveyou": true,
		"princess": true, "starwars": true, "123123": true, "password1": true,
		"123qwe": true, "qwerty123": true, "1q2w3e4r": true, "baseball": true,
		"superman": true, "whatever": true, "trustno1": true, "michael": true,
	}
	if commonPasswords[strings.ToLower(password)] {
		return &ValidationError{Field: "password", Message: "Password is too common, please choose a stronger password"}
	}

	hasUpper := false
	hasLower := false
	hasDigit := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasDigit = true
		case strings.ContainsRune("!@#$%^&*()_+-=[]{}|;:,.<>?", char):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return &ValidationError{Field: "password", Message: "Password must contain at least one uppercase letter"}
	}
	if !hasLower {
		return &ValidationError{Field: "password", Message: "Password must contain at least one lowercase letter"}
	}
	if !hasDigit {
		return &ValidationError{Field: "password", Message: "Password must contain at least one digit"}
	}
	if !hasSpecial {
		return &ValidationError{Field: "password", Message: "Password must contain at least one special character"}
	}

	return nil
}

// ValidateMessageContent validates message content
func ValidateMessageContent(content string) error {
	if len(content) == 0 {
		return &ValidationError{Field: "content", Message: "Message content cannot be empty"}
	}

	if len(content) > 4000 {
		return &ValidationError{Field: "content", Message: "Message content too long (max 4000 characters)"}
	}

	// Check for potentially malicious content
	if strings.Contains(content, "<script") || strings.Contains(content, "javascript:") {
		return &ValidationError{Field: "content", Message: "Message content contains potentially malicious code"}
	}

	return nil
}

// ValidateChatTitle validates chat title
func ValidateChatTitle(title string) error {
	if title == "" {
		return &ValidationError{Field: "title", Message: "Chat title cannot be empty"}
	}

	if len(title) > 100 {
		return &ValidationError{Field: "title", Message: "Chat title too long (max 100 characters)"}
	}

	// Check for invalid characters
	if strings.Contains(title, "<") || strings.Contains(title, ">") {
		return &ValidationError{Field: "title", Message: "Chat title contains invalid characters"}
	}

	return nil
}

// ValidationError represents a validation error
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return e.Message
}

// ValidateJSONBody validates and decodes JSON body with size limits
func ValidateJSONBody(r *http.Request, v interface{}, maxSize int64) error {
	// Limit request body size using LimitReader (safe without ResponseWriter)
	limitedReader := io.LimitReader(r.Body, maxSize+1) // +1 to detect overflow

	decoder := json.NewDecoder(limitedReader)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(v); err != nil {
		if err.Error() == "http: request body too large" {
			return &ValidationError{Field: "body", Message: "Request body too large"}
		}
		return &ValidationError{Field: "body", Message: "Invalid JSON format"}
	}

	// Check if there's more data (body exceeded maxSize)
	extra := make([]byte, 1)
	if n, _ := limitedReader.Read(extra); n > 0 {
		return &ValidationError{Field: "body", Message: "Request body too large"}
	}

	return nil
}

// decodeJSON decodes JSON body (backward compatibility alias)
func decodeJSON(r io.Reader, v interface{}) error {
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	return decoder.Decode(v)
}

// SanitizeInput sanitizes user input
func SanitizeInput(input string) string {
	// Remove potentially dangerous characters
	input = strings.ReplaceAll(input, "<", "&lt;")
	input = strings.ReplaceAll(input, ">", "&gt;")
	input = strings.ReplaceAll(input, "&", "&amp;")
	input = strings.ReplaceAll(input, "\"", "&quot;")
	input = strings.ReplaceAll(input, "'", "&#x27;")

	// Trim whitespace and limit length
	input = strings.TrimSpace(input)
	if utf8.RuneCountInString(input) > 1000 {
		// Truncate to 1000 runes
		i := 0
		for pos := range input {
			if i >= 1000 {
				return input[:pos]
			}
			i++
		}
	}

	return input
}
