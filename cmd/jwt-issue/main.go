// jwt-issue mints HS256 JWTs for the nom-indexer-go API.
//
// Usage:
//
//	API_JWT_SECRET=... go run ./cmd/jwt-issue \
//	    --sub team-frontend \
//	    --ttl 24h \
//	    --scope read
//
// The signed token prints to stdout. The same secret must be loaded by the
// running API service (see internal/config APIConfig.JWTSecret).
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0x3639/nom-indexer-go/internal/auth"
)

func main() {
	sub := flag.String("sub", "", "subject (required) — identifies the client; appears as JWT 'sub'")
	ttl := flag.Duration("ttl", 24*time.Hour, "token lifetime, e.g. 24h, 30d (Go duration)")
	scope := flag.String("scope", "read", "comma-separated scope list, e.g. 'read,write'")
	flag.Parse()

	if *sub == "" {
		fmt.Fprintln(os.Stderr, "error: --sub is required")
		flag.Usage()
		os.Exit(2)
	}

	secret := os.Getenv("API_JWT_SECRET")
	if secret == "" {
		fmt.Fprintln(os.Stderr, "error: API_JWT_SECRET environment variable is required")
		os.Exit(2)
	}

	signer, err := auth.NewSigner(secret)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	scopes := splitScopes(*scope)
	token, err := signer.Issue(*sub, *ttl, scopes)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(token)
}

// splitScopes turns a "read,write,admin" CLI flag into ["read","write","admin"].
func splitScopes(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
