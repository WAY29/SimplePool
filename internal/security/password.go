package security

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"io"
	"strings"

	"github.com/WAY29/SimplePool/internal/apperr"
	"golang.org/x/crypto/bcrypt"
)

const sessionTokenBytes = 32

func HashPassword(password string) (string, error) {
	const op = "security.HashPassword"

	if strings.TrimSpace(password) == "" {
		return "", apperr.New(apperr.CodeSecurity, op, "password is required")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", apperr.Wrap(apperr.CodeSecurity, op, err)
	}

	return string(hash), nil
}

func VerifyPassword(hash, password string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func GenerateSessionToken(reader io.Reader) (string, error) {
	const op = "security.GenerateSessionToken"

	if reader == nil {
		reader = rand.Reader
	}

	buf := make([]byte, sessionTokenBytes)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", apperr.Wrap(apperr.CodeSecurity, op, err)
	}

	return base64.RawURLEncoding.EncodeToString(buf), nil
}

func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func VerifyTokenHash(expectedHash, token string) bool {
	actual := HashToken(token)
	return subtle.ConstantTimeCompare([]byte(expectedHash), []byte(actual)) == 1
}
