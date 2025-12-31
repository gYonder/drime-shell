// Package crypto provides encryption utilities for vault operations.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"runtime"

	"golang.org/x/crypto/pbkdf2"
)

const (
	// KeySize is the size of AES-256 keys in bytes.
	KeySize = 32
	// IVSize is the size of GCM initialization vectors (12 bytes per NIST).
	IVSize = 12
	// SaltSize is the size of PBKDF2 salt in bytes.
	SaltSize = 16
	// PBKDF2Iterations matches the Drime web app (250,000 iterations).
	PBKDF2Iterations = 250000
	// CheckPlaintext is the known plaintext used for password verification.
	// Must match the web app / pydrime value: "vault-unlock"
	CheckPlaintext = "vault-unlock"
)

var (
	// ErrKeyZeroed is returned when attempting to use a zeroed key.
	ErrKeyZeroed = errors.New("encryption key has been zeroed")
	// ErrCiphertextTooShort is returned when ciphertext is too short to contain IV + data.
	ErrCiphertextTooShort = errors.New("ciphertext too short")
	// ErrDecryptionFailed is returned when decryption fails (wrong key or tampered data).
	ErrDecryptionFailed = errors.New("decryption failed: wrong password or corrupted data")
	// ErrPasswordMismatch is returned when password verification fails.
	ErrPasswordMismatch = errors.New("incorrect vault password")
)

// VaultKey holds the derived encryption key for vault operations.
// The key should be zeroed when no longer needed.
type VaultKey struct {
	key []byte
}

// DeriveKey derives a 256-bit AES key from a password and salt using PBKDF2-SHA256.
// The returned key should be zeroed with Zero() when no longer needed.
func DeriveKey(password string, salt []byte) *VaultKey {
	key := pbkdf2.Key(
		[]byte(password),
		salt,
		PBKDF2Iterations,
		KeySize,
		sha256.New,
	)
	return &VaultKey{key: key}
}

// Zero securely clears the key from memory.
// After calling Zero, the key cannot be used for encryption/decryption.
func (vk *VaultKey) Zero() {
	if vk == nil || vk.key == nil {
		return
	}
	for i := range vk.key {
		vk.key[i] = 0
	}
	vk.key = nil
	// Hint to GC to run (best effort, not guaranteed)
	runtime.GC()
}

// IsZeroed returns true if the key has been zeroed or is nil.
func (vk *VaultKey) IsZeroed() bool {
	return vk == nil || vk.key == nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random IV.
// Returns the ciphertext and the IV separately (IV is not prepended).
func (vk *VaultKey) Encrypt(plaintext []byte) (ciphertext []byte, iv []byte, err error) {
	if vk.IsZeroed() {
		return nil, nil, ErrKeyZeroed
	}

	block, err := aes.NewCipher(vk.key)
	if err != nil {
		return nil, nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, fmt.Errorf("create GCM: %w", err)
	}

	// Generate random IV
	iv = make([]byte, IVSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return nil, nil, fmt.Errorf("generate IV: %w", err)
	}

	// Seal encrypts and appends authentication tag
	ciphertext = gcm.Seal(nil, iv, plaintext, nil)

	return ciphertext, iv, nil
}

// Decrypt decrypts ciphertext using AES-256-GCM with the provided IV.
func (vk *VaultKey) Decrypt(ciphertext []byte, iv []byte) ([]byte, error) {
	if vk.IsZeroed() {
		return nil, ErrKeyZeroed
	}

	if len(iv) != IVSize {
		return nil, fmt.Errorf("invalid IV size: expected %d, got %d", IVSize, len(iv))
	}

	block, err := aes.NewCipher(vk.key)
	if err != nil {
		return nil, fmt.Errorf("create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("create GCM: %w", err)
	}

	plaintext, err := gcm.Open(nil, iv, ciphertext, nil)
	if err != nil {
		return nil, ErrDecryptionFailed
	}

	return plaintext, nil
}

// GenerateSalt creates a random salt for PBKDF2 key derivation.
func GenerateSalt() ([]byte, error) {
	salt := make([]byte, SaltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generate salt: %w", err)
	}
	return salt, nil
}

// CreateCheckValue creates an encrypted check value for password verification.
// The check value is encrypted known plaintext that can be decrypted to verify
// the password is correct without decrypting actual user data.
// Returns base64-encoded ciphertext and base64-encoded IV.
func (vk *VaultKey) CreateCheckValue() (checkB64 string, ivB64 string, err error) {
	ciphertext, iv, err := vk.Encrypt([]byte(CheckPlaintext))
	if err != nil {
		return "", "", err
	}
	return base64.StdEncoding.EncodeToString(ciphertext),
		base64.StdEncoding.EncodeToString(iv),
		nil
}

// VerifyPassword checks if a password is correct by decrypting the check value.
// Parameters are the same format returned by the Drime API:
// - salt: base64-encoded PBKDF2 salt
// - checkB64: base64-encoded encrypted check value
// - ivB64: base64-encoded IV for the check value
func VerifyPassword(password, saltB64, checkB64, ivB64 string) (bool, error) {
	salt, err := base64.StdEncoding.DecodeString(saltB64)
	if err != nil {
		return false, fmt.Errorf("decode salt: %w", err)
	}

	check, err := base64.StdEncoding.DecodeString(checkB64)
	if err != nil {
		return false, fmt.Errorf("decode check value: %w", err)
	}

	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return false, fmt.Errorf("decode IV: %w", err)
	}

	vk := DeriveKey(password, salt)
	defer vk.Zero()

	plaintext, err := vk.Decrypt(check, iv)
	if err != nil {
		return false, nil // Wrong password, not an error
	}

	// Constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare(plaintext, []byte(CheckPlaintext)) == 1, nil
}

// DecodeIV decodes a base64-encoded IV string to bytes.
func DecodeIV(ivB64 string) ([]byte, error) {
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return nil, fmt.Errorf("decode IV: %w", err)
	}
	if len(iv) != IVSize {
		return nil, fmt.Errorf("invalid IV size: expected %d, got %d", IVSize, len(iv))
	}
	return iv, nil
}

// EncodeIV encodes an IV to base64 string.
func EncodeIV(iv []byte) string {
	return base64.StdEncoding.EncodeToString(iv)
}

// DecodeBase64 decodes a base64-encoded string to bytes.
func DecodeBase64(s string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(s)
}

// EncodeBase64 encodes bytes to a base64 string.
func EncodeBase64(b []byte) string {
	return base64.StdEncoding.EncodeToString(b)
}

// VerifyCheckValue verifies a password by decrypting the check value with a derived key.
// Returns true if the decrypted check value matches the expected plaintext.
func VerifyCheckValue(vk *VaultKey, check, iv []byte) bool {
	if vk.IsZeroed() {
		return false
	}

	plaintext, err := vk.Decrypt(check, iv)
	if err != nil {
		return false
	}

	return subtle.ConstantTimeCompare(plaintext, []byte(CheckPlaintext)) == 1
}

// CreateCheckValue creates an encrypted check value for password verification.
// This is the standalone function that returns raw bytes (not base64).
func CreateCheckValue(vk *VaultKey) (ciphertext []byte, iv []byte, err error) {
	return vk.Encrypt([]byte(CheckPlaintext))
}
