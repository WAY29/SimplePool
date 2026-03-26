package crypto_test

import (
	"bytes"
	"testing"

	appcrypto "github.com/WAY29/SimplePool/internal/crypto"
)

func TestAESGCMRoundTrip(t *testing.T) {
	cipher, err := appcrypto.NewAESGCM(bytes.Repeat([]byte{2}, 32))
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	ciphertext, nonce, err := cipher.Encrypt([]byte("secret-payload"), []byte("node:credential"))
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if bytes.Equal(ciphertext, []byte("secret-payload")) {
		t.Fatal("ciphertext unexpectedly equals plaintext")
	}

	plaintext, err := cipher.Decrypt(nonce, ciphertext, []byte("node:credential"))
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	if !bytes.Equal(plaintext, []byte("secret-payload")) {
		t.Fatalf("plaintext = %q, want %q", plaintext, "secret-payload")
	}
}

func TestAESGCMRejectsTampering(t *testing.T) {
	cipher, err := appcrypto.NewAESGCM(bytes.Repeat([]byte{3}, 32))
	if err != nil {
		t.Fatalf("NewAESGCM() error = %v", err)
	}

	ciphertext, nonce, err := cipher.Encrypt([]byte("secret-payload"), nil)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	tampered := append([]byte(nil), ciphertext...)
	tampered[0] ^= 0xFF

	if _, err := cipher.Decrypt(nonce, tampered, nil); err == nil {
		t.Fatal("Decrypt() error = nil, want error")
	}
}
