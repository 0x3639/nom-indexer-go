package dto

import "strconv"

// Amount is a wire-safe representation of an int64 token amount. The
// indexer stores raw integer amounts (no decimals applied) and a single
// ZNN total supply (~9e16) already exceeds JavaScript's Number.MAX_SAFE_INTEGER
// (2^53-1 ≈ 9.007e15). Marshaling as a JSON string is the standard
// crypto-API workaround; clients in numeric-only languages can still
// strconv it.
//
// Use AmountFromInt64(x) when constructing DTOs; never assign raw int64
// to a JSON-exposed field that may hold amounts.
type Amount string

func AmountFromInt64(v int64) Amount {
	return Amount(strconv.FormatInt(v, 10))
}
