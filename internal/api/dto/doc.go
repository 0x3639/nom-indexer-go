// Package dto holds the JSON response shapes the API ships over the wire.
//
// DTOs are intentionally distinct from internal/models — the models use
// pgx db: tags and reflect the database row layout, while DTOs use json:
// tags and choose the wire-friendly representation. The most important
// divergence: BIGINT amounts ship as JSON strings ("100000000000") to
// dodge the JavaScript Number precision foot-gun (max safe integer is
// 2^53-1, well below int64).
//
// Every DTO type exposes a FromX(*models.X) *X constructor; a sibling
// test in dto_test.go asserts that every exported model field is consumed
// (so adding a column to the schema causes a compile or test failure
// here, not a silent omission).
package dto
