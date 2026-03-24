package crypto

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// VaultEncryptor implements EncryptorInterface using HashiCorp Vault's Transit
// Engine for encryption and decryption. This centralizes key management in
// Vault, enabling automated key rotation, audit logging, and hardware-backed
// key storage without application code changes.
type VaultEncryptor struct {
	addr       string // e.g. "http://vault:8200"
	token      string // Vault auth token
	transitKey string // Transit key name, e.g. "llm-router"
	client     *http.Client
}

// NewVaultEncryptor creates a new VaultEncryptor configured to use the given
// Vault server, authentication token, and transit engine key name.
func NewVaultEncryptor(addr, token, transitKey string) *VaultEncryptor {
	return &VaultEncryptor{
		addr:       strings.TrimRight(addr, "/"),
		token:      token,
		transitKey: transitKey,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// vaultRequest is the generic envelope for Vault Transit API requests.
type vaultRequest struct {
	Plaintext  string `json:"plaintext,omitempty"`
	Ciphertext string `json:"ciphertext,omitempty"`
}

// vaultResponse is the generic envelope for Vault Transit API responses.
type vaultResponse struct {
	Data struct {
		Plaintext  string `json:"plaintext,omitempty"`
		Ciphertext string `json:"ciphertext,omitempty"`
	} `json:"data"`
	Errors []string `json:"errors,omitempty"`
}

// Encrypt encrypts plaintext using the Vault Transit Engine.
// The plaintext is base64-encoded before sending to Vault (Vault requirement).
// Returns the Vault ciphertext string (prefixed with "vault:v1:...").
func (v *VaultEncryptor) Encrypt(plaintext string) (string, error) {
	url := fmt.Sprintf("%s/v1/transit/encrypt/%s", v.addr, v.transitKey)

	reqBody := vaultRequest{
		Plaintext: base64.StdEncoding.EncodeToString([]byte(plaintext)),
	}

	resp, err := v.doRequest("POST", url, reqBody)
	if err != nil {
		return "", fmt.Errorf("vault encrypt: %w", err)
	}

	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("vault encrypt: %s", strings.Join(resp.Errors, "; "))
	}

	return resp.Data.Ciphertext, nil
}

// Decrypt decrypts a Vault Transit-encrypted ciphertext.
// Returns the original plaintext string.
func (v *VaultEncryptor) Decrypt(ciphertext string) (string, error) {
	url := fmt.Sprintf("%s/v1/transit/decrypt/%s", v.addr, v.transitKey)

	reqBody := vaultRequest{
		Ciphertext: ciphertext,
	}

	resp, err := v.doRequest("POST", url, reqBody)
	if err != nil {
		return "", fmt.Errorf("vault decrypt: %w", err)
	}

	if len(resp.Errors) > 0 {
		return "", fmt.Errorf("vault decrypt: %s", strings.Join(resp.Errors, "; "))
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Data.Plaintext)
	if err != nil {
		return "", fmt.Errorf("vault decrypt: failed to decode plaintext: %w", err)
	}

	return string(decoded), nil
}

// doRequest performs an authenticated HTTP request to the Vault API.
func (v *VaultEncryptor) doRequest(method, url string, payload interface{}) (*vaultResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(method, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Vault-Token", v.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vault returned HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var vaultResp vaultResponse
	if err := json.Unmarshal(respBody, &vaultResp); err != nil {
		return nil, fmt.Errorf("vault response parse error: %w", err)
	}

	return &vaultResp, nil
}
