---
title: Add a contract handler
---

# Add a contract handler

How to wire indexing for a new embedded contract. The recipe assumes
the contract already exists on-chain and the SDK exposes its ABI; if
not, that's prerequisite work.

## 1. Register the address

Add a constant to
[`internal/models/constants.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/models/constants.go):

```go
const (
    // ... existing constants ...
    FooAddress = "z1qx…"  // the new embedded contract
)
```

Add it to `embeddedContractAddresses` so the decoder will try to
parse method calls targeting this address.

## 2. Add the dispatch case

In
[`internal/indexer/embedded.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/embedded.go),
extend `indexEmbeddedContracts`:

```go
switch block.Address.String() {
case models.PillarAddress:
    i.indexPillarContract(ctx, batch, block, txData, m)
// ...
case models.FooAddress:
    i.indexFooContract(ctx, batch, block, txData, m)
}
```

## 3. Write the handler

```go
func (i *Indexer) indexFooContract(
    ctx context.Context, batch *pgx.Batch,
    block *api.AccountBlock, txData *models.TxData, m *api.Momentum,
) {
    switch txData.Method {
    case "Bar":
        // Read decoded inputs:
        amount := txData.Inputs["amount"]
        // Resolve paired send if needed:
        if block.PairedAccountBlock == nil { return }
        // Queue a batched write through the relevant repository.
        i.repos.Foo.InsertBatch(batch, &models.Foo{ /* … */ })
    case "Baz":
        // ...
    }
}
```

Patterns to mirror from existing handlers:

- **Bail out early** on missing PairedAccountBlock when the handler
  needs it.
- **`strconv.ParseInt`** for `amount` inputs; log and return on error.
- **Use `safeBigIntToInt64`** if the value comes from `*big.Int`.
- **Use the batch parameter** — never call repository methods that
  open their own transactions.

## 4. Add the repository (if you need a new table)

See [`add-table.md`](add-table.md).

## 5. Document the contract

Add `docs/indexing/foo-contract.md` following the standard template:

1. Contract address (link to `internal/models/constants.go`).
2. Methods observed (table: method | inputs | triggers).
3. Per-method write effects.
4. Special computation (cancel_id / voting_id / etc.).
5. Tests (links to relevant `_test.go` files).

Also add an entry to `docs/indexing/index.md`'s contract table and
`mkdocs.yml`'s nav.

## 6. Add tests

Unit tests for any pure helpers in
[`internal/indexer/decoder_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/indexer/decoder_test.go)
or a new sibling. Integration tests for round-trips in
[`internal/repository/integration_new_tables_test.go`](https://github.com/0x3639/nom-indexer-go/blob/main/internal/repository/integration_new_tables_test.go)
if you added a table.

## 7. Update the LLM index

```bash
.venv-docs/bin/python scripts/docs/gen-llms-txt.py
.venv-docs/bin/python scripts/docs/gen-llms-full.py
.venv-docs/bin/mkdocs build --strict
```

CI fails if these are out of sync.

## Reference

`indexPillarContract` and `indexTokenContract` are the most idiomatic
handlers to copy. `indexAcceleratorContract` shows the pattern for
ABI-derived secondary IDs (`voting_id`).
