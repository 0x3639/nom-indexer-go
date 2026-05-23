---
title: Token contract
---

# Token contract

## Contract address

`z1qxemdeddedxt0kenxxxxxxxxxxxxxxxxh9amk0` — `TokenAddress`.

## Methods observed

Handler: `indexTokenContract` in `embedded.go`.

| Method | Inputs | Triggers |
|---|---|---|
| `Mint` | `tokenStandard`, `amount`, `receiveAddress` | Insert into [`token_mints`](../schema/token_mints.md). |
| `Burn` | (none) | Insert into [`token_burns`](../schema/token_burns.md); update `tokens.total_burned`. |
| `UpdateToken` | `tokenStandard`, `owner`, `isMintable`, `isBurnable` | Set `tokens.last_update_timestamp`. |

Token registration (`Issue`) is captured indirectly: `processAccountBlocks`
upserts a [`tokens`](../schema/tokens.md) row whenever a block carries a
`TokenInfo` field, regardless of the method.

## Per-method write effects

- **Mint**
    - Requires `block.PairedAccountBlock`.
    - `tokenStandard`, `amount`, `receiver` from decoded inputs;
      `amount` parsed via `strconv.ParseInt` (warning logged + return
      on parse error).
    - `issuer` = `block.PairedAccountBlock.Address` (the embedded
      contract or token owner that called Mint).
    - `token_mints`: `InsertMintBatch`, idempotent on
      `account_block_hash`.
- **Burn**
    - Requires `block.PairedAccountBlock`.
    - `tokenStandard` + `amount` come from the paired send block (the
      `Burn` ABI has no inputs).
    - `token_burns`: `InsertBurnBatch`.
    - `tokens.total_burned`: `UpdateBurnAmountBatch` adds `amount` to
      the running counter in the same batch transaction.
- **UpdateToken**
    - `tokens.last_update_timestamp`: bumped via
      `UpdateLastUpdateTimestampBatch`.

## Special computation

None — every value comes from either the decoded inputs or the paired
send block.

## Tests

- [`internal/indexer/decoder_real_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder_real_test.go) — `Delegate` end-to-end decode (representative of the same pattern Mint uses).
- [`internal/repository/integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go) — `TestIntegration_TokenEvents_MintBurnRoundTrip` and `TestIntegration_TokenEvents_SumDailyMintsBurns`.

## Notes

`token_mints.issuer` is the **direct** caller of `Mint`. For
embedded-contract reward mints this is the reward source (Pillar,
Sentinel, Stake, or Liquidity contract address). For owner-initiated
mints it's the token owner. The reward classifier in
[`rewards.md`](rewards.md) reads `issuer` to attribute rewards.
