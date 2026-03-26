package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"fmt"
	"io"

	"github.com/WAY29/SimplePool/internal/apperr"
)

type AESGCM struct {
	aead cipher.AEAD
	rand io.Reader
}

func NewAESGCM(key []byte) (*AESGCM, error) {
	const op = "crypto.NewAESGCM"

	if len(key) != 32 {
		return nil, apperr.New(apperr.CodeSecurity, op, fmt.Sprintf("AES-GCM key must be 32 bytes, got %d", len(key)))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeSecurity, op, err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeSecurity, op, err)
	}

	return &AESGCM{
		aead: aead,
		rand: rand.Reader,
	}, nil
}

func (c *AESGCM) Encrypt(plaintext, aad []byte) ([]byte, []byte, error) {
	const op = "crypto.AESGCM.Encrypt"

	if c == nil || c.aead == nil {
		return nil, nil, apperr.New(apperr.CodeSecurity, op, "cipher is not initialized")
	}

	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(c.rand, nonce); err != nil {
		return nil, nil, apperr.Wrap(apperr.CodeSecurity, op, err)
	}

	return c.aead.Seal(nil, nonce, plaintext, aad), nonce, nil
}

func (c *AESGCM) Decrypt(nonce, ciphertext, aad []byte) ([]byte, error) {
	const op = "crypto.AESGCM.Decrypt"

	if c == nil || c.aead == nil {
		return nil, apperr.New(apperr.CodeSecurity, op, "cipher is not initialized")
	}

	if len(nonce) != c.aead.NonceSize() {
		return nil, apperr.New(apperr.CodeSecurity, op, "nonce size mismatch")
	}

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, aad)
	if err != nil {
		return nil, apperr.Wrap(apperr.CodeSecurity, op, err)
	}

	return plaintext, nil
}
