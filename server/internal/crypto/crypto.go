// Package crypto provides encryption utilities for sensitive data.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"sync"
)

var (
	// ErrInvalidKey is returned when the encryption key is invalid.
	ErrInvalidKey = errors.New("encryption key must be 32 bytes")
	// ErrInvalidCiphertext is returned when the ciphertext is invalid.
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
)

// Encryptor handles encryption and decryption of sensitive data.
type Encryptor struct {
	key []byte
	mu  sync.RWMutex
}

var (
	defaultEncryptor *Encryptor
	once             sync.Once
)

// Initialize sets up the default encryptor with the given key.
// The key must be exactly 32 bytes for AES-256.
func Initialize(key string) error {
	if len(key) != 32 {
		return ErrInvalidKey
	}

	once.Do(func() {
		defaultEncryptor = &Encryptor{
			key: []byte(key),
		}
	})

	return nil
}

// GetEncryptor returns the default encryptor instance.
func GetEncryptor() *Encryptor {
	return defaultEncryptor
}

// Encrypt encrypts plaintext using AES-256-GCM.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if e == nil || len(e.key) == 0 {
		// If no encryption key is set, return plaintext (for backward compatibility)
		return plaintext, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts ciphertext using AES-256-GCM.
func (e *Encryptor) Decrypt(ciphertext string) (string, error) {
	if e == nil || len(e.key) == 0 {
		// If no encryption key is set, return ciphertext as-is (for backward compatibility)
		return ciphertext, nil
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		// If it's not base64, it might be unencrypted legacy data
		return ciphertext, nil
	}

	block, err := aes.NewCipher(e.key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		// Too short to be encrypted, return as-is (legacy unencrypted data)
		return ciphertext, nil
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		// Decryption failed, might be unencrypted legacy data
		return ciphertext, nil
	}

	return string(plaintext), nil
}

// Encrypt encrypts using the default encryptor.
func Encrypt(plaintext string) (string, error) {
	return defaultEncryptor.Encrypt(plaintext)
}

// Decrypt decrypts using the default encryptor.
func Decrypt(ciphertext string) (string, error) {
	return defaultEncryptor.Decrypt(ciphertext)
}

// IsInitialized returns true if the encryptor has been initialized.
func IsInitialized() bool {
	return defaultEncryptor != nil && len(defaultEncryptor.key) > 0
}
