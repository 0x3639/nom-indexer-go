package auth

import (
	"encoding/base64"
	"errors"
	"strconv"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestNewSigner_RejectsEmptySecret(t *testing.T) {
	if _, err := NewSigner(""); !errors.Is(err, ErrEmptySecret) {
		t.Errorf("expected ErrEmptySecret, got %v", err)
	}
}

func TestSigner_IssueAndVerifyRoundTrip(t *testing.T) {
	s, err := NewSigner("test-secret")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	token, err := s.Issue("admin", time.Hour, []string{"read", "write"})
	if err != nil {
		t.Fatalf("Issue: %v", err)
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	claims, err := s.Verify(token)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if claims.Subject != "admin" {
		t.Errorf("sub = %q, want admin", claims.Subject)
	}
	got := claims.Scopes()
	want := []string{"read", "write"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("Scopes() = %v, want %v", got, want)
	}
	if claims.ExpiresAt == nil {
		t.Fatal("ExpiresAt nil")
	}
	if claims.IssuedAt == nil {
		t.Fatal("IssuedAt nil")
	}
}

func TestSigner_Issue_RejectsBadInputs(t *testing.T) {
	s, _ := NewSigner("secret")

	if _, err := s.Issue("", time.Hour, nil); err == nil {
		t.Error("empty sub should error")
	}
	if _, err := s.Issue("admin", 0, nil); err == nil {
		t.Error("zero ttl should error")
	}
	if _, err := s.Issue("admin", -time.Second, nil); err == nil {
		t.Error("negative ttl should error")
	}
}

func TestSigner_Verify_RejectsExpired(t *testing.T) {
	s, _ := NewSigner("secret")

	// Mint a token whose exp is in the past by going through the library
	// directly (Issue refuses negative ttl).
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   "admin",
			IssuedAt:  jwt.NewNumericDate(now.Add(-2 * time.Hour)),
			ExpiresAt: jwt.NewNumericDate(now.Add(-time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, _ := tok.SignedString(s.secret)

	if _, err := s.Verify(signed); err == nil {
		t.Error("expected expired-token error, got nil")
	}
}

func TestSigner_Verify_RejectsWrongSecret(t *testing.T) {
	a, _ := NewSigner("secret-a")
	b, _ := NewSigner("secret-b")
	tok, _ := a.Issue("admin", time.Hour, nil)
	if _, err := b.Verify(tok); err == nil {
		t.Error("expected verify failure on wrong secret")
	}
}

func TestSigner_Verify_RejectsNoneAlgorithm(t *testing.T) {
	s, _ := NewSigner("secret")
	// Hand-craft an "alg: none" token to defeat the classic downgrade attack.
	enc := base64.RawURLEncoding.EncodeToString
	header := enc([]byte(`{"alg":"none","typ":"JWT"}`))
	payload := enc([]byte(`{"sub":"attacker","exp":` + strconv.FormatInt(time.Now().Add(time.Hour).Unix(), 10) + `}`))
	token := header + "." + payload + "."

	if _, err := s.Verify(token); err == nil {
		t.Error("alg=none must be rejected")
	}
}

func TestSigner_Verify_EmptyToken(t *testing.T) {
	s, _ := NewSigner("secret")
	if _, err := s.Verify(""); err == nil {
		t.Error("empty token should error")
	}
}

func TestClaims_Scopes_Empty(t *testing.T) {
	c := &Claims{}
	if got := c.Scopes(); got != nil {
		t.Errorf("expected nil scopes for empty Scope, got %v", got)
	}
}

func TestClaims_Scopes_MultipleSpaces(t *testing.T) {
	c := &Claims{Scope: "  read   write  admin  "}
	got := c.Scopes()
	if len(got) != 3 || got[0] != "read" || got[1] != "write" || got[2] != "admin" {
		t.Errorf("Scopes() with extra spaces = %v, want [read write admin]", got)
	}
}
