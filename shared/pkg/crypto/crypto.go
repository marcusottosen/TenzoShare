// Package crypto provides cryptographic primitives used across all services:
//   - AES-256-GCM authenticated encryption/decryption
//   - Argon2id password hashing (OWASP recommended parameters)
//   - Cryptographically secure random bytes / URL-safe tokens
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/argon2"
)

// Argon2id parameters — OWASP recommended minimum.
const (
	argonTime    uint32 = 1
	argonMemory  uint32 = 64 * 1024 // 64 MB
	argonThreads uint8  = 4
	argonKeyLen  uint32 = 32
	saltLen             = 16
)

// Encrypt encrypts plaintext with AES-256-GCM using the provided 32-byte key.
// Returns a base64-encoded string: [12-byte nonce][ciphertext][16-byte tag].
func Encrypt(plaintext, key []byte) (string, error) {
	if len(key) != 32 {
		return "", fmt.Errorf("crypto: key must be exactly 32 bytes (got %d)", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generate nonce: %w", err)
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext.
func Decrypt(ciphertextB64 string, key []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("crypto: key must be exactly 32 bytes (got %d)", len(key))
	}
	ciphertext, err := base64.StdEncoding.DecodeString(ciphertextB64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decode base64: %w", err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: new gcm: %w", err)
	}
	if len(ciphertext) < gcm.NonceSize() {
		return nil, fmt.Errorf("crypto: ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:gcm.NonceSize()], ciphertext[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypt: %w", err)
	}
	return plaintext, nil
}

// HashPassword hashes password+pepper with Argon2id and a random salt.
// Encoded format: "argon2id$<saltBase64>$<hashBase64>"
func HashPassword(password, pepper string) (string, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return "", fmt.Errorf("crypto: generate salt: %w", err)
	}
	peppered := []byte(password + pepper)
	hash := argon2.IDKey(peppered, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("argon2id$%s$%s",
		base64.StdEncoding.EncodeToString(salt),
		base64.StdEncoding.EncodeToString(hash),
	), nil
}

// VerifyPassword returns true if password+pepper matches the stored Argon2id hash.
// Timing-safe comparison is used to prevent timing attacks.
func VerifyPassword(password, stored, pepper string) (bool, error) {
	parts := strings.SplitN(stored, "$", 3)
	if len(parts) != 3 || parts[0] != "argon2id" {
		return false, fmt.Errorf("crypto: invalid hash format")
	}
	salt, err := base64.StdEncoding.DecodeString(parts[1])
	if err != nil {
		return false, fmt.Errorf("crypto: decode salt: %w", err)
	}
	expectedHash, err := base64.StdEncoding.DecodeString(parts[2])
	if err != nil {
		return false, fmt.Errorf("crypto: decode hash: %w", err)
	}
	peppered := []byte(password + pepper)
	computed := argon2.IDKey(peppered, salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return subtle.ConstantTimeCompare(expectedHash, computed) == 1, nil
}

// RandomBytes returns n cryptographically secure random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("crypto: random bytes: %w", err)
	}
	return b, nil
}

// RandomToken returns a URL-safe base64-encoded random token of byteLen random bytes.
func RandomToken(byteLen int) (string, error) {
	b, err := RandomBytes(byteLen)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
