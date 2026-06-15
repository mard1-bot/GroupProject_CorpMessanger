package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"sync"
)

var (
	masterKey      []byte
	masterKeyMutex sync.RWMutex
)

// InitMasterKey initializes the master encryption key from environment variable
func InitMasterKey() error {
	keyStr := os.Getenv("ENCRYPTION_MASTER_KEY")
	if keyStr == "" {
		if filePath := os.Getenv("ENCRYPTION_MASTER_KEY_FILE"); filePath != "" {
			content, err := os.ReadFile(filePath)
			if err == nil {
				keyStr = string(content)
			}
		}
	}
	if keyStr == "" {
		return errors.New("ENCRYPTION_MASTER_KEY environment variable is required")
	}

	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return fmt.Errorf("failed to decode master key: %w", err)
	}

	// AES-256 requires 32-byte key
	if len(keyBytes) != 32 {
		return fmt.Errorf("master key must be 32 bytes (AES-256), got %d bytes", len(keyBytes))
	}

	masterKeyMutex.Lock()
	masterKey = keyBytes
	masterKeyMutex.Unlock()

	return nil
}

// IsInitialized checks if the master key has been initialized
func IsInitialized() bool {
	masterKeyMutex.RLock()
	defer masterKeyMutex.RUnlock()
	return len(masterKey) > 0
}

// Encrypt encrypts plaintext using AES-256-GCM with the master key
// Returns base64-encoded ciphertext with nonce prepended
func Encrypt(plaintext string) (string, error) {
	masterKeyMutex.RLock()
	key := make([]byte, len(masterKey))
	copy(key, masterKey)
	masterKeyMutex.RUnlock()

	if len(key) == 0 {
		return "", errors.New("master key not initialized")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce (12 bytes for GCM)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt and prepend nonce
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)

	// Encode as base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// EncryptBytes encrypts binary data using AES-256-GCM with the master key.
// Returns ciphertext with nonce prepended (no base64 encoding).
func EncryptBytes(plaintext []byte) ([]byte, error) {
	masterKeyMutex.RLock()
	key := make([]byte, len(masterKey))
	copy(key, masterKey)
	masterKeyMutex.RUnlock()

	if len(key) == 0 {
		return nil, errors.New("master key not initialized")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	return gcm.Seal(nonce, nonce, plaintext, nil), nil
}

// DecryptBytes decrypts binary data encrypted by EncryptBytes.
func DecryptBytes(ciphertext []byte) ([]byte, error) {
	masterKeyMutex.RLock()
	key := make([]byte, len(masterKey))
	copy(key, masterKey)
	masterKeyMutex.RUnlock()

	if len(key) == 0 {
		return nil, errors.New("master key not initialized")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, errors.New("ciphertext too short")
	}

	nonce, data := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt: %w", err)
	}

	return plaintext, nil
}

// Decrypt decrypts base64-encoded ciphertext using AES-256-GCM with the master key
func Decrypt(ciphertext string) (string, error) {
	masterKeyMutex.RLock()
	key := make([]byte, len(masterKey))
	copy(key, masterKey)
	masterKeyMutex.RUnlock()

	if len(key) == 0 {
		return "", errors.New("master key not initialized")
	}

	// Decode base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	// Extract nonce and ciphertext
	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}
