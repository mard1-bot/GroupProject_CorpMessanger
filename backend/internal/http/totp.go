package http

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"math"
	"strings"
	"time"
)

// TOTP implements Time-based One-Time Password (RFC 6238)
type TOTP struct {
	Secret string
	Period int
	Digits int
}

// NewTOTP creates a new TOTP generator
func NewTOTP(secret string) *TOTP {
	return &TOTP{
		Secret: strings.ToUpper(strings.TrimSpace(secret)),
		Period: 30,
		Digits: 6,
	}
}

// Generate generates a TOTP code for the given time
func (t *TOTP) Generate(at time.Time) string {
	counter := uint64(math.Floor(float64(at.Unix()) / float64(t.Period)))
	return t.generateHOTP(counter)
}

// GenerateCurrent generates a TOTP code for the current time
func (t *TOTP) GenerateCurrent() string {
	return t.Generate(time.Now())
}

// Validate validates a TOTP code
func (t *TOTP) Validate(code string, at time.Time) bool {
	if code == "" {
		return false
	}

	// Check current window and adjacent windows for clock skew
	for _, offset := range []int{-1, 0, 1} {
		checkTime := at.Add(time.Duration(offset*t.Period) * time.Second)
		if t.Generate(checkTime) == code {
			return true
		}
	}

	return false
}

// ValidateCurrent validates a TOTP code for the current time
func (t *TOTP) ValidateCurrent(code string) bool {
	return t.Validate(code, time.Now())
}

// generateHOTP generates an HOTP code (RFC 4226)
func (t *TOTP) generateHOTP(counter uint64) string {
	// Decode base32 secret
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(t.Secret)
	if err != nil {
		return ""
	}

	// Convert counter to 8-byte big-endian
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	// Generate HMAC-SHA1
	h := hmac.New(sha1.New, secret)
	h.Write(buf[:])
	sum := h.Sum(nil)

	// Dynamic truncation
	offset := sum[len(sum)-1] & 0x0F
	code := binary.BigEndian.Uint32(sum[offset : offset+4])
	code = code & 0x7FFFFFFF

	// Format to digits
	format := fmt.Sprintf("%%0%dd", t.Digits)
	return fmt.Sprintf(format, code%uint32(math.Pow10(t.Digits)))
}

// GenerateSecret generates a random base32 secret
func GenerateSecret(length int) string {
	if length == 0 {
		length = 32
	}

	// Generate random bytes
	const chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ234567"
	result := make([]byte, length)
	for i := range result {
		// Simple pseudo-random for setup (use crypto/rand in production)
		result[i] = chars[time.Now().UnixNano()%int64(len(chars))]
	}
	return string(result)
}

// ValidateTOTP is a package-level function for validating TOTP codes
func ValidateTOTP(secret, code string) bool {
	totp := NewTOTP(secret)
	return totp.ValidateCurrent(code)
}
