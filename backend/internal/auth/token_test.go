package auth

import (
	"errors"
	"testing"
	"time"
)

func TestTokenRoundTrip(t *testing.T) {
	m := NewManager("test-secret", time.Hour)
	tok, err := m.Generate(42)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	got, err := m.Parse(tok)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 42 {
		t.Errorf("user id = %d, want 42", got)
	}
}

func TestTokenExpired(t *testing.T) {
	m := NewManager("test-secret", -time.Hour) // already expired
	tok, _ := m.Generate(1)
	if _, err := m.Parse(tok); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for expired token, got %v", err)
	}
}

func TestTokenWrongSecret(t *testing.T) {
	signer := NewManager("secret-a", time.Hour)
	verifier := NewManager("secret-b", time.Hour)
	tok, _ := signer.Generate(7)
	if _, err := verifier.Parse(tok); !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected ErrInvalidToken for wrong secret, got %v", err)
	}
}

func TestPasswordHashing(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if err := CheckPassword(hash, "correct horse battery staple"); err != nil {
		t.Errorf("expected match, got %v", err)
	}
	if err := CheckPassword(hash, "wrong"); err == nil {
		t.Error("expected mismatch error, got nil")
	}
}
