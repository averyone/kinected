package plaid

import (
	"crypto/rand"
	"encoding/base64"
	"testing"
)

func TestEncryptDecrypt(t *testing.T) {
	// Generate a test key
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	testCases := []struct {
		name      string
		plaintext string
	}{
		{"empty string", ""},
		{"short string", "test"},
		{"plaid token", "access-sandbox-12345678-1234-1234-1234-123456789012"},
		{"unicode", "テスト日本語"},
		{"long string", string(make([]byte, 1024))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			encrypted, err := encrypt(tc.plaintext, key)
			if err != nil {
				t.Fatalf("encrypt failed: %v", err)
			}

			if encrypted == tc.plaintext && tc.plaintext != "" {
				t.Error("encrypted should not equal plaintext")
			}

			decrypted, err := decrypt(encrypted, key)
			if err != nil {
				t.Fatalf("decrypt failed: %v", err)
			}

			if decrypted != tc.plaintext {
				t.Errorf("expected %q, got %q", tc.plaintext, decrypted)
			}
		})
	}
}

func TestEncryptDecrypt_DifferentOutputs(t *testing.T) {
	// Verify that encryption produces different outputs each time (due to nonce)
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	plaintext := "test-access-token"

	encrypted1, err := encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("first encrypt failed: %v", err)
	}

	encrypted2, err := encrypt(plaintext, key)
	if err != nil {
		t.Fatalf("second encrypt failed: %v", err)
	}

	if encrypted1 == encrypted2 {
		t.Error("two encryptions of same plaintext should produce different outputs")
	}

	// Both should decrypt to the same value
	decrypted1, _ := decrypt(encrypted1, key)
	decrypted2, _ := decrypt(encrypted2, key)

	if decrypted1 != decrypted2 {
		t.Errorf("decryptions don't match: %q vs %q", decrypted1, decrypted2)
	}
}

func TestEncrypt_InvalidKey(t *testing.T) {
	testCases := []struct {
		name string
		key  []byte
	}{
		{"empty key", []byte{}},
		{"key too short", []byte("short")},
		{"key too long", make([]byte, 64)},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := encrypt("test", tc.key)
			if err == nil {
				t.Error("expected error for invalid key")
			}
		})
	}
}

func TestDecrypt_InvalidData(t *testing.T) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}

	testCases := []struct {
		name string
		data string
	}{
		{"invalid base64", "not-valid-base64!!!"},
		{"too short", base64.StdEncoding.EncodeToString([]byte("short"))},
		{"corrupted", base64.StdEncoding.EncodeToString(make([]byte, 100))},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := decrypt(tc.data, key)
			if err == nil {
				t.Error("expected error for invalid data")
			}
		})
	}
}
