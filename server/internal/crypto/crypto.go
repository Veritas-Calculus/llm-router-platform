// Package crypto provides encryption utilities for sensitive data.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
)

var (
	// ErrInvalidKey is returned when the encryption key is invalid.
	ErrInvalidKey = errors.New("encryption key must be 32 bytes")
	// ErrInvalidCiphertext is returned when the ciphertext is invalid.
	ErrInvalidCiphertext = errors.New("invalid ciphertext")
	// ErrNotInitialized is returned when encryption is used before initialization.
	ErrNotInitialized = errors.New("encryption not initialized: ENCRYPTION_KEY is required")
)

// EncryptorInterface defines the interface for encryption/decryption backends.
// Implementations include local AES-256-GCM (Encryptor) and HashiCorp Vault
// Transit Engine (VaultEncryptor).
type EncryptorInterface interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

// Encryptor handles encryption and decryption of sensitive data using local AES-256-GCM.
type Encryptor struct {
	key []byte
	mu  sync.RWMutex
}

var (
	defaultEncryptor EncryptorInterface
	localEncryptor   *Encryptor // kept for HMACHash which needs direct key access
	once             sync.Once
)

// Initialize sets up the default encryptor with a local AES-256-GCM backend.
// The key must be exactly 32 bytes for AES-256.
func Initialize(key string) error {
	if len(key) != 32 {
		return ErrInvalidKey
	}

	once.Do(func() {
		enc := &Encryptor{
			key: []byte(key),
		}
		localEncryptor = enc
		defaultEncryptor = enc
	})

	return nil
}

// InitializeVault sets up the default encryptor with a Vault Transit Engine backend.
// The local AES encryptor is still initialized for HMACHash operations.
func InitializeVault(localKey string, vaultAddr, vaultToken, transitKey string) error {
	if len(localKey) != 32 {
		return ErrInvalidKey
	}

	once.Do(func() {
		localEncryptor = &Encryptor{key: []byte(localKey)}
		defaultEncryptor = NewVaultEncryptor(vaultAddr, vaultToken, transitKey)
	})

	return nil
}

// MustInitialize calls Initialize and panics on error.
// Use at application startup to guarantee encryption is available.
func MustInitialize(key string) {
	if err := Initialize(key); err != nil {
		panic("crypto: " + err.Error())
	}
}

// GetEncryptor returns the default encryptor instance.
func GetEncryptor() EncryptorInterface {
	return defaultEncryptor
}

// Encrypt encrypts plaintext using AES-256-GCM.
func (e *Encryptor) Encrypt(plaintext string) (string, error) {
	if e == nil || len(e.key) == 0 {
		return "", ErrNotInitialized
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
		return "", ErrNotInitialized
	}

	e.mu.RLock()
	defer e.mu.RUnlock()

	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("%w: base64 decode failed: %v", ErrInvalidCiphertext, err)
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
		return "", fmt.Errorf("%w: ciphertext too short", ErrInvalidCiphertext)
	}

	nonce, ciphertextBytes := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertextBytes, nil)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrInvalidCiphertext, err)
	}

	return string(plaintext), nil
}

// Encrypt encrypts using the default encryptor.
func Encrypt(plaintext string) (string, error) {
	if defaultEncryptor == nil {
		return "", ErrNotInitialized
	}
	return defaultEncryptor.Encrypt(plaintext)
}

// Decrypt decrypts using the default encryptor.
func Decrypt(ciphertext string) (string, error) {
	if defaultEncryptor == nil {
		return "", ErrNotInitialized
	}
	return defaultEncryptor.Decrypt(ciphertext)
}

// IsInitialized returns true if the encryptor has been initialized.
func IsInitialized() bool {
	return defaultEncryptor != nil
}

// HMACHash computes HMAC-SHA256 of data using the encryption key as salt.
// The key never leaves this package — callers only get the hash result.
// This always uses the local AES key, even when Vault is the primary encryptor.
func HMACHash(data []byte) []byte {
	if localEncryptor == nil || len(localEncryptor.key) == 0 {
		return nil
	}
	h := hmac.New(sha256.New, localEncryptor.key)
	h.Write(data)
	return h.Sum(nil)
}
