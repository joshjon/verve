package crypto

import (
	"testing"
)

func validKey() []byte {
	return []byte("0123456789abcdef0123456789abcdef") // exactly 32 bytes
}

func TestEncryptDecrypt_RoundTrip(t *testing.T) {
	key := validKey()
	plaintext := "ghp_mySecretToken12345"

	encrypted, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == "" {
		t.Fatal("expected non-empty ciphertext")
	}
	if encrypted == plaintext {
		t.Error("ciphertext should differ from plaintext")
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("expected %q, got %q", plaintext, decrypted)
	}
}

func TestEncrypt_DifferentCiphertexts(t *testing.T) {
	key := validKey()
	plaintext := "same-plaintext"

	c1, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt 1: %v", err)
	}
	c2, err := Encrypt(key, plaintext)
	if err != nil {
		t.Fatalf("encrypt 2: %v", err)
	}

	// Due to random nonces, each encryption should produce different ciphertext
	if c1 == c2 {
		t.Error("expected different ciphertexts for same plaintext (different nonces)")
	}
}

func TestEncrypt_InvalidKeyLength(t *testing.T) {
	shortKey := []byte("too-short")
	_, err := Encrypt(shortKey, "test")
	if err == nil {
		t.Error("expected error for short key")
	}
}

func TestDecrypt_InvalidKeyLength(t *testing.T) {
	shortKey := []byte("too-short")
	_, err := Decrypt(shortKey, "dGVzdA==")
	if err == nil {
		t.Error("expected error for short key")
	}
}

func TestDecrypt_InvalidBase64(t *testing.T) {
	key := validKey()
	_, err := Decrypt(key, "not-valid-base64!!!")
	if err == nil {
		t.Error("expected error for invalid base64")
	}
}

func TestDecrypt_TooShortCiphertext(t *testing.T) {
	key := validKey()
	// Base64 encode a very short string (shorter than nonce)
	_, err := Decrypt(key, "YQ==") // just "a"
	if err == nil {
		t.Error("expected error for ciphertext too short")
	}
}

func TestDecrypt_WrongKey(t *testing.T) {
	key1 := validKey()
	key2 := []byte("abcdefghijklmnopqrstuvwxyz012345") // different 32-byte key

	encrypted, err := Encrypt(key1, "secret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = Decrypt(key2, encrypted)
	if err == nil {
		t.Error("expected error decrypting with wrong key")
	}
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
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEncrypt_EmptyPlaintext(t *testing.T) {
	key := validKey()
	encrypted, err := Encrypt(key, "")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	decrypted, err := Decrypt(key, encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty string, got %q", decrypted)
	}
}
