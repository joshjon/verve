package crypto

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef") // exactly 32 bytes
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := validKey()
	plaintext := "ghp_mySecretToken12345"

	encrypted, err := Encrypt(key, plaintext)
	require.NoError(t, err, "encrypt")
	assert.NotEmpty(t, encrypted, "expected non-empty ciphertext")
	assert.NotEqual(t, plaintext, encrypted, "ciphertext should differ from plaintext")

	decrypted, err := Decrypt(key, encrypted)
	require.NoError(t, err, "decrypt")
	assert.Equal(t, plaintext, decrypted)
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	key := validKey()
	plaintext := "same-plaintext"

	c1, err := Encrypt(key, plaintext)
	require.NoError(t, err, "encrypt 1")
	c2, err := Encrypt(key, plaintext)
	require.NoError(t, err, "encrypt 2")

	// Due to random nonces, each encryption should produce different ciphertext
	assert.NotEqual(t, c1, c2, "expected different ciphertexts for same plaintext (different nonces)")
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	shortKey := []byte("too-short")
	_, err := Encrypt(shortKey, "test")
	assert.Error(t, err, "expected error for short key")
}

func TestDecrypt_InvalidKeyLength(t *testing.T) {
	shortKey := []byte("too-short")
	_, err := Decrypt(shortKey, "dGVzdA==")
	assert.Error(t, err, "expected error for short key")
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := validKey()
	_, err := Decrypt(key, "not-valid-base64!!!")
	assert.Error(t, err, "expected error for invalid base64")
}

func TestDecrypt_TooShortCiphertext(t *testing.T) {
	key := validKey()
	// Base64 encode a very short string (shorter than nonce)
	_, err := Decrypt(key, "YQ==") // just "a"
	assert.Error(t, err, "expected error for ciphertext too short")
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := validKey()
	key2 := []byte("abcdefghijklmnopqrstuvwxyz012345") // different 32-byte key

	encrypted, err := Encrypt(key1, "secret")
	require.NoError(t, err, "encrypt")

	_, err = Decrypt(key2, encrypted)
	assert.Error(t, err, "expected error decrypting with wrong key")
}

func TestValidateKey(t *testing.T) {
	tests := []struct {
		name    string
		key     []byte
		wantErr bool
	}{
		{"valid 32 bytes", validKey(), false},
		{"too short", []byte("short"), true},
		{"too long", make([]byte, 64), true},
		{"empty", []byte{}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateKey(tt.key)
			if tt.wantErr {
				assert.Error(t, err, "ValidateKey() expected error")
			} else {
				assert.NoError(t, err, "ValidateKey() unexpected error")
			}
		})
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	key := validKey()
	encrypted, err := Encrypt(key, "")
	require.NoError(t, err, "encrypt")

	decrypted, err := Decrypt(key, encrypted)
	require.NoError(t, err, "decrypt")
	assert.Equal(t, "", decrypted, "expected empty string")
}
