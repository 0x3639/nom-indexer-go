!!! warning "int64 cap on `*big.Int` values"
    Stored as `BIGINT`. Values larger than `math.MaxInt64` (≈9.22 × 10¹⁸) are
    silently capped — the conversion runs through
    [`safeBigIntToInt64`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/processor.go)
    which logs a warning and returns `math.MaxInt64`. ZNN/QSR amounts use 1e8
    satoshi scaling and stay well below the cap, but custom-token supplies
    that approach 9.22e18 satoshi will lose precision. Full discussion in
    `schema/conventions.md` under "int64 cap".
