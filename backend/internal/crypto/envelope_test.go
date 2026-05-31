package crypto

import (
	"encoding/base64"
	"os"
	"testing"
)

func TestInitMasterKey(t *testing.T) {
	tests := []struct {
		name      string
		keyValue  string
		wantError bool
	}{
		{
			name:      "valid 32-byte key",
			keyValue:  base64.StdEncoding.EncodeToString(make([]byte, 32)),
			wantError: false,
		},
		{
			name:      "empty key",
			keyValue:  "",
			wantError: true,
		},
		{
			name:      "invalid base64",
			keyValue:  "not-valid-base64!!!",
			wantError: true,
		},
		{
			name:      "wrong key length (16 bytes)",
			keyValue:  base64.StdEncoding.EncodeToString(make([]byte, 16)),
			wantError: true,
		},
		{
			name:      "wrong key length (64 bytes)",
			keyValue:  base64.StdEncoding.EncodeToString(make([]byte, 64)),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset master key
			masterKeyMutex.Lock()
			masterKey = nil
			masterKeyMutex.Unlock()

			if tt.keyValue != "" {
				os.Setenv("ENCRYPTION_MASTER_KEY", tt.keyValue)
			} else {
				os.Unsetenv("ENCRYPTION_MASTER_KEY")
			}
			defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

			err := InitMasterKey()
			if (err != nil) != tt.wantError {
				t.Errorf("InitMasterKey() error = %v, wantError %v", err, tt.wantError)
			}

			if !tt.wantError && !IsInitialized() {
				t.Error("IsInitialized() = false, want true after successful init")
			}
		})
	}
}

func TestIsInitialized(t *testing.T) {
	// Reset master key
	masterKeyMutex.Lock()
	masterKey = nil
	masterKeyMutex.Unlock()

	if IsInitialized() {
		t.Error("IsInitialized() = true, want false before initialization")
	}

	// Initialize with valid key
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		t.Fatalf("InitMasterKey() failed: %v", err)
	}

	if !IsInitialized() {
		t.Error("IsInitialized() = false, want true after initialization")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	// Setup: Initialize master key
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		t.Fatalf("InitMasterKey() failed: %v", err)
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{
			name:      "simple text",
			plaintext: "Hello, World!",
		},
		{
			name:      "empty string",
			plaintext: "",
		},
		{
			name:      "unicode text",
			plaintext: "Привет, мир! 🌍",
		},
		{
			name:      "long text",
			plaintext: "Lorem ipsum dolor sit amet, consectetur adipiscing elit. " + 
				"Sed do eiusmod tempor incididunt ut labore et dolore magna aliqua.",
		},
		{
			name:      "special characters",
			plaintext: "!@#$%^&*()_+-=[]{}|;':\",./<>?",
		},
		{
			name:      "json data",
			plaintext: `{"email":"user@example.com","password":"secret123"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Encrypt
			ciphertext, err := Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt() error = %v", err)
			}

			if ciphertext == "" {
				t.Error("Encrypt() returned empty ciphertext")
			}

			// Verify it's valid base64
			if _, err := base64.StdEncoding.DecodeString(ciphertext); err != nil {
				t.Errorf("Encrypt() returned invalid base64: %v", err)
			}

			// Decrypt
			decrypted, err := Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt() error = %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypt() = %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptWithoutInitialization(t *testing.T) {
	// Reset master key
	masterKeyMutex.Lock()
	masterKey = nil
	masterKeyMutex.Unlock()

	_, err := Encrypt("test")
	if err == nil {
		t.Error("Encrypt() without initialization should return error")
	}

	if err.Error() != "master key not initialized" {
		t.Errorf("Encrypt() error = %v, want 'master key not initialized'", err)
	}
}

func TestDecryptWithoutInitialization(t *testing.T) {
	// Reset master key
	masterKeyMutex.Lock()
	masterKey = nil
	masterKeyMutex.Unlock()

	_, err := Decrypt("dGVzdA==")
	if err == nil {
		t.Error("Decrypt() without initialization should return error")
	}

	if err.Error() != "master key not initialized" {
		t.Errorf("Decrypt() error = %v, want 'master key not initialized'", err)
	}
}

func TestDecryptInvalidData(t *testing.T) {
	// Setup: Initialize master key
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		t.Fatalf("InitMasterKey() failed: %v", err)
	}

	tests := []struct {
		name       string
		ciphertext string
		wantError  string
	}{
		{
			name:       "invalid base64",
			ciphertext: "not-valid-base64!!!",
			wantError:  "failed to decode ciphertext",
		},
		{
			name:       "too short ciphertext",
			ciphertext: base64.StdEncoding.EncodeToString([]byte("short")),
			wantError:  "ciphertext too short",
		},
		{
			name:       "corrupted data",
			ciphertext: base64.StdEncoding.EncodeToString(make([]byte, 50)),
			wantError:  "failed to decrypt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Decrypt(tt.ciphertext)
			if err == nil {
				t.Error("Decrypt() should return error for invalid data")
			}
		})
	}
}

func TestEncryptDeterminism(t *testing.T) {
	// Setup: Initialize master key
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		t.Fatalf("InitMasterKey() failed: %v", err)
	}

	plaintext := "test message"

	// Encrypt same plaintext multiple times
	ciphertext1, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	ciphertext2, err := Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Ciphertexts should be different (due to random nonce)
	if ciphertext1 == ciphertext2 {
		t.Error("Encrypt() should produce different ciphertexts for same plaintext (non-deterministic)")
	}

	// But both should decrypt to same plaintext
	decrypted1, err := Decrypt(ciphertext1)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	decrypted2, err := Decrypt(ciphertext2)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if decrypted1 != plaintext || decrypted2 != plaintext {
		t.Error("Decrypt() should return original plaintext")
	}
}

func TestConcurrentEncryptDecrypt(t *testing.T) {
	// Setup: Initialize master key
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		t.Fatalf("InitMasterKey() failed: %v", err)
	}

	// Run concurrent encryptions
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(id int) {
			plaintext := "concurrent test message"
			ciphertext, err := Encrypt(plaintext)
			if err != nil {
				t.Errorf("Concurrent Encrypt() error = %v", err)
			}

			decrypted, err := Decrypt(ciphertext)
			if err != nil {
				t.Errorf("Concurrent Decrypt() error = %v", err)
			}

			if decrypted != plaintext {
				t.Errorf("Concurrent Decrypt() = %q, want %q", decrypted, plaintext)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}
}

func BenchmarkEncrypt(b *testing.B) {
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		b.Fatalf("InitMasterKey() failed: %v", err)
	}

	plaintext := "benchmark test message"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Encrypt(plaintext)
	}
}

func BenchmarkDecrypt(b *testing.B) {
	validKey := base64.StdEncoding.EncodeToString(make([]byte, 32))
	os.Setenv("ENCRYPTION_MASTER_KEY", validKey)
	defer os.Unsetenv("ENCRYPTION_MASTER_KEY")

	if err := InitMasterKey(); err != nil {
		b.Fatalf("InitMasterKey() failed: %v", err)
	}

	plaintext := "benchmark test message"
	ciphertext, err := Encrypt(plaintext)
	if err != nil {
		b.Fatalf("Encrypt() failed: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = Decrypt(ciphertext)
	}
}
