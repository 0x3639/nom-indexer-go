// Package auth issues and verifies HS256 JWTs for the nom-indexer-go API.
//
// Tokens are minted out-of-band by the admin via cmd/jwt-issue and presented
// by clients in the Authorization: Bearer <token> header on every protected
// request. The same shared secret (API_JWT_SECRET) signs and verifies, so
// rotating it requires re-minting every issued token.
//
// Claims follow OAuth 2.0 conventions: sub (subject), exp (expiry),
// iat (issued at), and scope (space-separated list).
package auth
