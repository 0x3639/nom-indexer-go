---
title: ABI decoding
---

# ABI decoding

How raw account-block call data becomes `(method, inputs)` for the
per-contract handlers.

## The `TxData` model

In [`internal/models/models.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/models.go):

```go
type TxData struct {
    Method string             `json:"method"`
    Inputs map[string]string  `json:"inputs"`
}
```

Inputs are stringified — every ABI value is rendered to a string by
`formatArg` (in
[`internal/indexer/decoder.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder.go)).
This shape is what lands in
[`account_blocks.input`](../schema/account_blocks.md) as JSONB.

## Decode flow

`tryDecodeTxData(block)` in `decoder.go`:

1. Skip blocks with no `Data`.
2. Skip blocks whose `ToAddress` isn't an embedded contract
   (see [`docs/reference/addresses.md`](../reference/addresses.md)).
3. Try the **Common** ABI first — methods like `CollectReward`,
   `WithdrawQsr` live there.
4. On miss, switch on `ToAddress` to pick the contract-specific ABI
   (`embedded.Plasma`, `embedded.Pillar`, `embedded.Token`, etc.).
5. `tryDecodeFromAbi(data, abi)` walks the ABI's function entries:
    - Match the first 4 bytes of `data` against the entry's
      `EncodeSignature()[:4]`.
    - On match, set `Method = entry.Name` and decode `data` through
      `abi.DecodeFunction`.
    - Populate `Inputs[param.Name] = formatArg(arg)` for each named
      input.

## `formatArg`

Stringifies arbitrary ABI values:

- `[]byte` → string form (UTF-8 reinterpret).
- `string` → as-is.
- everything else → `fmt.Sprintf("%v", arg)`.

This is lossy for binary inputs but matches the JSONB-friendly shape
the rest of the schema expects.

## Where decoded data goes

- `account_blocks.method` — the resolved method name.
- `account_blocks.input` — JSON-encoded `TxData.Inputs`, sanitized via
  `sanitizeJSONForPostgres` (NUL bytes and literal ` ` escapes are
  stripped because PG rejects them in JSONB).
- The dispatch in `embedded.go` uses `txData.Method` and reads
  `txData.Inputs["…"]` per contract.

## Pillar-name enrichment

For Pillar contract methods that take a `name` input (`Register`,
`RegisterLegacy`, `UpdatePillar`, `Revoke`, `Delegate`), the
processAccountBlocks pre-pass injects a synthetic `pillarOwner` key
into `txData.Inputs` by looking up the cached `pillarNameToOwner` map.
This avoids re-doing the map lookup in the handler.

## SDK accelerator types issue

A historical SDK bug caused panics when decoding accelerator project
data with phases. The current checkout uses upstream
`github.com/0x3639/znn-sdk-go` directly; there is no local SDK replace
or `vendor-sdk/` directory. See
[`docs/reference/known-issues.md`](../reference/known-issues.md) for
the history and what to check if the panic returns after an SDK bump.

## Tests

- [`internal/indexer/decoder_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder_test.go) — `formatArg` table-driven tests.
- [`internal/indexer/decoder_real_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder_real_test.go) — encode/decode roundtrips against real ABIs (`Delegate`, `VoteByName`, plus error cases).
- [`internal/repository/account_block_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/account_block_test.go) — `sanitizeJSONForPostgres`.
