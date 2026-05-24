package auth

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload the API issues and verifies.
//
// Scope is rendered as the OAuth 2.0 standard space-separated "scope"
// claim in the on-wire JWT, but exposed to Go callers as a slice for
// ergonomics.
type Claims struct {
	jwt.RegisteredClaims
	// Scope is the OAuth 2.0 "scope" claim — space-separated on the wire,
	// e.g. "read write". Decoded into a slice on parse.
	Scope string `json:"scope,omitempty"`
}

// Scopes returns the parsed scope list.
func (c *Claims) Scopes() []string {
	if c.Scope == "" {
		return nil
	}
	return strings.Fields(c.Scope)
}

// Signer issues and verifies HS256 JWTs against a shared secret. It is safe
// for concurrent use; the secret is captured at construction.
type Signer struct {
	secret []byte
}

// ErrEmptySecret is returned by NewSigner when the secret is empty.
var ErrEmptySecret = errors.New("auth: jwt secret must not be empty")

// NewSigner returns a Signer that signs and verifies with the given HS256
// secret. Returns ErrEmptySecret if secret is empty.
func NewSigner(secret string) (*Signer, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}
	return &Signer{secret: []byte(secret)}, nil
}

// Issue mints a token for sub with the given TTL and scopes. Scopes are
// joined with spaces per OAuth 2.0. iat is set to now; exp to now+ttl.
func (s *Signer) Issue(sub string, ttl time.Duration, scopes []string) (string, error) {
	if sub == "" {
		return "", errors.New("auth: sub must not be empty")
	}
	if ttl <= 0 {
		return "", errors.New("auth: ttl must be positive")
	}
	now := time.Now().UTC()
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   sub,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
		Scope: strings.Join(scopes, " "),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("auth: sign token: %w", err)
	}
	return signed, nil
}

// Verify parses and validates a Bearer token. The returned *Claims carries
// the subject, expiry, and parsed scope list. Returns an error if the
// signature is invalid, the token is expired, or the algorithm is not HS256.
func (s *Signer) Verify(tokenString string) (*Claims, error) {
	if tokenString == "" {
		return nil, errors.New("auth: empty token")
	}
	parsed, err := jwt.ParseWithClaims(
		tokenString,
		&Claims{},
		func(t *jwt.Token) (interface{}, error) {
			// Hard-pin to HS256: reject any other algorithm (defeats the
			// classic "alg: none" / "alg: HS256" downgrade attacks).
			if t.Method != jwt.SigningMethodHS256 {
				return nil, fmt.Errorf("auth: unexpected signing method: %v", t.Header["alg"])
			}
			return s.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("auth: invalid token")
	}
	return claims, nil
}
