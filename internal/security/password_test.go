package security_test

import (
	"crypto/rand"
	"testing"

	"github.com/WAY29/SimplePool/internal/security"
)

func TestHashPasswordUsesSaltAndVerifies(t *testing.T) {
	hashA, err := security.HashPassword("super-secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	hashB, err := security.HashPassword("super-secret")
	if err != nil {
		t.Fatalf("HashPassword() error = %v", err)
	}

	if hashA == "super-secret" || hashB == "super-secret" {
		t.Fatal("password hash leaks plaintext")
	}

	if hashA == hashB {
		t.Fatal("password hash should include salt")
	}

	if err := security.VerifyPassword(hashA, "super-secret"); err != nil {
		t.Fatalf("VerifyPassword() error = %v", err)
	}

	if err := security.VerifyPassword(hashA, "wrong-secret"); err == nil {
		t.Fatal("VerifyPassword() error = nil, want error")
	}
}

func TestSessionTokenHashingAndVerification(t *testing.T) {
	tokenA, err := security.GenerateSessionToken(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateSessionToken() error = %v", err)
	}

	tokenB, err := security.GenerateSessionToken(rand.Reader)
	if err != nil {
		t.Fatalf("GenerateSessionToken() error = %v", err)
	}

	if tokenA == tokenB {
		t.Fatal("generated tokens should differ")
	}

	hash := security.HashToken(tokenA)
	if hash == tokenA {
		t.Fatal("HashToken() leaks plaintext")
	}

	if !security.VerifyTokenHash(hash, tokenA) {
		t.Fatal("VerifyTokenHash() = false, want true")
	}

	if security.VerifyTokenHash(hash, tokenB) {
		t.Fatal("VerifyTokenHash() = true for mismatched token")
	}
}
