package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestJWTService_GenerateToken(t *testing.T) {
	secret := "test-secret-key"
	jwt := NewJWTService(secret)

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := 24 * time.Hour

	token, err := jwt.GenerateToken(userID, email, role, duration)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestJWTService_ParseToken(t *testing.T) {
	secret := "test-secret-key"
	jwt := NewJWTService(secret)

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := 24 * time.Hour

	token, err := jwt.GenerateToken(userID, email, role, duration)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	claims, err := jwt.ParseToken(token)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, claims.UserID)
	}

	if claims.Email != email {
		t.Errorf("expected email %s, got %s", email, claims.Email)
	}

	if claims.Role != role {
		t.Errorf("expected role %s, got %s", role, claims.Role)
	}
}

func TestJWTService_ParseToken_Invalid(t *testing.T) {
	secret := "test-secret-key"
	jwt := NewJWTService(secret)

	invalidToken := "invalid.token.here"

	_, err := jwt.ParseToken(invalidToken)
	if err == nil {
		t.Error("expected error for invalid token")
	}
}

func TestJWTService_ParseToken_WrongSecret(t *testing.T) {
	secret1 := "test-secret-key-1"
	jwt1 := NewJWTService(secret1)

	secret2 := "test-secret-key-2"
	jwt2 := NewJWTService(secret2)

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := 24 * time.Hour

	token, err := jwt1.GenerateToken(userID, email, role, duration)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = jwt2.ParseToken(token)
	if err == nil {
		t.Error("expected error when validating token with different secret")
	}
}

func TestJWTService_ParseToken_Expired(t *testing.T) {
	secret := "test-secret-key"
	jwt := NewJWTService(secret)

	userID := uuid.New()
	email := "test@example.com"
	role := "user"
	duration := -1 * time.Hour // Expired

	token, err := jwt.GenerateToken(userID, email, role, duration)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	_, err = jwt.ParseToken(token)
	if err == nil {
		t.Error("expected error for expired token")
	}
}

func TestContextWithClaims(t *testing.T) {
	claims := &Claims{
		UserID: uuid.New(),
		Email:  "test@example.com",
		Role:   "user",
	}

	ctx := ContextWithClaims(context.Background(), claims)

	retrievedClaims, ok := ClaimsFromContext(ctx)
	if !ok {
		t.Error("expected claims to be in context")
	}

	if retrievedClaims.UserID != claims.UserID {
		t.Errorf("expected userID %s, got %s", claims.UserID, retrievedClaims.UserID)
	}
}

func TestClaimsFromContext_NotFound(t *testing.T) {
	ctx := context.Background()

	_, ok := ClaimsFromContext(ctx)
	if ok {
		t.Error("expected claims not to be in context")
	}
}
