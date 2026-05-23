// Package middleware composes the HTTP middleware stack the API server
// uses. Each middleware is independent; they're applied in
// internal/api/router in the order: requestid → logger → recover →
// cors → ratelimit → auth (auth only on protected routes).
//
// All middleware that fails a request writes an RFC 7807
// application/problem+json body via internal/api/httpx.WriteProblem.
package middleware
