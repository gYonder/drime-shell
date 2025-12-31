package crypto

import (
	"bytes"
	"encoding/base64"
	"testing"
)

func TestDeriveKey(t *testing.T) {
	password := "test-password"
	salt := []byte("0123456789abcdef") // 16 bytes

	key := DeriveKey(password, salt)
	defer key.Zero()

	if key.IsZeroed() {
		t.Error("key should not be zeroed after creation")
	}

	// Key should be 32 bytes (AES-256)
	if len(key.key) != KeySize {
		t.Errorf("expected key size %d, got %d", KeySize, len(key.key))
	}

	// Same password+salt should produce same key
	key2 := DeriveKey(password, salt)
	defer key2.Zero()

	if !bytes.Equal(key.key, key2.key) {
		t.Error("same password+salt should produce same key")
	}

	// Different password should produce different key
	key3 := DeriveKey("different-password", salt)
	defer key3.Zero()

	if bytes.Equal(key.key, key3.key) {
		t.Error("different passwords should produce different keys")
	}
}

func TestVaultKeyZero(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))

	if key.IsZeroed() {
		t.Error("key should not be zeroed initially")
	}

	key.Zero()

	if !key.IsZeroed() {
		t.Error("key should be zeroed after Zero()")
	}

	// Should be safe to zero multiple times
	key.Zero()
	if !key.IsZeroed() {
		t.Error("key should remain zeroed")
	}
}

func TestEncryptDecrypt(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	defer key.Zero()

	plaintext := []byte("Hello, Vault!")

	ciphertext, iv, err := key.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed: %v", err)
	}

	if len(iv) != IVSize {
		t.Errorf("expected IV size %d, got %d", IVSize, len(iv))
	}

	// Ciphertext should be different from plaintext
	if bytes.Equal(ciphertext, plaintext) {
		t.Error("ciphertext should not equal plaintext")
	}

	// Decrypt should recover original plaintext
	decrypted, err := key.Decrypt(ciphertext, iv)
	if err != nil {
		t.Fatalf("Decrypt failed: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptProducesUniqueIVs(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	defer key.Zero()

	plaintext := []byte("test")

	_, iv1, _ := key.Encrypt(plaintext)
	_, iv2, _ := key.Encrypt(plaintext)

	if bytes.Equal(iv1, iv2) {
		t.Error("each encryption should produce a unique IV")
	}
}

func TestDecryptWithWrongKey(t *testing.T) {
	key1 := DeriveKey("password1", []byte("0123456789abcdef"))
	key2 := DeriveKey("password2", []byte("0123456789abcdef"))
	defer key1.Zero()
	defer key2.Zero()

	plaintext := []byte("secret data")

	ciphertext, iv, _ := key1.Encrypt(plaintext)

	// Decrypting with wrong key should fail
	_, err := key2.Decrypt(ciphertext, iv)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed, got %v", err)
	}
}

func TestDecryptWithTamperedCiphertext(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	defer key.Zero()

	ciphertext, iv, _ := key.Encrypt([]byte("secret"))

	// Tamper with ciphertext
	ciphertext[0] ^= 0xFF

	_, err := key.Decrypt(ciphertext, iv)
	if err != ErrDecryptionFailed {
		t.Errorf("expected ErrDecryptionFailed for tampered data, got %v", err)
	}
}

func TestZeroedKeyCannotEncrypt(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	key.Zero()

	_, _, err := key.Encrypt([]byte("test"))
	if err != ErrKeyZeroed {
		t.Errorf("expected ErrKeyZeroed, got %v", err)
	}
}

func TestZeroedKeyCannotDecrypt(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	key.Zero()

	_, err := key.Decrypt([]byte("test"), make([]byte, IVSize))
	if err != ErrKeyZeroed {
		t.Errorf("expected ErrKeyZeroed, got %v", err)
	}
}

func TestGenerateSalt(t *testing.T) {
	salt1, err := GenerateSalt()
	if err != nil {
		t.Fatalf("GenerateSalt failed: %v", err)
	}

	if len(salt1) != SaltSize {
		t.Errorf("expected salt size %d, got %d", SaltSize, len(salt1))
	}

	salt2, _ := GenerateSalt()
	if bytes.Equal(salt1, salt2) {
		t.Error("each call should produce unique salt")
	}
}

func TestCreateCheckValue(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	defer key.Zero()

	checkB64, ivB64, err := key.CreateCheckValue()
	if err != nil {
		t.Fatalf("CreateCheckValue failed: %v", err)
	}

	// Should be valid base64
	check, err := base64.StdEncoding.DecodeString(checkB64)
	if err != nil {
		t.Errorf("check value is not valid base64: %v", err)
	}

	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		t.Errorf("IV is not valid base64: %v", err)
	}

	// Should be able to decrypt to known plaintext
	decrypted, err := key.Decrypt(check, iv)
	if err != nil {
		t.Fatalf("failed to decrypt check value: %v", err)
	}

	if string(decrypted) != CheckPlaintext {
		t.Errorf("expected %q, got %q", CheckPlaintext, decrypted)
	}
}

func TestVerifyPassword(t *testing.T) {
	password := "correct-password"
	salt, _ := GenerateSalt()
	saltB64 := base64.StdEncoding.EncodeToString(salt)

	key := DeriveKey(password, salt)
	checkB64, ivB64, _ := key.CreateCheckValue()
	key.Zero()

	// Correct password should verify
	ok, err := VerifyPassword(password, saltB64, checkB64, ivB64)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if !ok {
		t.Error("correct password should verify")
	}

	// Wrong password should not verify
	ok, err = VerifyPassword("wrong-password", saltB64, checkB64, ivB64)
	if err != nil {
		t.Fatalf("VerifyPassword failed: %v", err)
	}
	if ok {
		t.Error("wrong password should not verify")
	}
}

func TestDecodeIV(t *testing.T) {
	original := make([]byte, IVSize)
	for i := range original {
		original[i] = byte(i)
	}

	encoded := base64.StdEncoding.EncodeToString(original)

	decoded, err := DecodeIV(encoded)
	if err != nil {
		t.Fatalf("DecodeIV failed: %v", err)
	}

	if !bytes.Equal(decoded, original) {
		t.Error("decoded IV should match original")
	}
}

func TestDecodeIVInvalidSize(t *testing.T) {
	// Wrong size IV
	wrongSize := base64.StdEncoding.EncodeToString([]byte("short"))

	_, err := DecodeIV(wrongSize)
	if err == nil {
		t.Error("DecodeIV should fail for wrong size IV")
	}
}

func TestEncodeIV(t *testing.T) {
	iv := make([]byte, IVSize)
	for i := range iv {
		iv[i] = byte(i * 10)
	}

	encoded := EncodeIV(iv)
	decoded, _ := base64.StdEncoding.DecodeString(encoded)

	if !bytes.Equal(decoded, iv) {
		t.Error("EncodeIV should produce valid base64")
	}
}

func TestLargeDataEncryption(t *testing.T) {
	key := DeriveKey("password", []byte("0123456789abcdef"))
	defer key.Zero()

	// 1MB of data
	plaintext := make([]byte, 1024*1024)
	for i := range plaintext {
		plaintext[i] = byte(i % 256)
	}

	ciphertext, iv, err := key.Encrypt(plaintext)
	if err != nil {
		t.Fatalf("Encrypt failed for large data: %v", err)
	}

	decrypted, err := key.Decrypt(ciphertext, iv)
	if err != nil {
		t.Fatalf("Decrypt failed for large data: %v", err)
	}

	if !bytes.Equal(decrypted, plaintext) {
		t.Error("large data should decrypt correctly")
	}
}
