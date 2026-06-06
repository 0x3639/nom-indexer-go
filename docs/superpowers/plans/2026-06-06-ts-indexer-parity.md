# TS-Indexer Parity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close the four real indexing gaps the TypeScript indexer (`digitalSloth/nom-ts-indexer`) covers that our Go indexer does not: HTLC contract indexing, legacy Swap contract indexing, bridge time-challenge tracking, and a webhook/event-push subsystem.

**Architecture:** Each phase follows the established Go-indexer pattern — a numbered SQL migration, a flat model struct, a pool-backed repository with `Insert`/`InsertBatch`/`Update` methods using `ON CONFLICT` idioms, and either (a) a per-account-block handler dispatched from `indexEmbeddedContracts` (HTLC, Swap) or (b) an RPC-poll hook in the bridge sync loop (time challenges). The webhook phase adds a fire-and-forget dispatcher invoked after each momentum's transaction commits. All four phases are independent and can ship separately.

**Tech Stack:** Go, pgx/v5 (batch + pool), golang-migrate (file source), viper (config), `github.com/0x3639/znn-sdk-go/embedded` (contract ABIs) and `/api/embedded` (RPC), zap (logging). Tests: stdlib `testing` + integration tests gated by `//go:build integration` and `TEST_DATABASE_URL`.

---

## Context: grounding the implementer

Read these before starting — they are the patterns every phase mirrors:

| Concern | Reference file:line |
|---|---|
| Contract dispatch switch | `internal/indexer/embedded.go:14-36` (`indexEmbeddedContracts`) |
| Full handler example (Stake/Cancel) | `internal/indexer/embedded.go:136-175` (`indexStakeContract`) |
| Cancel-ID / ABI encode-decode | `internal/indexer/indexer.go:1105-1135` (`getStakeCancelID`) |
| Decoder (ABIs already mapped) | `internal/indexer/decoder.go` — HTLC/Swap/Bridge cases already present |
| Migration runner | `internal/database/migrations.go` (golang-migrate, `file://` source) |
| Migration files | `migrations/NNN_*.up.sql` / `.down.sql` (latest is `013_indexer_sync_status`) |
| Repository pattern | `internal/repository/stake.go` (full file) |
| Model + RewardType enum | `internal/models/models.go` |
| Bridge RPC sync loop | `internal/indexer/indexer.go:692-878` (`runBridgeSyncLoop` → `syncBridgeData` → `updateBridgeConfig`) |
| Config (viper) | `internal/config/config.go` (struct + `load`) |
| Integration tests | `internal/repository/integration_test.go` (`TestMain`, `newTestDB`) |
| Overflow conversion | `safeBigIntToInt64` (used in handlers, e.g. `embedded.go:151`) |

**Key facts established during research:**

- `internal/indexer/decoder.go` **already** selects `embedded.Htlc`, `embedded.Swap`, and `embedded.Bridge` ABIs and already tries `embedded.Common` first (so time-challenge/common methods decode). Address constants `models.HtlcAddress`, `models.SwapAddress`, `models.BridgeAddress` **already exist**. What is missing is the per-contract `case` in `indexEmbeddedContracts` plus the tables/repos/handlers.
- Contract addresses (from go-zenon `common/types/address.go`):
  - HTLC: `z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw`
  - Swap: `z1qxemdeddedxswapxxxxxxxxxxxxxxxxxxl4yww`
  - Bridge: `z1qxemdeddedxdrydgexxxxxxxxxxxxxxxmqgr0d`
- HTLC ABI (`embedded.Htlc`): `Create(hashLocked address, expirationTime int64, hashType uint8, keyMaxSize uint8, hashLock bytes)`, `Unlock(id hash, preimage bytes)`, `Reclaim(id hash)`, `DenyProxyUnlock()`, `AllowProxyUnlock()`. `HtlcInfo` fields: id, timeLocked, hashLocked, tokenStandard, amount, expirationTime (unix s), hashType (0=SHA3,1=SHA256), keyMaxSize, hashLock (bytes). **No enumeration RPC** — only `HtlcApi.GetById(id)`. So index from blocks, not polling.
- Swap ABI (`embedded.Swap`): only `RetrieveAssets(publicKey string, signature string)`. `SwapApi.GetAssets()` enumerates remaining `{Znn, Qsr}` keyed by `keyIdHash`. Legacy, mostly-static dataset.
- Time challenges are a **shared Common mechanism**. `BridgeApi.GetTimeChallengesInfo() (*TimeChallengesList, error)` returns `{Count, List[]{MethodName, ParamsHash, ChallengeStartHeight}}`. Challenged bridge methods: `SetTokenPair`, `ChangeTssECDSAPubKey`, `ChangeAdministrator`, `NominateGuardians`. A challenge row disappears once executed/expired, so authoritative state must come from the poll (delete rows absent from the latest list).

**Conventions (from `CLAUDE.md` / `docs/schema/conventions.md`):** all amounts/balances int64 BIGINT via `safeBigIntToInt64`; all timestamps Unix seconds BIGINT; all hashes 64-char lowercase hex TEXT; `TEXT NOT NULL DEFAULT ''` instead of NULL; no FK constraints; per-momentum writes are transactional (handlers enqueue on the shared `*pgx.Batch`).

**Verification commands (run from repo root):**

```bash
GOWORK=off go build ./...
GOWORK=off go test ./...
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/...
```

**Per the user's workflow (memory `feedback_branch_pr_workflow`):** implement on this branch (`feat/ts-indexer-parity`); do **not** push to GitHub until explicitly asked; Codex review is the QA gate. Each phase ends in a commit. Schema changes are public-API changes — add a `docs/schema/<table>.md` page per new table and regenerate the docs indexes (`python scripts/docs/gen-llms-txt.py && python scripts/docs/gen-llms-full.py`) before the phase's final commit.

---

## Phase 1 — HTLC contract indexing

**What:** Index the HTLC (hash-time-locked contract) lifecycle from account blocks: `Create` opens an entry; `Unlock` settles it with a preimage; `Reclaim` returns funds to the time-locked party after expiry. Net-new — neither indexer covers HTLC today.

**Approach:** Per-account-block decoding, modeled on `indexStakeContract`. One table `htlcs`, one model, one repository, one handler, one dispatch case.

### Task 1.1: Migration — `htlcs` table

**Files:**
- Create: `migrations/014_htlcs.up.sql`
- Create: `migrations/014_htlcs.down.sql`

- [ ] **Step 1: Write `migrations/014_htlcs.up.sql`**

```sql
-- migrations/014_htlcs.up.sql
-- HTLC (hash-time-locked contract) entries, indexed from account blocks.
-- An HTLC is Created by the time-locked party, Unlocked by the hash-locked
-- party with a preimage, or Reclaimed by the time-locked party after expiry.
CREATE TABLE IF NOT EXISTS htlcs (
    id                    TEXT PRIMARY KEY,           -- creating account-block hash (64-hex)
    time_locked_address   TEXT   NOT NULL DEFAULT '', -- can Reclaim after expiry (sender)
    hash_locked_address   TEXT   NOT NULL DEFAULT '', -- can Unlock with preimage
    token_standard        TEXT   NOT NULL DEFAULT '',
    amount                BIGINT NOT NULL DEFAULT 0,
    expiration_timestamp  BIGINT NOT NULL DEFAULT 0,  -- Unix seconds
    hash_type             SMALLINT NOT NULL DEFAULT 0,-- 0=SHA3, 1=SHA256
    key_max_size          SMALLINT NOT NULL DEFAULT 0,
    hash_lock             TEXT   NOT NULL DEFAULT '', -- hex-encoded
    status                SMALLINT NOT NULL DEFAULT 0,-- 0=active,1=unlocked,2=reclaimed
    preimage              TEXT   NOT NULL DEFAULT '', -- hex-encoded, set on Unlock
    creation_momentum_height    BIGINT NOT NULL DEFAULT 0,
    creation_momentum_timestamp BIGINT NOT NULL DEFAULT 0,
    settle_momentum_height      BIGINT NOT NULL DEFAULT 0, -- height of Unlock/Reclaim
    settle_momentum_timestamp   BIGINT NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_htlcs_time_locked ON htlcs (time_locked_address);
CREATE INDEX IF NOT EXISTS idx_htlcs_hash_locked ON htlcs (hash_locked_address);
CREATE INDEX IF NOT EXISTS idx_htlcs_status ON htlcs (status);
```

- [ ] **Step 2: Write `migrations/014_htlcs.down.sql`**

```sql
-- migrations/014_htlcs.down.sql
DROP TABLE IF EXISTS htlcs;
```

- [ ] **Step 3: Verify migration applies against the test DB**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Momentum_InsertAndGet -v
```
Expected: PASS (TestMain runs migrations up to 014; no migration error).

- [ ] **Step 4: Commit**

```bash
git add migrations/014_htlcs.up.sql migrations/014_htlcs.down.sql
git commit -m "feat(htlc): add htlcs table migration"
```

### Task 1.2: Model — `Htlc` + status enum

**Files:**
- Modify: `internal/models/models.go` (append)

- [ ] **Step 1: Append the model and status constants to `internal/models/models.go`**

```go
// HtlcStatus represents the lifecycle state of an HTLC entry.
type HtlcStatus int16

const (
	HtlcStatusActive    HtlcStatus = 0
	HtlcStatusUnlocked  HtlcStatus = 1
	HtlcStatusReclaimed HtlcStatus = 2
)

// Htlc represents a hash-time-locked contract entry.
type Htlc struct {
	ID                        string `db:"id"`
	TimeLockedAddress         string `db:"time_locked_address"`
	HashLockedAddress         string `db:"hash_locked_address"`
	TokenStandard             string `db:"token_standard"`
	Amount                    int64  `db:"amount"`
	ExpirationTimestamp       int64  `db:"expiration_timestamp"`
	HashType                  int16  `db:"hash_type"`
	KeyMaxSize                int16  `db:"key_max_size"`
	HashLock                  string `db:"hash_lock"`
	Status                    int16  `db:"status"`
	Preimage                  string `db:"preimage"`
	CreationMomentumHeight    int64  `db:"creation_momentum_height"`
	CreationMomentumTimestamp int64  `db:"creation_momentum_timestamp"`
	SettleMomentumHeight      int64  `db:"settle_momentum_height"`
	SettleMomentumTimestamp   int64  `db:"settle_momentum_timestamp"`
}
```

- [ ] **Step 2: Verify it compiles**

Run: `GOWORK=off go build ./internal/models/...`
Expected: no output (success).

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(htlc): add Htlc model and status enum"
```

### Task 1.3: Repository — `HtlcRepository`

**Files:**
- Create: `internal/repository/htlc.go`
- Test: `internal/repository/htlc_integration_test.go`

- [ ] **Step 1: Write the failing integration test `internal/repository/htlc_integration_test.go`**

```go
//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_Htlc_InsertSettleList(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewHtlcRepository(pool)

	h := &models.Htlc{
		ID:                        "a1",
		TimeLockedAddress:         "z1qsender",
		HashLockedAddress:         "z1qreceiver",
		TokenStandard:             models.ZnnTokenStandard,
		Amount:                    500,
		ExpirationTimestamp:       1700001000,
		HashType:                  0,
		KeyMaxSize:                32,
		HashLock:                  "deadbeef",
		Status:                    int16(models.HtlcStatusActive),
		CreationMomentumHeight:    10,
		CreationMomentumTimestamp: 1700000000,
	}
	if err := repo.Insert(ctx, h); err != nil {
		t.Fatalf("insert: %v", err)
	}
	// idempotent re-insert
	if err := repo.Insert(ctx, h); err != nil {
		t.Fatalf("re-insert: %v", err)
	}

	if err := repo.Settle(ctx, "a1", int16(models.HtlcStatusUnlocked), "c0ffee", 11, 1700000100); err != nil {
		t.Fatalf("settle: %v", err)
	}

	got, err := repo.GetByID(ctx, "a1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != int16(models.HtlcStatusUnlocked) || got.Preimage != "c0ffee" || got.SettleMomentumHeight != 11 {
		t.Errorf("settle not applied: %+v", got)
	}

	list, total, err := repo.List(ctx, ListOpts{Limit: 10, Offset: 0})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if total != 1 || len(list) != 1 {
		t.Errorf("list total=%d len=%d, want 1/1", total, len(list))
	}
}
```

- [ ] **Step 2: Run it to verify it fails to compile**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Htlc -v
```
Expected: FAIL — `undefined: NewHtlcRepository`.

- [ ] **Step 3: Write `internal/repository/htlc.go`**

```go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type HtlcRepository struct {
	pool *pgxpool.Pool
}

func NewHtlcRepository(pool *pgxpool.Pool) *HtlcRepository {
	return &HtlcRepository{pool: pool}
}

const htlcCols = `id, time_locked_address, hash_locked_address, token_standard, amount,
	expiration_timestamp, hash_type, key_max_size, hash_lock, status, preimage,
	creation_momentum_height, creation_momentum_timestamp,
	settle_momentum_height, settle_momentum_timestamp`

func (r *HtlcRepository) Insert(ctx context.Context, h *models.Htlc) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO htlcs (`+htlcCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO NOTHING`,
		h.ID, h.TimeLockedAddress, h.HashLockedAddress, h.TokenStandard, h.Amount,
		h.ExpirationTimestamp, h.HashType, h.KeyMaxSize, h.HashLock, h.Status, h.Preimage,
		h.CreationMomentumHeight, h.CreationMomentumTimestamp,
		h.SettleMomentumHeight, h.SettleMomentumTimestamp)
	return err
}

// InsertBatch enqueues an HTLC Create on the per-momentum batch.
func (r *HtlcRepository) InsertBatch(batch *pgx.Batch, h *models.Htlc) {
	batch.Queue(`
		INSERT INTO htlcs (`+htlcCols+`)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)
		ON CONFLICT (id) DO NOTHING`,
		h.ID, h.TimeLockedAddress, h.HashLockedAddress, h.TokenStandard, h.Amount,
		h.ExpirationTimestamp, h.HashType, h.KeyMaxSize, h.HashLock, h.Status, h.Preimage,
		h.CreationMomentumHeight, h.CreationMomentumTimestamp,
		h.SettleMomentumHeight, h.SettleMomentumTimestamp)
}

// Settle marks an HTLC unlocked (with preimage) or reclaimed.
func (r *HtlcRepository) Settle(ctx context.Context, id string, status int16, preimage string, height, ts int64) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE htlcs SET status = $2, preimage = $3,
			settle_momentum_height = $4, settle_momentum_timestamp = $5
		WHERE id = $1`,
		id, status, preimage, height, ts)
	return err
}

// SettleBatch enqueues an Unlock/Reclaim settle on the per-momentum batch.
func (r *HtlcRepository) SettleBatch(batch *pgx.Batch, id string, status int16, preimage string, height, ts int64) {
	batch.Queue(`
		UPDATE htlcs SET status = $2, preimage = $3,
			settle_momentum_height = $4, settle_momentum_timestamp = $5
		WHERE id = $1`,
		id, status, preimage, height, ts)
}

func (r *HtlcRepository) GetByID(ctx context.Context, id string) (*models.Htlc, error) {
	var h models.Htlc
	err := r.pool.QueryRow(ctx, `SELECT `+htlcCols+` FROM htlcs WHERE id = $1`, id).Scan(
		&h.ID, &h.TimeLockedAddress, &h.HashLockedAddress, &h.TokenStandard, &h.Amount,
		&h.ExpirationTimestamp, &h.HashType, &h.KeyMaxSize, &h.HashLock, &h.Status, &h.Preimage,
		&h.CreationMomentumHeight, &h.CreationMomentumTimestamp,
		&h.SettleMomentumHeight, &h.SettleMomentumTimestamp)
	if err != nil {
		return nil, err
	}
	return &h, nil
}

func (r *HtlcRepository) List(ctx context.Context, opts ListOpts) ([]*models.Htlc, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT `+htlcCols+`, COUNT(*) OVER () AS total
		FROM htlcs ORDER BY creation_momentum_height DESC
		LIMIT $1 OFFSET $2`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Htlc
		total int64
	)
	for rows.Next() {
		var h models.Htlc
		if err := rows.Scan(&h.ID, &h.TimeLockedAddress, &h.HashLockedAddress, &h.TokenStandard, &h.Amount,
			&h.ExpirationTimestamp, &h.HashType, &h.KeyMaxSize, &h.HashLock, &h.Status, &h.Preimage,
			&h.CreationMomentumHeight, &h.CreationMomentumTimestamp,
			&h.SettleMomentumHeight, &h.SettleMomentumTimestamp, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &h)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM htlcs`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
```

- [ ] **Step 4: Run the test to verify it passes**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Htlc -v
```
Expected: PASS.

- [ ] **Step 5: Add `htlcs` to the truncate list in `internal/repository/integration_test.go`**

In the `TRUNCATE ...` statement inside `newTestDB`, add `htlcs,` to the table list (after `stakes,`).

- [ ] **Step 6: Commit**

```bash
git add internal/repository/htlc.go internal/repository/htlc_integration_test.go internal/repository/integration_test.go
git commit -m "feat(htlc): add HtlcRepository with insert/settle/list"
```

### Task 1.4: Wire `HtlcRepository` into the repository registry

**Files:**
- Modify: `internal/repository/repository.go` (or wherever the `Repositories` aggregate struct + constructor live — grep `Stake.*StakeRepository`)

- [ ] **Step 1: Locate the registry**

Run: `grep -rn "StakeRepository" internal/repository/*.go internal/indexer/*.go | grep -iE "struct|New.*Repositories|\.Stake ="`
Expected: identifies the aggregate struct (field `Stake *StakeRepository`) and its constructor.

- [ ] **Step 2: Add the field and constructor wiring**

In the aggregate `Repositories` struct, add alongside `Stake`:
```go
	Htlc *HtlcRepository
```
In the constructor that builds it (next to `Stake: NewStakeRepository(pool),`), add:
```go
		Htlc: NewHtlcRepository(pool),
```

- [ ] **Step 3: Verify build**

Run: `GOWORK=off go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/repository/
git commit -m "feat(htlc): register HtlcRepository in repository set"
```

### Task 1.5: Handler — `indexHtlcContract` + dispatch case

**Files:**
- Modify: `internal/indexer/embedded.go` (add handler + dispatch case)
- Test: `internal/indexer/htlc_test.go`

- [ ] **Step 1: Write a failing unit test `internal/indexer/htlc_test.go` for the hash-encoding helper**

```go
package indexer

import "testing"

// bytesToHex is the small helper the HTLC handler uses to store hashLock/preimage.
func TestBytesToHex(t *testing.T) {
	got := bytesToHex([]byte{0xde, 0xad, 0xbe, 0xef})
	if got != "deadbeef" {
		t.Errorf("bytesToHex = %q, want deadbeef", got)
	}
	if bytesToHex(nil) != "" {
		t.Errorf("bytesToHex(nil) should be empty")
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `GOWORK=off go test ./internal/indexer/ -run TestBytesToHex -v`
Expected: FAIL — `undefined: bytesToHex`.

- [ ] **Step 3: Add `indexHtlcContract`, the `bytesToHex` helper, and the dispatch case to `internal/indexer/embedded.go`**

Add the dispatch case inside `indexEmbeddedContracts`'s `switch address` (alongside the others):
```go
	case models.HtlcAddress:
		i.indexHtlcContract(ctx, batch, block, txData, m)
```

Add the handler and helper at the end of the file:
```go
// bytesToHex hex-encodes a byte slice; empty/nil yields "".
func bytesToHex(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	return hex.EncodeToString(b)
}

// indexHtlcContract handles HTLC Create / Unlock / Reclaim events.
//
// HTLC blocks arrive as ContractReceive on the HTLC address paired with a user
// send. The entry id is the paired send-block hash (same convention as Stake).
// Create carries the lock params + the send's amount/token/sender; Unlock and
// Reclaim carry the target id (and Unlock a preimage) to settle the entry.
func (i *Indexer) indexHtlcContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	if block.PairedAccountBlock == nil {
		return
	}
	paired := block.PairedAccountBlock

	switch txData.Method {
	case "Create":
		id := paired.Hash.String()
		expiration, _ := strconv.ParseInt(txData.Inputs["expirationTime"], 10, 64)
		hashType, _ := strconv.Atoi(txData.Inputs["hashType"])
		keyMaxSize, _ := strconv.Atoi(txData.Inputs["keyMaxSize"])
		amount := safeBigIntToInt64(paired.Amount, i.logger,
			"htlc amount overflow", zap.String("htlcID", id))

		h := &models.Htlc{
			ID:                        id,
			TimeLockedAddress:         paired.Address.String(), // sender can Reclaim
			HashLockedAddress:         txData.Inputs["hashLocked"],
			TokenStandard:             paired.TokenStandard.String(),
			Amount:                    amount,
			ExpirationTimestamp:       expiration,
			HashType:                  int16(hashType),
			KeyMaxSize:                int16(keyMaxSize),
			HashLock:                  hexMaybe(txData.Inputs["hashLock"]),
			Status:                    int16(models.HtlcStatusActive),
			CreationMomentumHeight:    int64(m.Height),
			CreationMomentumTimestamp: int64(m.TimestampUnix),
		}
		i.repos.Htlc.InsertBatch(batch, h)

	case "Unlock":
		id := txData.Inputs["id"]
		if id == "" {
			return
		}
		i.repos.Htlc.SettleBatch(batch, id, int16(models.HtlcStatusUnlocked),
			hexMaybe(txData.Inputs["preimage"]), int64(m.Height), int64(m.TimestampUnix))

	case "Reclaim":
		id := txData.Inputs["id"]
		if id == "" {
			return
		}
		i.repos.Htlc.SettleBatch(batch, id, int16(models.HtlcStatusReclaimed),
			"", int64(m.Height), int64(m.TimestampUnix))
	}
}

// hexMaybe normalizes an ABI-decoded bytes input (the decoder may hand back a
// raw string of bytes via formatArg) into lowercase hex. If the value already
// looks like hex it is returned lowercased; otherwise the bytes are encoded.
func hexMaybe(s string) string {
	if s == "" {
		return ""
	}
	if _, err := hex.DecodeString(s); err == nil {
		return strings.ToLower(s)
	}
	return hex.EncodeToString([]byte(s))
}
```

- [ ] **Step 4: Add imports to `internal/indexer/embedded.go`**

Ensure the import block includes `"encoding/hex"`, `"strconv"`, and `"strings"` (some may already be present — do not duplicate).

- [ ] **Step 5: Run the unit test + build**

Run: `GOWORK=off go test ./internal/indexer/ -run TestBytesToHex -v && GOWORK=off go build ./...`
Expected: PASS, then build success.

- [ ] **Step 6: Commit**

```bash
git add internal/indexer/embedded.go internal/indexer/htlc_test.go
git commit -m "feat(htlc): index Create/Unlock/Reclaim from account blocks"
```

### Task 1.6: Verify HTLC id linkage against real chain data

> The id convention (paired send-block hash) must match what `Unlock`/`Reclaim` reference, or settles will silently no-op. This task verifies against live data before declaring the phase done.

**Files:** none (investigation).

- [ ] **Step 1: Find a settled HTLC on-chain and confirm the id matches**

With the docker stack running and caught up, query for HTLC blocks and confirm a `Create` id equals the `id` input of its later `Unlock`/`Reclaim`:
```bash
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer -P pager=off -c "
SELECT method, input->>'id' AS id_input, hash, paired_account_block
FROM account_blocks
WHERE to_address = 'z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw'
   OR address    = 'z1qxemdeddedxhtlcxxxxxxxxxxxxxxxxxygecvw'
ORDER BY momentum_height DESC LIMIT 20;"
```
Expected: an `Unlock`/`Reclaim` row whose `id_input` equals a `Create`'s paired send-block hash. If they do NOT match, adjust the `Create` id derivation to use the contract response/descendant block hash (see `getStakeCancelID` for the ABI encode/decode pattern) and re-run Task 1.3's test.

- [ ] **Step 2: Document the finding in the table doc (next task) and commit if code changed**

### Task 1.7: Schema doc + docs index regen

**Files:**
- Create: `docs/schema/htlcs.md`
- Modify: docs indexes (generated)

- [ ] **Step 1: Write `docs/schema/htlcs.md`**

Follow the format of an existing page (e.g. `docs/schema/stakes.md`): a one-paragraph summary, a column table (name / type / nullable / default / description), the writer (`internal/indexer/embedded.go indexHtlcContract`), and the indexing reference (`docs/indexing/htlc-contract.md` if you add one). Record the id-linkage finding from Task 1.6.

- [ ] **Step 2: Regenerate indexes and preview**

Run:
```bash
python scripts/docs/gen-llms-txt.py
python scripts/docs/gen-llms-full.py
mkdocs build --strict
```
Expected: clean build, no warnings.

- [ ] **Step 3: Commit**

```bash
git add docs/ llms.txt llms-full.txt
git commit -m "docs(htlc): document htlcs table"
```

---

## Phase 2 — Swap contract indexing (legacy)

**What:** Index the legacy genesis Swap contract. Two data shapes: (a) `RetrieveAssets` events (a key holder claiming their genesis ZNN/QSR), captured per-block; (b) a periodic snapshot of remaining unswapped balances via `SwapApi.GetAssets()`.

**Approach:** A per-block handler for `RetrieveAssets` writing to `swap_retrievals`, plus an RPC-poll snapshot into `swap_assets` hooked into a low-frequency refresh (legacy data is near-static).

### Task 2.1: Migration — `swap_retrievals` + `swap_assets`

**Files:**
- Create: `migrations/015_swap.up.sql`
- Create: `migrations/015_swap.down.sql`

- [ ] **Step 1: Write `migrations/015_swap.up.sql`**

```sql
-- migrations/015_swap.up.sql
-- Legacy genesis-swap retrieval events (RetrieveAssets), one row per claim.
CREATE TABLE IF NOT EXISTS swap_retrievals (
    id                  TEXT PRIMARY KEY,           -- claiming account-block hash
    address             TEXT   NOT NULL DEFAULT '', -- recipient (claimant)
    public_key          TEXT   NOT NULL DEFAULT '',
    znn_amount          BIGINT NOT NULL DEFAULT 0,
    qsr_amount          BIGINT NOT NULL DEFAULT 0,
    momentum_height     BIGINT NOT NULL DEFAULT 0,
    momentum_timestamp  BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_swap_retrievals_address ON swap_retrievals (address);

-- Remaining (unswapped) genesis balances, snapshotted from swap.getAssets.
CREATE TABLE IF NOT EXISTS swap_assets (
    key_id_hash             TEXT PRIMARY KEY,        -- 64-hex storage key
    znn                     BIGINT NOT NULL DEFAULT 0,
    qsr                     BIGINT NOT NULL DEFAULT 0,
    last_updated_timestamp  BIGINT NOT NULL DEFAULT 0
);
```

- [ ] **Step 2: Write `migrations/015_swap.down.sql`**

```sql
-- migrations/015_swap.down.sql
DROP TABLE IF EXISTS swap_assets;
DROP TABLE IF EXISTS swap_retrievals;
```

- [ ] **Step 3: Verify migration applies**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Momentum -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add migrations/015_swap.up.sql migrations/015_swap.down.sql
git commit -m "feat(swap): add swap_retrievals and swap_assets tables"
```

### Task 2.2: Models — `SwapRetrieval`, `SwapAsset`

**Files:**
- Modify: `internal/models/models.go` (append)

- [ ] **Step 1: Append models**

```go
// SwapRetrieval is one RetrieveAssets claim against the legacy genesis swap.
type SwapRetrieval struct {
	ID                string `db:"id"`
	Address           string `db:"address"`
	PublicKey         string `db:"public_key"`
	ZnnAmount         int64  `db:"znn_amount"`
	QsrAmount         int64  `db:"qsr_amount"`
	MomentumHeight    int64  `db:"momentum_height"`
	MomentumTimestamp int64  `db:"momentum_timestamp"`
}

// SwapAsset is a remaining unswapped genesis balance keyed by keyIdHash.
type SwapAsset struct {
	KeyIDHash            string `db:"key_id_hash"`
	Znn                  int64  `db:"znn"`
	Qsr                  int64  `db:"qsr"`
	LastUpdatedTimestamp int64  `db:"last_updated_timestamp"`
}
```

- [ ] **Step 2: Verify build**

Run: `GOWORK=off go build ./internal/models/...`
Expected: success.

- [ ] **Step 3: Commit**

```bash
git add internal/models/models.go
git commit -m "feat(swap): add SwapRetrieval and SwapAsset models"
```

### Task 2.3: Repository — `SwapRepository`

**Files:**
- Create: `internal/repository/swap.go`
- Test: `internal/repository/swap_integration_test.go`

- [ ] **Step 1: Write the failing integration test `internal/repository/swap_integration_test.go`**

```go
//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_Swap_RetrievalAndAssetUpsert(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewSwapRepository(pool)

	if err := repo.InsertRetrieval(ctx, &models.SwapRetrieval{
		ID: "r1", Address: "z1qclaim", PublicKey: "pk", ZnnAmount: 100, QsrAmount: 200,
		MomentumHeight: 5, MomentumTimestamp: 1700000000,
	}); err != nil {
		t.Fatalf("insert retrieval: %v", err)
	}

	// Upsert asset twice; second should overwrite.
	if err := repo.UpsertAsset(ctx, &models.SwapAsset{KeyIDHash: "k1", Znn: 10, Qsr: 20, LastUpdatedTimestamp: 1}); err != nil {
		t.Fatalf("upsert asset 1: %v", err)
	}
	if err := repo.UpsertAsset(ctx, &models.SwapAsset{KeyIDHash: "k1", Znn: 7, Qsr: 0, LastUpdatedTimestamp: 2}); err != nil {
		t.Fatalf("upsert asset 2: %v", err)
	}

	a, err := repo.GetAsset(ctx, "k1")
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if a.Znn != 7 || a.Qsr != 0 || a.LastUpdatedTimestamp != 2 {
		t.Errorf("asset not upserted: %+v", a)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Swap -v
```
Expected: FAIL — `undefined: NewSwapRepository`.

- [ ] **Step 3: Write `internal/repository/swap.go`**

```go
package repository

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type SwapRepository struct {
	pool *pgxpool.Pool
}

func NewSwapRepository(pool *pgxpool.Pool) *SwapRepository {
	return &SwapRepository{pool: pool}
}

func (r *SwapRepository) InsertRetrieval(ctx context.Context, s *models.SwapRetrieval) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO swap_retrievals (id, address, public_key, znn_amount, qsr_amount,
			momentum_height, momentum_timestamp)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING`,
		s.ID, s.Address, s.PublicKey, s.ZnnAmount, s.QsrAmount, s.MomentumHeight, s.MomentumTimestamp)
	return err
}

// InsertRetrievalBatch enqueues a RetrieveAssets event on the per-momentum batch.
func (r *SwapRepository) InsertRetrievalBatch(batch *pgx.Batch, s *models.SwapRetrieval) {
	batch.Queue(`
		INSERT INTO swap_retrievals (id, address, public_key, znn_amount, qsr_amount,
			momentum_height, momentum_timestamp)
		VALUES ($1,$2,$3,$4,$5,$6,$7)
		ON CONFLICT (id) DO NOTHING`,
		s.ID, s.Address, s.PublicKey, s.ZnnAmount, s.QsrAmount, s.MomentumHeight, s.MomentumTimestamp)
}

func (r *SwapRepository) UpsertAsset(ctx context.Context, a *models.SwapAsset) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO swap_assets (key_id_hash, znn, qsr, last_updated_timestamp)
		VALUES ($1,$2,$3,$4)
		ON CONFLICT (key_id_hash) DO UPDATE SET
			znn = EXCLUDED.znn, qsr = EXCLUDED.qsr,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		a.KeyIDHash, a.Znn, a.Qsr, a.LastUpdatedTimestamp)
	return err
}

func (r *SwapRepository) GetAsset(ctx context.Context, keyIDHash string) (*models.SwapAsset, error) {
	var a models.SwapAsset
	err := r.pool.QueryRow(ctx, `
		SELECT key_id_hash, znn, qsr, last_updated_timestamp
		FROM swap_assets WHERE key_id_hash = $1`, keyIDHash).Scan(
		&a.KeyIDHash, &a.Znn, &a.Qsr, &a.LastUpdatedTimestamp)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (r *SwapRepository) ListRetrievals(ctx context.Context, opts ListOpts) ([]*models.SwapRetrieval, int64, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, address, public_key, znn_amount, qsr_amount, momentum_height, momentum_timestamp,
			COUNT(*) OVER () AS total
		FROM swap_retrievals ORDER BY momentum_height DESC
		LIMIT $1 OFFSET $2`, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.SwapRetrieval
		total int64
	)
	for rows.Next() {
		var s models.SwapRetrieval
		if err := rows.Scan(&s.ID, &s.Address, &s.PublicKey, &s.ZnnAmount, &s.QsrAmount,
			&s.MomentumHeight, &s.MomentumTimestamp, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &s)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		total, err = fallbackCount(ctx, r.pool, `SELECT COUNT(*) FROM swap_retrievals`)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}
```

- [ ] **Step 4: Run to verify it passes**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Swap -v
```
Expected: PASS.

- [ ] **Step 5: Add `swap_retrievals, swap_assets` to the truncate list in `integration_test.go`; register `Swap *SwapRepository` in the repository set (same as Task 1.4).**

- [ ] **Step 6: Commit**

```bash
git add internal/repository/ 
git commit -m "feat(swap): add SwapRepository and register it"
```

### Task 2.4: Handler — `indexSwapContract` (RetrieveAssets) + dispatch case

**Files:**
- Modify: `internal/indexer/embedded.go`

> The send-back of ZNN and QSR happens as two descendant blocks from the Swap contract. For simplicity and determinism, record the retrieval keyed by the user's send (RetrieveAssets) block hash and capture amounts from the contract's descendant sends if present on the block; otherwise leave amounts 0 and rely on the `swap_assets` snapshot delta. Record `public_key` from the decoded input.

- [ ] **Step 1: Add dispatch case in `indexEmbeddedContracts`**

```go
	case models.SwapAddress:
		i.indexSwapContract(ctx, batch, block, txData, m)
```

- [ ] **Step 2: Add the handler to `internal/indexer/embedded.go`**

```go
// indexSwapContract handles legacy genesis-swap RetrieveAssets claims.
// Amounts disbursed are the contract's descendant ZNN/QSR sends back to the
// claimant; when those are attached to the block we sum them by token, else 0.
func (i *Indexer) indexSwapContract(ctx context.Context, batch *pgx.Batch, block *api.AccountBlock, txData *models.TxData, m *api.Momentum) {
	if txData.Method != "RetrieveAssets" || block.PairedAccountBlock == nil {
		return
	}
	paired := block.PairedAccountBlock

	var znn, qsr int64
	for _, d := range block.DescendantBlocks {
		amt := safeBigIntToInt64(d.Amount, i.logger, "swap retrieval amount overflow",
			zap.String("swapID", paired.Hash.String()))
		switch d.TokenStandard.String() {
		case models.ZnnTokenStandard:
			znn += amt
		case models.QsrTokenStandard:
			qsr += amt
		}
	}

	i.repos.Swap.InsertRetrievalBatch(batch, &models.SwapRetrieval{
		ID:                paired.Hash.String(),
		Address:           paired.Address.String(),
		PublicKey:         txData.Inputs["publicKey"],
		ZnnAmount:         znn,
		QsrAmount:         qsr,
		MomentumHeight:    int64(m.Height),
		MomentumTimestamp: int64(m.TimestampUnix),
	})
}
```

> If `api.AccountBlock` does not expose `DescendantBlocks`, drop the descendant loop and set `znn`/`qsr` to 0 (the snapshot in Task 2.5 carries authoritative remaining balances). Confirm the field name with `grep -rn "DescendantBlocks\|Descendant" $(go env GOMODCACHE)/github.com/0x3639/znn-sdk-go@*/`.

- [ ] **Step 3: Verify build**

Run: `GOWORK=off go build ./...`
Expected: success (resolve the `DescendantBlocks` field name if the build fails on it).

- [ ] **Step 4: Commit**

```bash
git add internal/indexer/embedded.go
git commit -m "feat(swap): index RetrieveAssets claims from account blocks"
```

### Task 2.5: RPC snapshot — `swap_assets` via `SwapApi.GetAssets`

**Files:**
- Modify: `internal/indexer/indexer.go` (add `syncSwapAssets`, call it from the bridge/cached sync at a low cadence)

- [ ] **Step 1: Add `syncSwapAssets` to `internal/indexer/indexer.go`**

```go
// syncSwapAssets snapshots remaining unswapped genesis balances. Legacy data is
// near-static, so this runs alongside the cached-data refresh rather than per
// momentum. Entries that reach zero balance are upserted as zero (kept for
// history), matching the contract view.
func (i *Indexer) syncSwapAssets(ctx context.Context) {
	assets, err := i.client().SwapApi.GetAssets()
	if err != nil {
		i.logger.Warn("swap sync: GetAssets failed", zap.Error(err))
		return
	}
	now := time.Now().Unix()
	for keyIDHash, entry := range assets {
		znn := safeBigIntToInt64(entry.Znn, i.logger, "swap asset znn overflow")
		qsr := safeBigIntToInt64(entry.Qsr, i.logger, "swap asset qsr overflow")
		if err := i.repos.Swap.UpsertAsset(ctx, &models.SwapAsset{
			KeyIDHash:            keyIDHash.String(),
			Znn:                  znn,
			Qsr:                  qsr,
			LastUpdatedTimestamp: now,
		}); err != nil {
			i.logger.Warn("swap sync: upsert asset failed", zap.Error(err))
		}
	}
	i.logger.Info("swap sync: assets snapshot complete", zap.Int("count", len(assets)))
}
```

> Confirm the `GetAssets` return shape: `grep -rn "func.*GetAssets" $(go env GOMODCACHE)/github.com/0x3639/znn-sdk-go@*/api/embedded/swap.go`. The research notes it returns `map[types.Hash]*SwapAssetEntrySimple` with `.Znn`/`.Qsr`. Adjust field access if names differ.

- [ ] **Step 2: Call it from the cached-data refresh**

Find where `updateCachedData` (pillars/sentinels/projects, 5-min cadence) is invoked (`grep -n "updateCachedData" internal/indexer/indexer.go`). Add a call to `i.syncSwapAssets(ctx)` at the end of that function so the snapshot refreshes on the same low-frequency timer.

- [ ] **Step 3: Verify build**

Run: `GOWORK=off go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/indexer/indexer.go
git commit -m "feat(swap): snapshot remaining genesis balances via GetAssets"
```

### Task 2.6: Schema docs + regen

**Files:**
- Create: `docs/schema/swap_retrievals.md`, `docs/schema/swap_assets.md`
- Modify: generated indexes

- [ ] **Step 1: Write both schema pages** (mirror an existing page's structure; note the legacy/near-static nature and the two write paths: per-block handler vs. `GetAssets` snapshot).

- [ ] **Step 2: Regenerate + build**

Run:
```bash
python scripts/docs/gen-llms-txt.py && python scripts/docs/gen-llms-full.py && mkdocs build --strict
```
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add docs/ llms.txt llms-full.txt
git commit -m "docs(swap): document swap tables"
```

---

## Phase 3 — Bridge time-challenge tracking

**What:** Surface pending time-locked bridge security operations (the second-call delay window for `SetTokenPair`, `ChangeTssECDSAPubKey`, `ChangeAdministrator`, `NominateGuardians`). Useful for transparency: "a pending administrator change unlocks at height X."

**Approach:** RPC poll `BridgeApi.GetTimeChallengesInfo()` inside the existing bridge sync loop. Upsert current challenges; delete rows no longer present (a challenge vanishes once executed or expired). Single table `bridge_time_challenges`.

### Task 3.1: Migration — `bridge_time_challenges`

**Files:**
- Create: `migrations/016_bridge_time_challenges.up.sql`
- Create: `migrations/016_bridge_time_challenges.down.sql`

- [ ] **Step 1: Write `migrations/016_bridge_time_challenges.up.sql`**

```sql
-- migrations/016_bridge_time_challenges.up.sql
-- Pending bridge time challenges (the delay window before a security-sensitive
-- bridge method takes effect). Polled from bridge.getTimeChallengesInfo; rows
-- are removed once the challenge is executed or expires.
CREATE TABLE IF NOT EXISTS bridge_time_challenges (
    method_name             TEXT PRIMARY KEY,
    params_hash             TEXT   NOT NULL DEFAULT '',
    challenge_start_height  BIGINT NOT NULL DEFAULT 0,
    delay                   BIGINT NOT NULL DEFAULT 0,  -- soft or administrator delay (momentums)
    end_height              BIGINT NOT NULL DEFAULT 0,  -- start + delay
    last_updated_timestamp  BIGINT NOT NULL DEFAULT 0
);
```

- [ ] **Step 2: Write `migrations/016_bridge_time_challenges.down.sql`**

```sql
-- migrations/016_bridge_time_challenges.down.sql
DROP TABLE IF EXISTS bridge_time_challenges;
```

- [ ] **Step 3: Verify migration applies**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_Momentum -v
```
Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add migrations/016_bridge_time_challenges.up.sql migrations/016_bridge_time_challenges.down.sql
git commit -m "feat(bridge): add bridge_time_challenges table"
```

### Task 3.2: Model — `BridgeTimeChallenge`

**Files:**
- Modify: `internal/models/models.go` (append)

- [ ] **Step 1: Append model**

```go
// BridgeTimeChallenge is a pending time-locked bridge security operation.
type BridgeTimeChallenge struct {
	MethodName           string `db:"method_name"`
	ParamsHash           string `db:"params_hash"`
	ChallengeStartHeight int64  `db:"challenge_start_height"`
	Delay                int64  `db:"delay"`
	EndHeight            int64  `db:"end_height"`
	LastUpdatedTimestamp int64  `db:"last_updated_timestamp"`
}
```

- [ ] **Step 2: Build + commit**

Run: `GOWORK=off go build ./internal/models/...`
```bash
git add internal/models/models.go
git commit -m "feat(bridge): add BridgeTimeChallenge model"
```

### Task 3.3: Repository methods — on `BridgeConfigRepository`

**Files:**
- Modify: `internal/repository/bridge_config.go` (add `UpsertTimeChallenge`, `DeleteTimeChallengesNotIn`, `ListTimeChallenges`)
- Test: `internal/repository/bridge_time_challenge_integration_test.go`

- [ ] **Step 1: Write the failing integration test**

```go
//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

func TestIntegration_BridgeTimeChallenge_UpsertAndPrune(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewBridgeConfigRepository(pool)

	mustUpsert := func(name string) {
		if err := repo.UpsertTimeChallenge(ctx, &models.BridgeTimeChallenge{
			MethodName: name, ParamsHash: "h", ChallengeStartHeight: 100, Delay: 10, EndHeight: 110,
			LastUpdatedTimestamp: 1,
		}); err != nil {
			t.Fatalf("upsert %s: %v", name, err)
		}
	}
	mustUpsert("ChangeAdministrator")
	mustUpsert("NominateGuardians")

	// Prune everything except ChangeAdministrator (simulating NominateGuardians executed).
	if err := repo.DeleteTimeChallengesNotIn(ctx, []string{"ChangeAdministrator"}); err != nil {
		t.Fatalf("prune: %v", err)
	}

	list, err := repo.ListTimeChallenges(ctx)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].MethodName != "ChangeAdministrator" {
		t.Errorf("prune wrong: %+v", list)
	}

	// Empty keep-set prunes all.
	if err := repo.DeleteTimeChallengesNotIn(ctx, nil); err != nil {
		t.Fatalf("prune all: %v", err)
	}
	list, _ = repo.ListTimeChallenges(ctx)
	if len(list) != 0 {
		t.Errorf("expected empty after prune-all, got %+v", list)
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_BridgeTimeChallenge -v
```
Expected: FAIL — `undefined: ... UpsertTimeChallenge`.

- [ ] **Step 3: Add methods to `internal/repository/bridge_config.go`**

```go
// UpsertTimeChallenge inserts or refreshes a pending bridge time challenge.
func (r *BridgeConfigRepository) UpsertTimeChallenge(ctx context.Context, t *models.BridgeTimeChallenge) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO bridge_time_challenges
			(method_name, params_hash, challenge_start_height, delay, end_height, last_updated_timestamp)
		VALUES ($1,$2,$3,$4,$5,$6)
		ON CONFLICT (method_name) DO UPDATE SET
			params_hash = EXCLUDED.params_hash,
			challenge_start_height = EXCLUDED.challenge_start_height,
			delay = EXCLUDED.delay,
			end_height = EXCLUDED.end_height,
			last_updated_timestamp = EXCLUDED.last_updated_timestamp`,
		t.MethodName, t.ParamsHash, t.ChallengeStartHeight, t.Delay, t.EndHeight, t.LastUpdatedTimestamp)
	return err
}

// DeleteTimeChallengesNotIn removes challenge rows whose method_name is not in
// keep. An empty keep-set deletes all rows.
func (r *BridgeConfigRepository) DeleteTimeChallengesNotIn(ctx context.Context, keep []string) error {
	if len(keep) == 0 {
		_, err := r.pool.Exec(ctx, `DELETE FROM bridge_time_challenges`)
		return err
	}
	_, err := r.pool.Exec(ctx,
		`DELETE FROM bridge_time_challenges WHERE method_name <> ALL($1)`, keep)
	return err
}

// ListTimeChallenges returns all pending challenges ordered by end_height.
func (r *BridgeConfigRepository) ListTimeChallenges(ctx context.Context) ([]*models.BridgeTimeChallenge, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT method_name, params_hash, challenge_start_height, delay, end_height, last_updated_timestamp
		FROM bridge_time_challenges ORDER BY end_height`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []*models.BridgeTimeChallenge
	for rows.Next() {
		var t models.BridgeTimeChallenge
		if err := rows.Scan(&t.MethodName, &t.ParamsHash, &t.ChallengeStartHeight, &t.Delay,
			&t.EndHeight, &t.LastUpdatedTimestamp); err != nil {
			return nil, err
		}
		out = append(out, &t)
	}
	return out, rows.Err()
}
```

> Confirm the repo struct is named `BridgeConfigRepository` and exposes `pool`: `grep -n "BridgeConfigRepository\|func NewBridgeConfigRepository" internal/repository/bridge_config.go`. If the field is unexported under a different name, match it.

- [ ] **Step 4: Run to verify it passes; add `bridge_time_challenges` to the truncate list.**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/... -run TestIntegration_BridgeTimeChallenge -v
```
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/repository/bridge_config.go internal/repository/bridge_time_challenge_integration_test.go internal/repository/integration_test.go
git commit -m "feat(bridge): time-challenge upsert/prune/list repo methods"
```

### Task 3.4: Sync hook — poll `GetTimeChallengesInfo` in `updateBridgeConfig`

**Files:**
- Modify: `internal/indexer/indexer.go` (`updateBridgeConfig`)

- [ ] **Step 1: Add the poll block at the end of `updateBridgeConfig` (before `return nil`)**

```go
	// Time challenges: pending delay windows for security-sensitive bridge
	// methods. The set is authoritative — challenges vanish when executed or
	// expired — so prune rows not in the latest list.
	if tcl, err := i.client().BridgeApi.GetTimeChallengesInfo(); err != nil {
		i.logger.Warn("bridge config: GetTimeChallengesInfo failed", zap.Error(err))
	} else if tcl != nil {
		// SoftDelay governs SetTokenPair / ChangeTssECDSAPubKey; AdministratorDelay
		// governs ChangeAdministrator / NominateGuardians. Read delays from the
		// security info already fetched above; default 0 if unavailable.
		softDelay, adminDelay := i.bridgeDelays(ctx)
		keep := make([]string, 0, len(tcl.List))
		for _, tc := range tcl.List {
			delay := int64(softDelay)
			switch tc.MethodName {
			case "ChangeAdministrator", "NominateGuardians":
				delay = int64(adminDelay)
			}
			if err := i.repos.BridgeConfig.UpsertTimeChallenge(ctx, &models.BridgeTimeChallenge{
				MethodName:           tc.MethodName,
				ParamsHash:           tc.ParamsHash.String(),
				ChallengeStartHeight: int64(tc.ChallengeStartHeight),
				Delay:                delay,
				EndHeight:            int64(tc.ChallengeStartHeight) + delay,
				LastUpdatedTimestamp: now,
			}); err != nil {
				i.logger.Warn("bridge config: upsert time challenge failed", zap.Error(err))
				continue
			}
			keep = append(keep, tc.MethodName)
		}
		if err := i.repos.BridgeConfig.DeleteTimeChallengesNotIn(ctx, keep); err != nil {
			i.logger.Warn("bridge config: prune time challenges failed", zap.Error(err))
		}
	}
```

- [ ] **Step 2: Add the `bridgeDelays` helper**

```go
// bridgeDelays returns (softDelay, administratorDelay) in momentums from the
// bridge security info, or (0, 0) if unavailable.
func (i *Indexer) bridgeDelays(ctx context.Context) (soft, admin int) {
	sec, err := i.client().BridgeApi.GetSecurityInfo()
	if err != nil || sec == nil {
		return 0, 0
	}
	return int(sec.SoftDelay), int(sec.AdministratorDelay)
}
```

> `updateBridgeConfig` already calls `GetSecurityInfo` once; if you prefer one call, thread the delays from that existing block into the time-challenge loop instead of adding `bridgeDelays`. Either is correct; the helper keeps the diff localized.

- [ ] **Step 3: Verify build**

Run: `GOWORK=off go build ./...`
Expected: success.

- [ ] **Step 4: Commit**

```bash
git add internal/indexer/indexer.go
git commit -m "feat(bridge): poll and persist pending time challenges"
```

### Task 3.5: Schema doc + regen

**Files:**
- Create: `docs/schema/bridge_time_challenges.md`
- Modify: generated indexes

- [ ] **Step 1: Write the schema page** (note: RPC-polled, prune-on-absence semantics, the four challenged methods, which delay applies to each).

- [ ] **Step 2: Regenerate + build**

Run: `python scripts/docs/gen-llms-txt.py && python scripts/docs/gen-llms-full.py && mkdocs build --strict`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add docs/ llms.txt llms-full.txt
git commit -m "docs(bridge): document bridge_time_challenges"
```

---

## Phase 4 — Webhooks / event push

**What:** After each momentum's transaction commits, fire fire-and-forget HTTP webhooks for `momentum.inserted` and `account_block.inserted` events, with optional HMAC-SHA256 signing and bounded retries. Lets downstream consumers react in real time instead of polling.

**Approach:** A new `internal/webhooks` package with a `Dispatcher` that holds configured endpoints and a worker that POSTs JSON payloads. Config via viper (`webhooks:` YAML block + env vars). The indexer constructs the dispatcher and calls it after `processMomentum` commits successfully. No DB table (matches the TS core, which keeps webhooks stateless).

### Task 4.1: Config — `WebhookConfig`

**Files:**
- Modify: `internal/config/config.go`
- Test: `internal/config/config_test.go` (add a case)

- [ ] **Step 1: Add config structs to `internal/config/config.go`**

Add to the `Config` struct:
```go
	Webhooks WebhooksConfig `mapstructure:"webhooks"`
```
Add the new types:
```go
// WebhooksConfig configures outbound event push.
type WebhooksConfig struct {
	Enabled   bool              `mapstructure:"enabled"`
	Endpoints []WebhookEndpoint `mapstructure:"endpoints"`
	// TimeoutSeconds is the per-request HTTP timeout (default 5).
	TimeoutSeconds int `mapstructure:"timeout_seconds"`
	// MaxRetries is the number of resend attempts on failure (default 3).
	MaxRetries int `mapstructure:"max_retries"`
}

// WebhookEndpoint is one subscriber.
type WebhookEndpoint struct {
	URL    string `mapstructure:"url"`
	Secret string `mapstructure:"secret"`
	// Events is the set this endpoint receives; empty means all.
	Events []string `mapstructure:"events"`
}
```

- [ ] **Step 2: Add defaults in `load()`**

```go
	v.SetDefault("webhooks.enabled", false)
	v.SetDefault("webhooks.timeout_seconds", 5)
	v.SetDefault("webhooks.max_retries", 3)
```
And bind the enable flag:
```go
	_ = v.BindEnv("webhooks.enabled", "WEBHOOKS_ENABLED")
```

- [ ] **Step 3: Add a config test case to `internal/config/config_test.go`**

```go
func TestLoad_WebhooksDefaults(t *testing.T) {
	cfg, err := loadFromDir(t.TempDir()) // use the existing hermetic helper; if named differently, match it
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.Webhooks.Enabled {
		t.Error("webhooks should default disabled")
	}
	if cfg.Webhooks.TimeoutSeconds != 5 || cfg.Webhooks.MaxRetries != 3 {
		t.Errorf("unexpected webhook defaults: %+v", cfg.Webhooks)
	}
}
```

> The repo's config tests are hermetic (memory note `env-backcompat tests hermetic`). Find the existing helper used to load from an isolated dir (`grep -n "func load(\|loadFrom\|t.TempDir" internal/config/config_test.go`) and use it rather than `Load()` so ambient `config.yaml` can't leak in.

- [ ] **Step 4: Run the test + build**

Run: `GOWORK=off go test ./internal/config/ -run TestLoad_Webhooks -v && GOWORK=off go build ./...`
Expected: PASS, then success.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(webhooks): add webhook config"
```

### Task 4.2: Dispatcher package — payloads + signing

**Files:**
- Create: `internal/webhooks/dispatcher.go`
- Test: `internal/webhooks/dispatcher_test.go`

- [ ] **Step 1: Write the failing unit test `internal/webhooks/dispatcher_test.go`**

```go
package webhooks

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

func TestDispatcher_DeliversAndSigns(t *testing.T) {
	var (
		mu      sync.Mutex
		bodies  [][]byte
		sigs    []string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, r.ContentLength)
		_, _ = r.Body.Read(buf)
		mu.Lock()
		bodies = append(bodies, buf)
		sigs = append(sigs, r.Header.Get("X-Webhook-Signature"))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := New([]Endpoint{{URL: srv.URL, Secret: "s3cr3t"}}, 2*time.Second, 1, nil)
	d.Start()
	defer d.Stop()

	d.Emit(Event{Type: "momentum.inserted", Payload: map[string]any{"height": "42"}})

	// Wait for delivery.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		n := len(bodies)
		mu.Unlock()
		if n > 0 {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(bodies) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(bodies))
	}
	var got Event
	if err := json.Unmarshal(bodies[0], &got); err != nil {
		t.Fatalf("payload not JSON: %v", err)
	}
	if got.Type != "momentum.inserted" {
		t.Errorf("type = %q", got.Type)
	}
	if want := ComputeSignature("s3cr3t", bodies[0]); sigs[0] != want {
		t.Errorf("signature = %q, want %q", sigs[0], want)
	}
}

func TestDispatcher_EventFilter(t *testing.T) {
	d := New([]Endpoint{{URL: "http://x", Events: []string{"account_block.inserted"}}}, time.Second, 1, nil)
	if d.wants(d.endpoints[0], "momentum.inserted") {
		t.Error("should not want unsubscribed event")
	}
	if !d.wants(d.endpoints[0], "account_block.inserted") {
		t.Error("should want subscribed event")
	}
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `GOWORK=off go test ./internal/webhooks/ -v`
Expected: FAIL — package/symbols undefined.

- [ ] **Step 3: Write `internal/webhooks/dispatcher.go`**

```go
// Package webhooks delivers fire-and-forget event notifications to configured
// HTTP endpoints. Delivery is best-effort: failures are retried a bounded
// number of times and then dropped (with a log line). It never blocks the
// indexer's sync loop.
package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

// Event is one notification.
type Event struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

// Endpoint is a subscriber.
type Endpoint struct {
	URL    string
	Secret string
	Events []string // empty = all
}

// Dispatcher fans events out to endpoints via a buffered queue + worker.
type Dispatcher struct {
	endpoints  []Endpoint
	timeout    time.Duration
	maxRetries int
	logger     *zap.Logger
	queue      chan Event
	client     *http.Client
	done       chan struct{}
}

// New builds a Dispatcher. logger may be nil (a no-op logger is used).
func New(endpoints []Endpoint, timeout time.Duration, maxRetries int, logger *zap.Logger) *Dispatcher {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Dispatcher{
		endpoints:  endpoints,
		timeout:    timeout,
		maxRetries: maxRetries,
		logger:     logger,
		queue:      make(chan Event, 1024),
		client:     &http.Client{Timeout: timeout},
		done:       make(chan struct{}),
	}
}

// Start launches the delivery worker.
func (d *Dispatcher) Start() {
	go d.run()
}

// Stop drains and stops the worker.
func (d *Dispatcher) Stop() {
	close(d.queue)
	<-d.done
}

// Emit enqueues an event. Non-blocking: if the queue is full the event is
// dropped with a warning (back-pressure must never stall the indexer).
func (d *Dispatcher) Emit(e Event) {
	select {
	case d.queue <- e:
	default:
		d.logger.Warn("webhook queue full; dropping event", zap.String("type", e.Type))
	}
}

func (d *Dispatcher) run() {
	defer close(d.done)
	for e := range d.queue {
		body, err := json.Marshal(e)
		if err != nil {
			d.logger.Warn("webhook marshal failed", zap.Error(err))
			continue
		}
		for _, ep := range d.endpoints {
			if !d.wants(ep, e.Type) {
				continue
			}
			d.deliver(ep, e.Type, body)
		}
	}
}

func (d *Dispatcher) wants(ep Endpoint, eventType string) bool {
	if len(ep.Events) == 0 {
		return true
	}
	for _, ev := range ep.Events {
		if ev == eventType {
			return true
		}
	}
	return false
}

func (d *Dispatcher) deliver(ep Endpoint, eventType string, body []byte) {
	for attempt := 0; attempt <= d.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 200 * time.Millisecond)
		}
		ctx, cancel := context.WithTimeout(context.Background(), d.timeout)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, ep.URL, bytes.NewReader(body))
		if err != nil {
			cancel()
			d.logger.Warn("webhook request build failed", zap.Error(err))
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if ep.Secret != "" {
			req.Header.Set("X-Webhook-Signature", ComputeSignature(ep.Secret, body))
		}
		resp, err := d.client.Do(req)
		cancel()
		if err == nil && resp.StatusCode >= 200 && resp.StatusCode < 300 {
			_ = resp.Body.Close()
			return
		}
		if resp != nil {
			_ = resp.Body.Close()
		}
	}
	d.logger.Warn("webhook delivery failed after retries",
		zap.String("url", ep.URL), zap.String("type", eventType))
}

// ComputeSignature returns the hex HMAC-SHA256 of body keyed by secret.
func ComputeSignature(secret string, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}
```

- [ ] **Step 4: Run to verify it passes**

Run: `GOWORK=off go test ./internal/webhooks/ -v`
Expected: PASS (both tests).

- [ ] **Step 5: Commit**

```bash
git add internal/webhooks/dispatcher.go internal/webhooks/dispatcher_test.go
git commit -m "feat(webhooks): event dispatcher with HMAC signing and retries"
```

### Task 4.3: Wire the dispatcher into the indexer

**Files:**
- Modify: `internal/indexer/indexer.go` (construct dispatcher; emit after commit)
- Modify: `internal/indexer/processor.go` (collect block events; emit after the momentum tx commits)

- [ ] **Step 1: Add a dispatcher field to the `Indexer` struct and construct it**

Find the `Indexer` struct and its constructor (`grep -n "type Indexer struct\|func New.*Indexer" internal/indexer/indexer.go`). Add:
```go
	webhooks *webhooks.Dispatcher
```
In the constructor, after config is available, build and start it when enabled:
```go
	if cfg.Webhooks.Enabled {
		eps := make([]webhooks.Endpoint, 0, len(cfg.Webhooks.Endpoints))
		for _, e := range cfg.Webhooks.Endpoints {
			eps = append(eps, webhooks.Endpoint{URL: e.URL, Secret: e.Secret, Events: e.Events})
		}
		d := webhooks.New(eps, time.Duration(cfg.Webhooks.TimeoutSeconds)*time.Second,
			cfg.Webhooks.MaxRetries, logger)
		d.Start()
		idx.webhooks = d
	}
```
Add `"github.com/0x3639/nom-indexer-go/internal/webhooks"` to imports. Ensure `d.Stop()` is called on indexer shutdown (next to other cleanup in the `Run` teardown / `ctx.Done()` path), guarded by `if i.webhooks != nil`.

- [ ] **Step 2: Emit events after a momentum commits**

`processMomentum` opens a tx/batch, runs it, and commits (per `CLAUDE.md` rule 4). After the **successful commit** (not inside the batch — webhooks must not be part of the DB transaction), emit:
```go
	if i.webhooks != nil {
		i.webhooks.Emit(webhooks.Event{
			Type: "momentum.inserted",
			Payload: map[string]any{
				"height":    m.Height,
				"hash":      m.Hash.String(),
				"timestamp": m.TimestampUnix,
			},
		})
		for _, ev := range blockEvents {
			i.webhooks.Emit(ev)
		}
	}
```
Build `blockEvents []webhooks.Event` while iterating blocks in `processAccountBlocks` (return it up to `processMomentum`, or stash on a per-momentum struct). Each entry:
```go
		webhooks.Event{
			Type: "account_block.inserted",
			Payload: map[string]any{
				"momentumHeight": m.Height,
				"hash":           block.Hash.String(),
				"address":        block.Address.String(),
				"toAddress":      block.ToAddress.String(),
				"blockType":      int(block.BlockType),
			},
		}
```

> Threading: the cleanest minimal change is for `processAccountBlocks` to return `[]webhooks.Event` and `processMomentum` to emit them only after `tx.Commit` succeeds. Do not emit on the rollback/error path — a retried momentum would double-fire.

- [ ] **Step 3: Verify build + existing tests still pass**

Run: `GOWORK=off go build ./... && GOWORK=off go test ./internal/indexer/...`
Expected: build success; indexer unit tests pass.

- [ ] **Step 4: Commit**

```bash
git add internal/indexer/indexer.go internal/indexer/processor.go
git commit -m "feat(webhooks): emit momentum and account_block events after commit"
```

### Task 4.4: Docs + example config

**Files:**
- Create: `docs/operations/webhooks.md`
- Modify: `config.yaml` example block (commented), `docs/config/reference.md`

- [ ] **Step 1: Document the webhook config keys, payload shapes, signature scheme, and at-least-once/best-effort semantics** in `docs/operations/webhooks.md` and add the keys to `docs/config/reference.md`. Add a commented `webhooks:` example to `config.yaml`.

- [ ] **Step 2: Regenerate + build**

Run: `python scripts/docs/gen-llms-txt.py && python scripts/docs/gen-llms-full.py && mkdocs build --strict`
Expected: clean.

- [ ] **Step 3: Commit**

```bash
git add docs/ config.yaml llms.txt llms-full.txt
git commit -m "docs(webhooks): document webhook config and payloads"
```

---

## Final verification (run after all selected phases)

- [ ] **Full build + unit tests**

Run: `GOWORK=off go build ./... && GOWORK=off go test ./...`
Expected: all pass.

- [ ] **Integration tests against a real Postgres**

Run:
```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./...
```
Expected: all pass (HTLC, Swap, time-challenge repos exercised).

- [ ] **Lint / CI parity**

Run: `dagger call ci --source .`
Expected: lint + test + build green, and the docs-sync check passes (llms.txt regenerated).

- [ ] **End-to-end smoke against live data** (docker stack running and caught up)

- HTLC: `SELECT count(*), status FROM htlcs GROUP BY status;` — expect rows once HTLC activity is replayed (note: only blocks processed *after* deploy populate it unless you backfill).
- Swap: `SELECT count(*) FROM swap_assets;` and `SELECT count(*) FROM swap_retrievals;`.
- Time challenges: `SELECT * FROM bridge_time_challenges;` — likely empty unless a bridge admin op is mid-window; confirm the poll ran via the `bridge config:` log lines.
- Webhooks: point an endpoint at a local catch server (`webhooks.enabled: true`) and confirm `momentum.inserted` arrives each momentum.

> **Backfill note:** HTLC and Swap retrieval events are indexed going forward from deploy. If historical coverage is required, a re-sync from genesis (or a targeted backfill of the HTLC/Swap address blocks) is needed — call this out to the user; it is out of scope for these phases.

---

## Self-review checklist (completed during authoring)

- **Spec coverage:** All four selected subsystems have phases (HTLC §1, Swap §2, time challenges §3, webhooks §4). ✓
- **Type consistency:** `Htlc`/`HtlcStatus`, `SwapRetrieval`/`SwapAsset`, `BridgeTimeChallenge`, `webhooks.Event`/`Endpoint`/`Dispatcher` are defined once and referenced consistently across tasks. Repository method names (`InsertBatch`, `SettleBatch`, `UpsertTimeChallenge`, `DeleteTimeChallengesNotIn`) match between definition and call sites. ✓
- **Unknowns flagged, not hidden:** Three points require live confirmation and are called out inline with the grep/SQL to resolve them — (1) HTLC id linkage (Task 1.6), (2) `DescendantBlocks` field name on the SDK `AccountBlock` (Task 2.4), (3) `GetAssets` return shape (Task 2.5). These are SDK-surface details that must be verified against the pinned module version, not invented.
- **Idempotency/transactionality:** Per-block writes enqueue on the shared batch (HTLC, Swap retrievals); RPC snapshots upsert outside the momentum tx (Swap assets, time challenges); webhooks emit only after commit. Matches `CLAUDE.md` rule 4. ✓
