package crypto

import (
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Initialize with a valid 32-byte key
	key := "01234567890123456789012345678901"
	if err := Initialize(key); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}

	enc := GetEncryptor()
	if enc == nil {
		t.Fatal("GetEncryptor returned nil after initialization")
	}

	tests := []struct {
		name      string
		plaintext string
	}{
		{name: "simple string", plaintext: "hello world"},
		{name: "empty string", plaintext: ""},
		{name: "special chars", plaintext: "sk-abc123!@#$%^&*()_+-={}[]|;':\",./<>?"},
		{name: "unicode", plaintext: "你好世界 こんにちは 🚀"},
		{name: "long string", plaintext: "a very long API key string that could be up to 256 characters in length for some providers like OpenAI or Anthropic"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ciphertext, err := enc.Encrypt(tt.plaintext)
			if err != nil {
				t.Fatalf("Encrypt failed: %v", err)
			}

			// Ciphertext should differ from plaintext (unless empty)
			if tt.plaintext != "" && ciphertext == tt.plaintext {
				t.Error("Encrypt returned plaintext unchanged")
			}

			decrypted, err := enc.Decrypt(ciphertext)
			if err != nil {
				t.Fatalf("Decrypt failed: %v", err)
			}

			if decrypted != tt.plaintext {
				t.Errorf("Decrypt mismatch: got %q, want %q", decrypted, tt.plaintext)
			}
		})
	}
}

func TestEncryptProducesDifferentCiphertexts(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz012345"
	if err := Initialize(key); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	enc := GetEncryptor()

	plaintext := "same-value"
	c1, _ := enc.Encrypt(plaintext)
	c2, _ := enc.Encrypt(plaintext)

	if c1 == c2 {
		t.Error("Two encryptions of the same plaintext produced identical ciphertexts (nonce reuse?)")
	}
}

func TestDecryptLegacyData(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz012345"
	if err := Initialize(key); err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	enc := GetEncryptor()

	// Unencrypted legacy data should now return an error (fail-hard policy)
	legacy := "sk-legacy-plain-api-key"
	_, err := enc.Decrypt(legacy)
	if err == nil {
		t.Fatal("Decrypt of non-encrypted data should return an error")
	}
}

func TestInitializeInvalidKey(t *testing.T) {
	// Reset the once for test isolation (can't easily do this, so test separately)
	err := Initialize("short-key")
	if err == nil {
		t.Error("Initialize should fail with short key")
	}
	if err != ErrInvalidKey {
		t.Errorf("expected ErrInvalidKey, got %v", err)
	}
}

func TestNilEncryptor(t *testing.T) {
	var enc *Encryptor

	// Should return ErrNotInitialized when encryptor is nil (fail-hard)
	_, err := enc.Encrypt("test")
	if err != ErrNotInitialized {
		t.Fatalf("nil encryptor Encrypt should return ErrNotInitialized, got: %v", err)
	}

	_, err = enc.Decrypt("test")
	if err != ErrNotInitialized {
		t.Fatalf("nil encryptor Decrypt should return ErrNotInitialized, got: %v", err)
	}
}

func TestIsInitialized(t *testing.T) {
	// After previous tests, defaultEncryptor should be set
	if !IsInitialized() {
		t.Error("IsInitialized should be true after Initialize")
	}
}

func TestHMACHash(t *testing.T) {
	hash := HMACHash([]byte("test data"))
	if hash == nil {
		t.Fatal("HMACHash returned nil")
	}
	if len(hash) != 32 { // SHA-256 produces 32 bytes
		t.Errorf("HMAC hash length = %d, want 32", len(hash))
	}

	// Same input should produce same hash
	hash2 := HMACHash([]byte("test data"))
	if string(hash) != string(hash2) {
		t.Error("HMACHash not deterministic")
	}

	// Different input should produce different hash
	hash3 := HMACHash([]byte("different data"))
	if string(hash) == string(hash3) {
		t.Error("Different inputs produced same HMAC hash")
	}
}
