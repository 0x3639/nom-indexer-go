# Continuous Sync Watchdog Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a background watchdog goroutine to `cmd/indexer` that detects indexer-vs-znnd drift and znnd-vs-chain drift, recovers in-process via the existing subscription restart path, and fails over to configured alternate nodes — all without restarting the container.

**Architecture:** One new ticker goroutine alongside the existing bridge/cached-data/cron loops. The SDK client field becomes `atomic.Pointer[rpc_client.RpcClient]` so any goroutine sees the current node. A new single-row `indexer_sync_status` table publishes state to the API/MCP. A tiny indexer-side HTTP server exposes `/healthz` and `/readyz`. The API's existing `/readyz` extends to consume the new table.

**Tech Stack:** Go 1.x with `pgxpool`, `zap`, `go.uber.org/atomic`-style `atomic.Pointer` (stdlib `sync/atomic`), `chi` router, `znn-sdk-go` `rpc_client`. Tests use the existing `withRetry`, `httptest`, and `-tags integration` Postgres harness.

**Spec:** [`docs/superpowers/specs/2026-05-28-continuous-sync-watchdog-design.md`](../specs/2026-05-28-continuous-sync-watchdog-design.md). Read it before starting.

**Branch:** `feat/continuous-sync-watchdog` (already exists; spec is committed).

**Build/test reminders:**
- All `go build` / `go test` invocations require `GOWORK=off` per CLAUDE.md.
- Migrations live under `migrations/NNN_*.up.sql` and `NNN_*.down.sql`. Current head is `012`. New migration is `013`.
- Integration tests use the `integration` build tag and need `TEST_DATABASE_URL`.
- The user's feedback memory says **never push to GitHub** until explicitly asked. Commit locally; surface a PR-ready summary at the end.

---

## File structure

| Path | Action | Purpose |
|---|---|---|
| `migrations/013_indexer_sync_status.up.sql` | create | Single-row `indexer_sync_status` table |
| `migrations/013_indexer_sync_status.down.sql` | create | Drop |
| `internal/models/models.go` | edit | Add `SyncStatus` struct |
| `internal/repository/sync_status.go` | create | `Upsert`, `Get` |
| `internal/repository/sync_status_test.go` | create | Unit (integration-tag) tests |
| `internal/repository/repositories.go` | edit | Register `SyncStatus` repo on `Repositories` aggregate |
| `internal/config/config.go` | edit | `NodesConfig`, `WatchdogConfig`, `HealthConfig`; env back-compat |
| `internal/config/config_test.go` | edit | New parse cases |
| `internal/indexer/syncinfo.go` | create | Raw JSON-RPC caller for `stats.syncInfo` |
| `internal/indexer/syncinfo_test.go` | create | `httptest`-backed tests |
| `internal/indexer/nodepool.go` | create | Ordered node list + `Probe()` |
| `internal/indexer/nodepool_test.go` | create | Probe tests against `httptest` |
| `internal/indexer/client_switch.go` | create | `swapClient` with genesis-hash verification |
| `internal/indexer/client_switch_test.go` | create | Switch refusal + atomic publish tests |
| `internal/indexer/watchdog.go` | create | `classify`, `react`, `runSyncWatchdogLoop` |
| `internal/indexer/watchdog_test.go` | create | Classify table tests + streak/reaction tests |
| `internal/indexer/indexer.go` | edit | `client` → `activeClient atomic.Pointer`; `lastProgressAt`; mutex-protected `syncState`; wire watchdog in `Run()` |
| `internal/indexer/processor.go` | edit | Update `lastProgressAt` after each successful momentum commit |
| `internal/health/server.go` | create | `/healthz`, `/readyz` HTTP server |
| `internal/health/server_test.go` | create | HTTP route tests |
| `cmd/indexer/main.go` | edit | Build NodePool from config; launch health server |
| `internal/api/router/router.go` | edit | Extend `readyz`; bump `minSchemaVersion` 12→13 |
| `internal/api/router/router_test.go` | edit | New readyz cases (degraded, first-run) |
| `docker-compose.yml` | edit | Indexer healthcheck + new env vars + internal port |
| `config.yaml.example` | edit | Document new keys |
| `docs/architecture/overview.md` | edit | Add watchdog row to the lane table |

---

## Task 1: Migration 013 — `indexer_sync_status` table

**Files:**
- Create: `migrations/013_indexer_sync_status.up.sql`
- Create: `migrations/013_indexer_sync_status.down.sql`

- [ ] **Step 1: Write the up migration**

```sql
-- migrations/013_indexer_sync_status.up.sql
CREATE TABLE indexer_sync_status (
    id                     SMALLINT PRIMARY KEY CHECK (id = 1),
    db_height              BIGINT       NOT NULL,
    znnd_frontier_height   BIGINT       NOT NULL,
    znnd_target_height     BIGINT       NOT NULL,
    drift_momentums        BIGINT       NOT NULL,
    node_lag_momentums     BIGINT       NOT NULL,
    state                  TEXT         NOT NULL,
    consecutive_bad_checks INTEGER      NOT NULL DEFAULT 0,
    active_node_url        TEXT         NOT NULL,
    active_node_label      TEXT         NOT NULL,
    chain_identifier       TEXT         NOT NULL,
    failed_over_at         BIGINT,
    last_progress_at       BIGINT       NOT NULL,
    checked_at             BIGINT       NOT NULL
);
```

- [ ] **Step 2: Write the down migration**

```sql
-- migrations/013_indexer_sync_status.down.sql
DROP TABLE IF EXISTS indexer_sync_status;
```

- [ ] **Step 3: Verify migrations apply cleanly**

```bash
# Against a scratch DB
docker compose up -d postgres
GOWORK=off go run ./cmd/indexer --migrate-only 2>&1 | tail -5
# If --migrate-only flag doesn't exist, just start the indexer and verify migration 013 logs.
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer -c '\d indexer_sync_status'
```

Expected: table prints with all 14 columns and the `CHECK (id = 1)` constraint.

- [ ] **Step 4: Verify down migration**

```bash
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer -c 'DROP TABLE indexer_sync_status;'
```

Expected: no error.

- [ ] **Step 5: Commit**

```bash
git add migrations/013_*
git commit -m "feat(schema): add indexer_sync_status table for watchdog state"
```

---

## Task 2: `SyncStatus` model struct

**Files:**
- Modify: `internal/models/models.go` (append; existing file is the catch-all model file)

- [ ] **Step 1: Write the model struct test**

```go
// internal/models/models_test.go — append
func TestSyncStatusZeroValue(t *testing.T) {
    var s models.SyncStatus
    if s.State != "" {
        t.Fatalf("zero value State should be empty, got %q", s.State)
    }
    if s.DBHeight != 0 {
        t.Fatalf("zero value DBHeight should be 0, got %d", s.DBHeight)
    }
}
```

- [ ] **Step 2: Run the test and confirm it fails (undefined type)**

```bash
GOWORK=off go test ./internal/models/ -run TestSyncStatusZeroValue
```

Expected: `undefined: models.SyncStatus`.

- [ ] **Step 3: Add the struct**

```go
// internal/models/models.go — append, after the existing types

// SyncStatus is the in-DB projection of the watchdog's last tick.
// The row is single-row (id=1) — see migrations/013.
type SyncStatus struct {
    DBHeight             int64
    ZnndFrontierHeight   int64
    ZnndTargetHeight     int64
    DriftMomentums       int64
    NodeLagMomentums     int64
    State                string // synced | indexer_lagging | node_lagging | stalled | probe_failed
    ConsecutiveBadChecks int
    ActiveNodeURL        string
    ActiveNodeLabel      string
    ChainIdentifier      string
    FailedOverAt         *int64 // null on primary
    LastProgressAt       int64
    CheckedAt            int64
}
```

- [ ] **Step 4: Run test, confirm pass**

```bash
GOWORK=off go test ./internal/models/ -run TestSyncStatusZeroValue
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/models/models.go internal/models/models_test.go
git commit -m "feat(models): add SyncStatus struct"
```

---

## Task 3: `SyncStatusRepository`

**Files:**
- Create: `internal/repository/sync_status.go`
- Create: `internal/repository/sync_status_test.go`
- Modify: `internal/repository/repositories.go` (register the new repo on the aggregate)

- [ ] **Step 1: Write the failing repo test**

```go
//go:build integration
// internal/repository/sync_status_test.go
package repository_test

import (
    "context"
    "testing"

    "github.com/0x3639/nom-indexer-go/internal/models"
    "github.com/0x3639/nom-indexer-go/internal/repository"
)

func TestSyncStatusUpsertGet(t *testing.T) {
    ctx := context.Background()
    pool := newTestPool(t) // existing helper used elsewhere in repository tests
    repos := repository.NewRepositories(pool)

    want := &models.SyncStatus{
        DBHeight:             100,
        ZnndFrontierHeight:   100,
        ZnndTargetHeight:     100,
        State:                "synced",
        ActiveNodeURL:        "ws://znnd:35998",
        ActiveNodeLabel:      "local",
        ChainIdentifier:      "genesis-hash",
        LastProgressAt:       1000,
        CheckedAt:            1001,
    }
    if err := repos.SyncStatus.Upsert(ctx, want); err != nil {
        t.Fatalf("Upsert: %v", err)
    }
    got, err := repos.SyncStatus.Get(ctx)
    if err != nil {
        t.Fatalf("Get: %v", err)
    }
    if got.DBHeight != want.DBHeight || got.State != want.State {
        t.Fatalf("round-trip mismatch:\n got %#v\n want %#v", got, want)
    }
}

func TestSyncStatusSingletonConstraint(t *testing.T) {
    ctx := context.Background()
    pool := newTestPool(t)

    _, err := pool.Exec(ctx,
        `INSERT INTO indexer_sync_status (id, db_height, znnd_frontier_height, znnd_target_height,
         drift_momentums, node_lag_momentums, state, active_node_url, active_node_label,
         chain_identifier, last_progress_at, checked_at)
         VALUES (2, 0, 0, 0, 0, 0, 'synced', '', '', '', 0, 0)`)
    if err == nil {
        t.Fatal("expected CHECK (id = 1) to reject id=2 insert")
    }
}
```

- [ ] **Step 2: Run test, confirm undefined-type failure**

```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/ -run TestSyncStatus
```

Expected: build failure — `undefined: repository.Repositories.SyncStatus`.

- [ ] **Step 3: Implement the repo**

```go
// internal/repository/sync_status.go
package repository

import (
    "context"
    "fmt"

    "github.com/jackc/pgx/v5/pgxpool"

    "github.com/0x3639/nom-indexer-go/internal/models"
)

type SyncStatusRepository struct {
    pool *pgxpool.Pool
}

func NewSyncStatusRepository(pool *pgxpool.Pool) *SyncStatusRepository {
    return &SyncStatusRepository{pool: pool}
}

const syncStatusUpsertSQL = `
INSERT INTO indexer_sync_status (
    id, db_height, znnd_frontier_height, znnd_target_height,
    drift_momentums, node_lag_momentums, state, consecutive_bad_checks,
    active_node_url, active_node_label, chain_identifier,
    failed_over_at, last_progress_at, checked_at
) VALUES (
    1, $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13
) ON CONFLICT (id) DO UPDATE SET
    db_height              = EXCLUDED.db_height,
    znnd_frontier_height   = EXCLUDED.znnd_frontier_height,
    znnd_target_height     = EXCLUDED.znnd_target_height,
    drift_momentums        = EXCLUDED.drift_momentums,
    node_lag_momentums     = EXCLUDED.node_lag_momentums,
    state                  = EXCLUDED.state,
    consecutive_bad_checks = EXCLUDED.consecutive_bad_checks,
    active_node_url        = EXCLUDED.active_node_url,
    active_node_label      = EXCLUDED.active_node_label,
    chain_identifier       = EXCLUDED.chain_identifier,
    failed_over_at         = EXCLUDED.failed_over_at,
    last_progress_at       = EXCLUDED.last_progress_at,
    checked_at             = EXCLUDED.checked_at;
`

func (r *SyncStatusRepository) Upsert(ctx context.Context, s *models.SyncStatus) error {
    _, err := r.pool.Exec(ctx, syncStatusUpsertSQL,
        s.DBHeight, s.ZnndFrontierHeight, s.ZnndTargetHeight,
        s.DriftMomentums, s.NodeLagMomentums,
        s.State, s.ConsecutiveBadChecks,
        s.ActiveNodeURL, s.ActiveNodeLabel, s.ChainIdentifier,
        s.FailedOverAt, s.LastProgressAt, s.CheckedAt,
    )
    if err != nil {
        return fmt.Errorf("upsert sync_status: %w", err)
    }
    return nil
}

const syncStatusGetSQL = `
SELECT db_height, znnd_frontier_height, znnd_target_height,
       drift_momentums, node_lag_momentums, state, consecutive_bad_checks,
       active_node_url, active_node_label, chain_identifier,
       failed_over_at, last_progress_at, checked_at
  FROM indexer_sync_status WHERE id = 1;
`

func (r *SyncStatusRepository) Get(ctx context.Context) (*models.SyncStatus, error) {
    var s models.SyncStatus
    err := r.pool.QueryRow(ctx, syncStatusGetSQL).Scan(
        &s.DBHeight, &s.ZnndFrontierHeight, &s.ZnndTargetHeight,
        &s.DriftMomentums, &s.NodeLagMomentums,
        &s.State, &s.ConsecutiveBadChecks,
        &s.ActiveNodeURL, &s.ActiveNodeLabel, &s.ChainIdentifier,
        &s.FailedOverAt, &s.LastProgressAt, &s.CheckedAt,
    )
    if err != nil {
        return nil, fmt.Errorf("get sync_status: %w", err)
    }
    return &s, nil
}
```

- [ ] **Step 4: Register on `Repositories` aggregate**

Open `internal/repository/repositories.go` and add the field + constructor call. (Read the file first to see the existing pattern — typically a struct field and a line in `NewRepositories`.)

```go
// Add to Repositories struct:
SyncStatus *SyncStatusRepository

// Add to NewRepositories function body:
SyncStatus: NewSyncStatusRepository(pool),
```

- [ ] **Step 5: Run tests, confirm pass**

```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/repository/ -run TestSyncStatus
```

Expected: both tests PASS. If `newTestPool` test helper does not migrate the DB automatically, run migrations against the test DB first (see other `_test.go` files in this package for the pattern).

- [ ] **Step 6: Commit**

```bash
git add internal/repository/sync_status.go internal/repository/sync_status_test.go \
        internal/repository/repositories.go
git commit -m "feat(repo): add SyncStatusRepository with Upsert/Get"
```

---

## Task 4: Config — `NodesConfig`, `WatchdogConfig`, `HealthConfig`

**Files:**
- Modify: `internal/config/config.go`
- Modify: `internal/config/config_test.go`

- [ ] **Step 1: Write failing parse tests**

```go
// internal/config/config_test.go — append

func TestNodesConfigFromEnvBackcompat(t *testing.T) {
    t.Setenv("NODE_URL_WS", "ws://znnd:35998")
    t.Setenv("NODE_URL_FALLBACKS", "wss://my.hc1node.com:35998,https://my.hc1node.com:35997")
    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if got, want := len(cfg.Indexer.Nodes), 3; got != want {
        t.Fatalf("nodes len = %d, want %d", got, want)
    }
    if cfg.Indexer.Nodes[0].URL != "ws://znnd:35998" {
        t.Fatalf("primary URL: %q", cfg.Indexer.Nodes[0].URL)
    }
    if cfg.Indexer.Nodes[0].Label != "primary" {
        t.Fatalf("primary auto-label: %q", cfg.Indexer.Nodes[0].Label)
    }
}

func TestWatchdogConfigDefaults(t *testing.T) {
    t.Setenv("NODE_URL_WS", "ws://znnd:35998")
    cfg, err := config.Load()
    if err != nil {
        t.Fatalf("Load: %v", err)
    }
    if cfg.Indexer.Watchdog.Interval != 30*time.Second {
        t.Fatalf("default interval: %v", cfg.Indexer.Watchdog.Interval)
    }
    if cfg.Indexer.Watchdog.UnhealthyStreak != 2 {
        t.Fatalf("default unhealthy_streak: %d", cfg.Indexer.Watchdog.UnhealthyStreak)
    }
    if cfg.Indexer.Watchdog.FailbackStreak != 5 {
        t.Fatalf("default failback_streak: %d", cfg.Indexer.Watchdog.FailbackStreak)
    }
}
```

- [ ] **Step 2: Run, confirm fail (undefined fields)**

```bash
GOWORK=off go test ./internal/config/ -run 'TestNodesConfig|TestWatchdogConfig'
```

Expected: compile error.

- [ ] **Step 3: Add the types and wire them**

Open `internal/config/config.go`. Add new types and add an `Indexer` field to the root `Config` struct. Implement the env-var fallback in `Load()` (or wherever existing env vars are read).

```go
// internal/config/config.go — additions

type IndexerConfig struct {
    Nodes    []NodeEntry     `mapstructure:"nodes"`
    Watchdog WatchdogConfig  `mapstructure:"watchdog"`
    Health   HealthConfig    `mapstructure:"health"`
}

type NodeEntry struct {
    URL   string `mapstructure:"url"`
    Label string `mapstructure:"label"`
}

type WatchdogConfig struct {
    Enabled               bool          `mapstructure:"enabled"`
    Interval              time.Duration `mapstructure:"interval"`
    StallThreshold        time.Duration `mapstructure:"stall_threshold"`
    IndexerDriftThreshold int64         `mapstructure:"indexer_drift_threshold"`
    NodeDriftThreshold    int64         `mapstructure:"node_drift_threshold"`
    UnhealthyStreak       int           `mapstructure:"unhealthy_streak"`
    FailbackStreak        int           `mapstructure:"failback_streak"`
    TolerateMissingSyncInfo bool        `mapstructure:"tolerate_missing_syncinfo"`
}

type HealthConfig struct {
    Enabled bool `mapstructure:"enabled"`
    Port    int  `mapstructure:"port"`
}

// On the root Config struct, add:
//   Indexer IndexerConfig `mapstructure:"indexer"`

func defaultWatchdog() WatchdogConfig {
    return WatchdogConfig{
        Enabled:                 false, // opt-in per spec rollout plan
        Interval:                30 * time.Second,
        StallThreshold:          60 * time.Second,
        IndexerDriftThreshold:   3,
        NodeDriftThreshold:      3,
        UnhealthyStreak:         2,
        FailbackStreak:          5,
        TolerateMissingSyncInfo: true,
    }
}

func defaultHealth() HealthConfig {
    return HealthConfig{Enabled: true, Port: 9092}
}
```

In `Load()`, after Viper unmarshal, apply defaults and the env-var back-compat:

```go
// Apply Watchdog/Health defaults if zero-valued.
if cfg.Indexer.Watchdog.Interval == 0 {
    cfg.Indexer.Watchdog = defaultWatchdog()
}
if cfg.Indexer.Health.Port == 0 {
    cfg.Indexer.Health = defaultHealth()
}

// Env back-compat: if Indexer.Nodes is empty, build from NODE_URL_WS + NODE_URL_FALLBACKS.
// Existing NodeConfig.WebSocketURL is still read for back-compat with old code paths;
// we copy it into Indexer.Nodes[0] when Nodes is empty.
if len(cfg.Indexer.Nodes) == 0 {
    if cfg.Node.WebSocketURL == "" {
        // existing validation already errors here; nothing to do
    } else {
        cfg.Indexer.Nodes = append(cfg.Indexer.Nodes, NodeEntry{
            URL: cfg.Node.WebSocketURL, Label: "primary",
        })
    }
    if fb := os.Getenv("NODE_URL_FALLBACKS"); fb != "" {
        for i, u := range strings.Split(fb, ",") {
            u = strings.TrimSpace(u)
            if u == "" {
                continue
            }
            cfg.Indexer.Nodes = append(cfg.Indexer.Nodes, NodeEntry{
                URL:   u,
                Label: fmt.Sprintf("fallback-%d", i+1),
            })
        }
    }
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/config/ -run 'TestNodesConfig|TestWatchdogConfig'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/config/config.go internal/config/config_test.go
git commit -m "feat(config): add IndexerConfig with nodes, watchdog, and health"
```

---

## Task 5: `stats.syncInfo` JSON-RPC caller

**Files:**
- Create: `internal/indexer/syncinfo.go`
- Create: `internal/indexer/syncinfo_test.go`

The SDK does not expose `StatsApi`. We make a raw JSON-RPC POST against the same HTTP endpoint the SDK uses. The live znnd returns:
```
{"jsonrpc":"2.0","id":1,"result":{"state":2,"currentHeight":N,"targetHeight":N}}
```
`state == 2` means synced; other values mean syncing. Use the URL the user configured (we'll pass it directly so we can also probe candidates).

- [ ] **Step 1: Write the failing test**

```go
// internal/indexer/syncinfo_test.go
package indexer

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"
)

func TestSyncInfoHappyPath(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        _ = json.NewEncoder(w).Encode(map[string]any{
            "jsonrpc": "2.0",
            "id":      1,
            "result": map[string]any{
                "state":         2,
                "currentHeight": 13366681,
                "targetHeight":  13366681,
            },
        })
    }))
    defer srv.Close()

    info, err := fetchSyncInfo(context.Background(), srv.URL)
    if err != nil {
        t.Fatalf("fetchSyncInfo: %v", err)
    }
    if info.State != 2 || info.CurrentHeight != 13366681 || info.TargetHeight != 13366681 {
        t.Fatalf("unexpected sync info: %#v", info)
    }
}

func TestSyncInfoRPCError(t *testing.T) {
    srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        _ = json.NewEncoder(w).Encode(map[string]any{
            "jsonrpc": "2.0",
            "id":      1,
            "error": map[string]any{
                "code":    -32601,
                "message": "method not found",
            },
        })
    }))
    defer srv.Close()

    _, err := fetchSyncInfo(context.Background(), srv.URL)
    if err == nil {
        t.Fatal("expected error on RPC error response")
    }
}
```

- [ ] **Step 2: Run, confirm fail (undefined)**

```bash
GOWORK=off go test ./internal/indexer/ -run TestSyncInfo
```

Expected: `undefined: fetchSyncInfo`.

- [ ] **Step 3: Implement**

```go
// internal/indexer/syncinfo.go
package indexer

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "time"
)

// SyncInfo mirrors the result shape of stats.syncInfo as observed on the
// live znnd: {"state":2,"currentHeight":N,"targetHeight":N}.
// State==2 means synced; smaller values mean syncing.
type SyncInfo struct {
    State         int    `json:"state"`
    CurrentHeight uint64 `json:"currentHeight"`
    TargetHeight  uint64 `json:"targetHeight"`
}

// IsSynced reports whether znnd considers itself caught up to its peers.
func (s SyncInfo) IsSynced() bool { return s.State == 2 }

// fetchSyncInfo calls stats.syncInfo against the given URL. The URL may
// be either http(s):// (used as-is) or ws(s):// (transparently rewritten
// to http(s)://; znnd serves JSON-RPC on the same host/port via HTTP).
func fetchSyncInfo(ctx context.Context, url string) (SyncInfo, error) {
    httpURL := strings.Replace(strings.Replace(url, "wss://", "https://", 1), "ws://", "http://", 1)

    body, _ := json.Marshal(map[string]any{
        "jsonrpc": "2.0", "id": 1,
        "method": "stats.syncInfo", "params": []any{},
    })

    req, err := http.NewRequestWithContext(ctx, "POST", httpURL, bytes.NewReader(body))
    if err != nil {
        return SyncInfo{}, fmt.Errorf("build request: %w", err)
    }
    req.Header.Set("Content-Type", "application/json")

    client := &http.Client{Timeout: 5 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return SyncInfo{}, fmt.Errorf("syncInfo POST: %w", err)
    }
    defer resp.Body.Close()

    var envelope struct {
        Result *SyncInfo `json:"result"`
        Error  *struct {
            Code    int    `json:"code"`
            Message string `json:"message"`
        } `json:"error"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
        return SyncInfo{}, fmt.Errorf("decode syncInfo: %w", err)
    }
    if envelope.Error != nil {
        return SyncInfo{}, fmt.Errorf("syncInfo rpc error %d: %s",
            envelope.Error.Code, envelope.Error.Message)
    }
    if envelope.Result == nil {
        return SyncInfo{}, fmt.Errorf("syncInfo empty result")
    }
    return *envelope.Result, nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/indexer/ -run TestSyncInfo
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/syncinfo.go internal/indexer/syncinfo_test.go
git commit -m "feat(indexer): add stats.syncInfo raw JSON-RPC caller"
```

---

## Task 6: `NodePool` with `Probe`

**Files:**
- Create: `internal/indexer/nodepool.go`
- Create: `internal/indexer/nodepool_test.go`

The pool holds the ordered list and one cached `*rpc_client.RpcClient` per URL for probes (so we don't reconnect a WS every tick). `Probe(ctx, idx)` calls both `fetchSyncInfo` and `LedgerApi.GetFrontierMomentum()` and returns a unified `ProbeResult`.

Genesis hash (the chain identifier) is fetched lazily on the *first* probe per node and cached on the pool, so it doesn't add per-tick load.

**Deviation from spec — note for the implementer.** The spec mentions
wrapping probe RPCs with `withRetry`. This implementation uses a raw
`http.Client` with a 5s timeout instead, deliberately: the watchdog
ticks every 30s, so a failed probe is naturally retried on the next
tick. Adding `withRetry` inside `Probe` would compound latency
(`withRetry` can take ~32s) and double-count the same RPC as a
"probe_failed" event. If subsequent operational experience shows
single-tick flakiness too often, swap the raw client for `withRetry`
then.

- [ ] **Step 1: Write the failing test**

```go
// internal/indexer/nodepool_test.go
package indexer

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "testing"

    "go.uber.org/zap"
)

func fakeJSONRPC(t *testing.T, responses map[string]any) *httptest.Server {
    t.Helper()
    return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req struct {
            Method string `json:"method"`
        }
        _ = json.NewDecoder(r.Body).Decode(&req)
        resp := map[string]any{"jsonrpc": "2.0", "id": 1}
        if result, ok := responses[req.Method]; ok {
            resp["result"] = result
        } else {
            resp["error"] = map[string]any{"code": -32601, "message": "method not found"}
        }
        _ = json.NewEncoder(w).Encode(resp)
    }))
}

func TestNodePoolProbeSynced(t *testing.T) {
    srv := fakeJSONRPC(t, map[string]any{
        "stats.syncInfo": map[string]any{
            "state": 2, "currentHeight": 100, "targetHeight": 100,
        },
        "ledger.getFrontierMomentum": map[string]any{"height": 100},
        "ledger.getMomentumsByHeight": map[string]any{
            "count": 1,
            "list":  []any{map[string]any{"height": 1, "hash": "genesis-hash"}},
        },
    })
    defer srv.Close()

    pool := NewNodePool([]NodeEntry{{URL: srv.URL, Label: "test"}}, zap.NewNop())
    result, err := pool.Probe(context.Background(), 0)
    if err != nil {
        t.Fatalf("Probe: %v", err)
    }
    if result.Frontier != 100 || result.Target != 100 {
        t.Fatalf("frontier/target: %d/%d", result.Frontier, result.Target)
    }
    if result.GenesisHash != "genesis-hash" {
        t.Fatalf("genesis hash: %q", result.GenesisHash)
    }
}

func TestNodePoolProbeUnreachable(t *testing.T) {
    pool := NewNodePool([]NodeEntry{{URL: "http://127.0.0.1:1", Label: "dead"}}, zap.NewNop())
    _, err := pool.Probe(context.Background(), 0)
    if err == nil {
        t.Fatal("expected probe failure")
    }
}
```

- [ ] **Step 2: Run, confirm fail (undefined types)**

```bash
GOWORK=off go test ./internal/indexer/ -run TestNodePool
```

Expected: `undefined: NewNodePool, ProbeResult`.

- [ ] **Step 3: Implement**

```go
// internal/indexer/nodepool.go
package indexer

import (
    "bytes"
    "context"
    "encoding/json"
    "fmt"
    "net/http"
    "strings"
    "sync"
    "time"

    "go.uber.org/zap"
)

type NodeEntry struct {
    URL   string
    Label string
}

// NodePool owns the ordered list of node entries and a cache of per-URL
// probe state. It does NOT own the active SDK client used by the
// subscription/RPC pipeline — that lives on the Indexer.
type NodePool struct {
    entries []NodeEntry
    logger  *zap.Logger

    mu       sync.Mutex
    genesisHash map[int]string // cache: nodeIdx -> first-momentum hash
}

func NewNodePool(entries []NodeEntry, logger *zap.Logger) *NodePool {
    return &NodePool{
        entries:     entries,
        logger:      logger,
        genesisHash: make(map[int]string),
    }
}

// Len reports the number of configured nodes.
func (p *NodePool) Len() int { return len(p.entries) }

// Entry returns the n-th node (panics on out-of-range — caller's bug).
func (p *NodePool) Entry(idx int) NodeEntry { return p.entries[idx] }

// ProbeResult is the per-tick health snapshot of a single node.
type ProbeResult struct {
    Frontier    uint64
    Target      uint64
    State       int    // 2 = synced
    GenesisHash string // chain identifier; cached after first successful probe
    Latency     time.Duration
}

// Probe issues stats.syncInfo + ledger.getFrontierMomentum + (first
// time only) ledger.getMomentumsByHeight(1, 1). Returns an error if
// any of the calls fail.
func (p *NodePool) Probe(ctx context.Context, idx int) (ProbeResult, error) {
    if idx < 0 || idx >= len(p.entries) {
        return ProbeResult{}, fmt.Errorf("probe: idx %d out of range", idx)
    }
    url := p.entries[idx].URL
    start := time.Now()

    info, err := fetchSyncInfo(ctx, url)
    if err != nil {
        return ProbeResult{}, fmt.Errorf("syncInfo: %w", err)
    }

    frontier, err := callForUint64(ctx, url, "ledger.getFrontierMomentum", []any{}, "height")
    if err != nil {
        return ProbeResult{}, fmt.Errorf("frontier: %w", err)
    }

    genesis, err := p.genesisFor(ctx, idx, url)
    if err != nil {
        return ProbeResult{}, fmt.Errorf("genesis: %w", err)
    }

    return ProbeResult{
        Frontier:    frontier,
        Target:      info.TargetHeight,
        State:       info.State,
        GenesisHash: genesis,
        Latency:     time.Since(start),
    }, nil
}

func (p *NodePool) genesisFor(ctx context.Context, idx int, url string) (string, error) {
    p.mu.Lock()
    if h, ok := p.genesisHash[idx]; ok {
        p.mu.Unlock()
        return h, nil
    }
    p.mu.Unlock()

    h, err := callForString(ctx, url, "ledger.getMomentumsByHeight", []any{1, 1}, "list.0.hash")
    if err != nil {
        return "", err
    }
    p.mu.Lock()
    p.genesisHash[idx] = h
    p.mu.Unlock()
    return h, nil
}

// rpcURL rewrites ws(s):// to http(s):// for JSON-RPC over HTTP.
func rpcURL(u string) string {
    u = strings.Replace(u, "wss://", "https://", 1)
    u = strings.Replace(u, "ws://", "http://", 1)
    return u
}

// callForUint64 / callForString are minimal JSON-RPC helpers parameterised
// by a dotted JSON path into the result object. They support enough of
// JSON traversal for our two call sites; anything more belongs in a
// proper RPC client.
func rpcCall(ctx context.Context, url, method string, params []any) (any, error) {
    body, _ := json.Marshal(map[string]any{
        "jsonrpc": "2.0", "id": 1, "method": method, "params": params,
    })
    req, err := http.NewRequestWithContext(ctx, "POST", rpcURL(url), bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    req.Header.Set("Content-Type", "application/json")
    resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    var envelope struct {
        Result any `json:"result"`
        Error  *struct {
            Code    int    `json:"code"`
            Message string `json:"message"`
        } `json:"error"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
        return nil, err
    }
    if envelope.Error != nil {
        return nil, fmt.Errorf("rpc %s: %s", method, envelope.Error.Message)
    }
    return envelope.Result, nil
}

func navigate(v any, path string) (any, error) {
    if path == "" {
        return v, nil
    }
    parts := strings.Split(path, ".")
    for _, p := range parts {
        switch cur := v.(type) {
        case map[string]any:
            v = cur[p]
        case []any:
            // numeric index
            var idx int
            _, _ = fmt.Sscanf(p, "%d", &idx)
            if idx < 0 || idx >= len(cur) {
                return nil, fmt.Errorf("index %d oob at %q", idx, p)
            }
            v = cur[idx]
        default:
            return nil, fmt.Errorf("non-object at %q", p)
        }
    }
    return v, nil
}

func callForUint64(ctx context.Context, url, method string, params []any, path string) (uint64, error) {
    res, err := rpcCall(ctx, url, method, params)
    if err != nil {
        return 0, err
    }
    leaf, err := navigate(res, path)
    if err != nil {
        return 0, err
    }
    switch v := leaf.(type) {
    case float64:
        return uint64(v), nil
    case json.Number:
        n, _ := v.Int64()
        return uint64(n), nil
    default:
        return 0, fmt.Errorf("expected number at %s, got %T", path, leaf)
    }
}

func callForString(ctx context.Context, url, method string, params []any, path string) (string, error) {
    res, err := rpcCall(ctx, url, method, params)
    if err != nil {
        return "", err
    }
    leaf, err := navigate(res, path)
    if err != nil {
        return "", err
    }
    s, ok := leaf.(string)
    if !ok {
        return "", fmt.Errorf("expected string at %s, got %T", path, leaf)
    }
    return s, nil
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/indexer/ -run TestNodePool
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/nodepool.go internal/indexer/nodepool_test.go
git commit -m "feat(indexer): add NodePool with probe + genesis-hash caching"
```

---

## Task 7: Make `Indexer.client` an `atomic.Pointer`

**Files:**
- Modify: `internal/indexer/indexer.go`

This is an internal refactor. The public constructor (`NewIndexerWithCron`) still takes a `*rpc_client.RpcClient`. Internally we hold an `atomic.Pointer` and provide `i.client()` accessor for read sites.

- [ ] **Step 1: Sketch the refactor**

```go
// internal/indexer/indexer.go — replace the `client` field and add accessor

import (
    // existing imports
    "sync/atomic"
)

type Indexer struct {
    activeClient atomic.Pointer[rpc_client.RpcClient]
    // ... other fields unchanged ...
}

func NewIndexerWithCron(client *rpc_client.RpcClient, pool *pgxpool.Pool, logger *zap.Logger, cron CronConfig) *Indexer {
    i := &Indexer{
        pool:              pool,
        repos:             repository.NewRepositories(pool),
        logger:            logger,
        cron:              cron,
        pillarNameToOwner: make(map[string]string),
        restartSubCh:      make(chan struct{}, 1),
    }
    i.activeClient.Store(client)
    return i
}

// client returns the currently-active SDK client. All RPC call sites
// must read through this — the underlying value can change at runtime
// when the watchdog fails over.
func (i *Indexer) client() *rpc_client.RpcClient {
    return i.activeClient.Load()
}
```

- [ ] **Step 2: Update all read sites**

Replace every occurrence of `i.client.` with `i.client().` in:
- `internal/indexer/indexer.go`
- `internal/indexer/processor.go`
- Any other file in `internal/indexer/` that references `i.client`.

Quick find:
```bash
GOWORK=off grep -rn '\bi\.client\b' internal/indexer/
```

Each result that is `i.client.X` becomes `i.client().X`. Do NOT touch references inside string literals or comments.

- [ ] **Step 3: Update connection callback registration**

In `Indexer.Run()`, the existing lines:
```go
i.client.AddOnConnectionEstablishedCallback(func() { ... })
i.client.AddOnConnectionLostCallback(func(err error) { ... })
```
become:
```go
i.client().AddOnConnectionEstablishedCallback(func() { ... })
i.client().AddOnConnectionLostCallback(func(err error) { ... })
```

Note: these callbacks must be re-registered after each `swapClient` — see Task 8 (we'll register them in a helper called from both `Run()` and `swapClient`).

- [ ] **Step 4: Build & run existing tests**

```bash
GOWORK=off go build ./...
GOWORK=off go test ./internal/indexer/...
```

Expected: everything compiles and existing tests still pass. No new tests in this task — it's a pure refactor protected by the existing suite.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/
git commit -m "refactor(indexer): hold SDK client in atomic.Pointer for hot-swap"
```

---

## Task 8: `swapClient` with genesis-hash verification

**Files:**
- Create: `internal/indexer/client_switch.go`
- Create: `internal/indexer/client_switch_test.go`

`swapClient`:
1. Builds a new `rpc_client.RpcClient` for the candidate URL.
2. Calls `fetchSyncInfo` against it (sanity ping).
3. Confirms the new client's genesis hash matches the stored chain identifier.
4. Atomically publishes the new client.
5. Sends `signalSubscriptionRestart()`.
6. Re-registers the SDK connection callbacks on the new client.
7. Schedules `oldClient.Stop()` after 60s grace.

- [ ] **Step 1: Write the failing test**

```go
// internal/indexer/client_switch_test.go
package indexer

import (
    "context"
    "testing"
    "time"

    "go.uber.org/zap"
)

func TestSwapClientRefusesOnChainMismatch(t *testing.T) {
    // primary returns "genesis-A"
    primary := fakeJSONRPC(t, map[string]any{
        "stats.syncInfo":            map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
        "ledger.getFrontierMomentum": map[string]any{"height": 100},
        "ledger.getMomentumsByHeight": map[string]any{"count": 1, "list": []any{map[string]any{"hash": "genesis-A"}}},
    })
    defer primary.Close()

    // candidate returns "genesis-B"
    candidate := fakeJSONRPC(t, map[string]any{
        "stats.syncInfo":            map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
        "ledger.getFrontierMomentum": map[string]any{"height": 100},
        "ledger.getMomentumsByHeight": map[string]any{"count": 1, "list": []any{map[string]any{"hash": "genesis-B"}}},
    })
    defer candidate.Close()

    pool := NewNodePool([]NodeEntry{
        {URL: primary.URL, Label: "primary"},
        {URL: candidate.URL, Label: "candidate"},
    }, zap.NewNop())

    // Seed the stored chain identifier from primary.
    storedGenesis := "genesis-A"

    err := swapClient(context.Background(), zap.NewNop(), pool, 1, storedGenesis,
        func(newURL string) error { t.Fatal("publish should not run on mismatch"); return nil },
        func() {})
    if err == nil {
        t.Fatal("expected error on chain mismatch")
    }
}

func TestSwapClientPublishesOnMatch(t *testing.T) {
    candidate := fakeJSONRPC(t, map[string]any{
        "stats.syncInfo":            map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
        "ledger.getFrontierMomentum": map[string]any{"height": 100},
        "ledger.getMomentumsByHeight": map[string]any{"count": 1, "list": []any{map[string]any{"hash": "genesis-shared"}}},
    })
    defer candidate.Close()

    pool := NewNodePool([]NodeEntry{
        {URL: "http://unused", Label: "primary"},
        {URL: candidate.URL, Label: "candidate"},
    }, zap.NewNop())

    var publishedURL string
    publish := func(newURL string) error { publishedURL = newURL; return nil }
    restartCalled := false
    restart := func() { restartCalled = true }

    if err := swapClient(context.Background(), zap.NewNop(), pool, 1, "genesis-shared", publish, restart); err != nil {
        t.Fatalf("swapClient: %v", err)
    }
    if publishedURL != candidate.URL {
        t.Fatalf("published URL: %q", publishedURL)
    }
    if !restartCalled {
        t.Fatal("restart was not called")
    }
    _ = time.Now() // silence unused
}
```

- [ ] **Step 2: Run, confirm fail (undefined)**

```bash
GOWORK=off go test ./internal/indexer/ -run TestSwapClient
```

Expected: `undefined: swapClient`.

- [ ] **Step 3: Implement**

```go
// internal/indexer/client_switch.go
package indexer

import (
    "context"
    "fmt"

    "go.uber.org/zap"
)

// swapClient verifies that the candidate node is on the expected chain,
// then publishes a new client via the supplied publish() and signals the
// subscription to restart via restart(). The publish/restart callbacks
// are injected so this function is unit-testable without an Indexer.
//
// storedGenesis is the canonical chain identifier (genesis momentum hash)
// previously recorded in indexer_sync_status. Pass "" on first-ever run
// to skip the comparison.
func swapClient(
    ctx context.Context,
    logger *zap.Logger,
    pool *NodePool,
    candidateIdx int,
    storedGenesis string,
    publish func(newURL string) error,
    restart func(),
) error {
    if candidateIdx < 0 || candidateIdx >= pool.Len() {
        return fmt.Errorf("swapClient: idx %d out of range", candidateIdx)
    }
    entry := pool.Entry(candidateIdx)

    result, err := pool.Probe(ctx, candidateIdx)
    if err != nil {
        return fmt.Errorf("probe candidate %q: %w", entry.Label, err)
    }
    if storedGenesis != "" && result.GenesisHash != storedGenesis {
        return fmt.Errorf("chain mismatch for %q: candidate genesis=%q, stored=%q",
            entry.Label, result.GenesisHash, storedGenesis)
    }

    if err := publish(entry.URL); err != nil {
        return fmt.Errorf("publish new client: %w", err)
    }
    restart()
    logger.Info("swapped active client",
        zap.String("label", entry.Label),
        zap.String("url", entry.URL))
    return nil
}
```

The integration into the Indexer (which actually builds the `*rpc_client.RpcClient` and calls `i.activeClient.Store`) happens in Task 13. This task ships only the verification logic.

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/indexer/ -run TestSwapClient
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/client_switch.go internal/indexer/client_switch_test.go
git commit -m "feat(indexer): add swapClient with chain-identity verification"
```

---

## Task 9: `classify()` pure function

**Files:**
- Create: `internal/indexer/watchdog.go` (start of file, just classify)
- Create: `internal/indexer/watchdog_test.go` (classify tests only)

The classifier has no goroutines, no state — pure inputs to a single enum out. Table-test it.

Precedence (per spec):
1. probe_failed
2. stalled
3. node_lagging
4. indexer_lagging
5. synced

- [ ] **Step 1: Write the failing tests**

```go
// internal/indexer/watchdog_test.go
package indexer

import (
    "errors"
    "testing"
    "time"
)

func TestClassify(t *testing.T) {
    cfg := classifyConfig{
        StallThreshold:        60 * time.Second,
        IndexerDriftThreshold: 3,
        NodeDriftThreshold:    3,
    }
    now := time.Unix(2000, 0)

    cases := []struct {
        name           string
        probe          ProbeResult
        probeErr       error
        dbHeight       int64
        lastProgressAt time.Time
        want           syncClass
    }{
        {"probe failure wins", ProbeResult{}, errors.New("nope"),
            100, now, classProbeFailed},
        {"stalled wins over node_lagging",
            ProbeResult{Frontier: 200, Target: 300},
            nil, 100, now.Add(-2 * time.Minute), classStalled},
        {"node_lagging wins over indexer_lagging",
            ProbeResult{Frontier: 100, Target: 200},
            nil, 50, now, classNodeLagging},
        {"indexer_lagging",
            ProbeResult{Frontier: 100, Target: 100},
            nil, 50, now, classIndexerLagging},
        {"synced",
            ProbeResult{Frontier: 100, Target: 100},
            nil, 100, now, classSynced},
        {"synced with 1-momentum drift (under threshold)",
            ProbeResult{Frontier: 100, Target: 100},
            nil, 99, now, classSynced},
    }
    for _, tc := range cases {
        t.Run(tc.name, func(t *testing.T) {
            got := classify(tc.probe, tc.probeErr, tc.dbHeight, tc.lastProgressAt, now, cfg)
            if got != tc.want {
                t.Fatalf("classify: got %s, want %s", got, tc.want)
            }
        })
    }
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
GOWORK=off go test ./internal/indexer/ -run TestClassify
```

Expected: undefined `classify, classifyConfig, syncClass, classProbeFailed, …`.

- [ ] **Step 3: Implement**

```go
// internal/indexer/watchdog.go
package indexer

import (
    "time"
)

type syncClass int

const (
    classSynced syncClass = iota
    classIndexerLagging
    classNodeLagging
    classStalled
    classProbeFailed
)

func (c syncClass) String() string {
    return [...]string{"synced", "indexer_lagging", "node_lagging", "stalled", "probe_failed"}[c]
}

type classifyConfig struct {
    StallThreshold        time.Duration
    IndexerDriftThreshold int64
    NodeDriftThreshold    int64
}

// classify maps a single tick's observations to exactly one class.
// Precedence (first match wins): probe_failed > stalled > node_lagging >
// indexer_lagging > synced. See spec for rationale.
func classify(
    probe ProbeResult,
    probeErr error,
    dbHeight int64,
    lastProgressAt time.Time,
    now time.Time,
    cfg classifyConfig,
) syncClass {
    if probeErr != nil {
        return classProbeFailed
    }
    if now.Sub(lastProgressAt) > cfg.StallThreshold {
        return classStalled
    }
    if int64(probe.Target)-int64(probe.Frontier) > cfg.NodeDriftThreshold {
        return classNodeLagging
    }
    if int64(probe.Frontier)-dbHeight > cfg.IndexerDriftThreshold {
        return classIndexerLagging
    }
    return classSynced
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/indexer/ -run TestClassify
```

Expected: all 6 cases PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/watchdog.go internal/indexer/watchdog_test.go
git commit -m "feat(indexer): add watchdog classify() with priority order"
```

---

## Task 10: Streak state + reaction dispatch

**Files:**
- Modify: `internal/indexer/watchdog.go` (append types + react function)
- Modify: `internal/indexer/watchdog_test.go` (append tests)

`syncState` holds per-node streaks. `react()` consumes a classification and decides which side effects fire (restart? failover? failback?). It returns a struct describing intent; the side effects themselves are wired in Task 13 to keep this unit testable.

- [ ] **Step 1: Write the failing tests**

```go
// internal/indexer/watchdog_test.go — append

func TestReactSyncedResetsBadStreak(t *testing.T) {
    state := newSyncState(2)
    state.streaks[0] = nodeStreaks{unhealthy: 5}
    cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

    intent := react(state, 0, classSynced, cfg)
    if intent.signalRestart || intent.failoverIdx != -1 || intent.failbackIdx != -1 {
        t.Fatalf("synced should produce no intent on primary: %+v", intent)
    }
    if state.streaks[0].unhealthy != 0 {
        t.Fatalf("expected unhealthy streak reset, got %d", state.streaks[0].unhealthy)
    }
    if state.streaks[0].healthy != 1 {
        t.Fatalf("expected healthy streak 1, got %d", state.streaks[0].healthy)
    }
}

func TestReactIndexerLaggingSignalsRestart(t *testing.T) {
    state := newSyncState(2)
    cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

    intent := react(state, 0, classIndexerLagging, cfg)
    if !intent.signalRestart {
        t.Fatal("expected restart signal")
    }
    if intent.failoverIdx != -1 {
        t.Fatal("expected no failover")
    }
}

func TestReactNodeLaggingFailoverAfterStreak(t *testing.T) {
    state := newSyncState(2)
    cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}

    intent := react(state, 0, classNodeLagging, cfg)
    if intent.failoverIdx != -1 {
        t.Fatalf("expected no failover at streak=1, got idx %d", intent.failoverIdx)
    }
    intent = react(state, 0, classNodeLagging, cfg)
    if intent.failoverIdx == -1 {
        t.Fatal("expected failover at streak=2")
    }
}

func TestReactStalledTriggersRestartAndStreak(t *testing.T) {
    state := newSyncState(2)
    cfg := watchdogReactConfig{UnhealthyStreak: 2, FailbackStreak: 5}
    intent := react(state, 0, classStalled, cfg)
    if !intent.signalRestart {
        t.Fatal("expected restart signal on stalled")
    }
    if state.streaks[0].unhealthy != 1 {
        t.Fatalf("expected unhealthy streak 1, got %d", state.streaks[0].unhealthy)
    }
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
GOWORK=off go test ./internal/indexer/ -run TestReact
```

Expected: undefined `react, newSyncState, syncState, reactIntent, nodeStreaks, watchdogReactConfig`.

- [ ] **Step 3: Implement**

```go
// internal/indexer/watchdog.go — append

type nodeStreaks struct {
    healthy   int
    unhealthy int
}

type syncState struct {
    activeIdx       int
    lastProgressAt  time.Time
    streaks         map[int]nodeStreaks
    chainIdentifier string // genesis hash, populated on first successful probe
    failedOverAt    *int64 // unix seconds; nil on primary
}

func newSyncState(numNodes int) *syncState {
    s := &syncState{
        streaks: make(map[int]nodeStreaks, numNodes),
    }
    for i := 0; i < numNodes; i++ {
        s.streaks[i] = nodeStreaks{}
    }
    return s
}

type watchdogReactConfig struct {
    UnhealthyStreak int
    FailbackStreak  int
}

// reactIntent is the side-effect plan produced by react(). The watchdog
// goroutine reads it and issues the actual restart/swap calls. Keeping
// react() pure means we can table-test streak logic without spinning up
// goroutines or fake clients.
type reactIntent struct {
    signalRestart bool
    failoverIdx   int // -1 if no failover
    failbackIdx   int // -1 if no failback
}

func react(s *syncState, activeIdx int, c syncClass, cfg watchdogReactConfig) reactIntent {
    intent := reactIntent{failoverIdx: -1, failbackIdx: -1}
    st := s.streaks[activeIdx]

    switch c {
    case classSynced:
        st.unhealthy = 0
        st.healthy++
        s.streaks[activeIdx] = st
        // Failback path is handled separately in Task 11 because it
        // requires probing other nodes; here we just maintain state.

    case classIndexerLagging:
        intent.signalRestart = true
        // do not touch node streaks — indexer is the laggard

    case classNodeLagging:
        st.unhealthy++
        st.healthy = 0
        s.streaks[activeIdx] = st
        if st.unhealthy >= cfg.UnhealthyStreak {
            intent.failoverIdx = activeIdx // placeholder; real idx chosen in Task 11
        }

    case classStalled:
        // first-tick cheap fix
        intent.signalRestart = true
        st.unhealthy++
        s.streaks[activeIdx] = st
        if st.unhealthy >= cfg.UnhealthyStreak {
            intent.failoverIdx = activeIdx
        }

    case classProbeFailed:
        st.unhealthy++
        s.streaks[activeIdx] = st
        if st.unhealthy >= cfg.UnhealthyStreak {
            intent.failoverIdx = activeIdx
        }
    }
    return intent
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/indexer/ -run TestReact
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/watchdog.go internal/indexer/watchdog_test.go
git commit -m "feat(indexer): add per-node streaks and react() dispatch"
```

---

## Task 11: Failover and failback selection

**Files:**
- Modify: `internal/indexer/watchdog.go` (append)
- Modify: `internal/indexer/watchdog_test.go` (append)

`selectFailoverTarget(pool, currentIdx, storedGenesis)` walks lower-priority candidates and returns the first healthy one (or -1).

`selectFailbackTarget(state, pool, currentIdx, healthyProbe map[int]ProbeResult, cfg)` walks higher-priority nodes, advances their healthy streaks (using probes the watchdog already made this tick), and returns the first one whose `healthy >= FailbackStreak`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/indexer/watchdog_test.go — append

func TestSelectFailoverPicksFirstHealthy(t *testing.T) {
    badSrv := fakeJSONRPC(t, map[string]any{
        // node_lagging: target far ahead of frontier
        "stats.syncInfo":            map[string]any{"state": 1, "currentHeight": 100, "targetHeight": 200},
        "ledger.getFrontierMomentum": map[string]any{"height": 100},
        "ledger.getMomentumsByHeight": map[string]any{"count": 1, "list": []any{map[string]any{"hash": "G"}}},
    })
    defer badSrv.Close()
    goodSrv := fakeJSONRPC(t, map[string]any{
        "stats.syncInfo":            map[string]any{"state": 2, "currentHeight": 100, "targetHeight": 100},
        "ledger.getFrontierMomentum": map[string]any{"height": 100},
        "ledger.getMomentumsByHeight": map[string]any{"count": 1, "list": []any{map[string]any{"hash": "G"}}},
    })
    defer goodSrv.Close()

    pool := NewNodePool([]NodeEntry{
        {URL: "http://primary-dead", Label: "primary"},
        {URL: badSrv.URL, Label: "bad-fb"},
        {URL: goodSrv.URL, Label: "good-fb"},
    }, zap.NewNop())

    cfg := classifyConfig{NodeDriftThreshold: 3}
    idx := selectFailoverTarget(context.Background(), pool, 0, "G", cfg)
    if idx != 2 {
        t.Fatalf("expected idx 2, got %d", idx)
    }
}

func TestSelectFailbackRequiresStreak(t *testing.T) {
    state := newSyncState(2)
    state.activeIdx = 1
    cfg := watchdogReactConfig{FailbackStreak: 3}

    // 1st healthy primary probe — streak advances to 1
    if selectFailback(state, 0, cfg) != -1 {
        t.Fatal("should not failback at streak 1")
    }
    if selectFailback(state, 0, cfg) != -1 {
        t.Fatal("should not failback at streak 2")
    }
    if got := selectFailback(state, 0, cfg); got != 0 {
        t.Fatalf("should failback at streak 3, got %d", got)
    }
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
GOWORK=off go test ./internal/indexer/ -run 'TestSelectFailover|TestSelectFailback'
```

Expected: undefined `selectFailoverTarget, selectFailback`.

- [ ] **Step 3: Implement**

```go
// internal/indexer/watchdog.go — append
import "context"

// selectFailoverTarget walks candidates starting *after* currentIdx and
// returns the first healthy + chain-matching one, or -1 if none.
func selectFailoverTarget(
    ctx context.Context,
    pool *NodePool,
    currentIdx int,
    storedGenesis string,
    cfg classifyConfig,
) int {
    for idx := currentIdx + 1; idx < pool.Len(); idx++ {
        probe, err := pool.Probe(ctx, idx)
        if err != nil {
            continue
        }
        if int64(probe.Target)-int64(probe.Frontier) > cfg.NodeDriftThreshold {
            continue
        }
        if storedGenesis != "" && probe.GenesisHash != storedGenesis {
            continue
        }
        return idx
    }
    return -1
}

// selectFailback advances the healthy-streak counter for the given
// higher-priority candidate and returns its idx when it crosses the
// FailbackStreak threshold. Mutates s.streaks[candidateIdx].
func selectFailback(s *syncState, candidateIdx int, cfg watchdogReactConfig) int {
    st := s.streaks[candidateIdx]
    st.healthy++
    s.streaks[candidateIdx] = st
    if st.healthy >= cfg.FailbackStreak {
        return candidateIdx
    }
    return -1
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/indexer/ -run 'TestSelectFailover|TestSelectFailback'
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/watchdog.go internal/indexer/watchdog_test.go
git commit -m "feat(indexer): add failover/failback selection"
```

---

## Task 12: `lastProgressAt` updates from the subscription loop

**Files:**
- Modify: `internal/indexer/indexer.go` (add `lastProgressAt atomic.Int64` field + accessor)
- Modify: `internal/indexer/indexer.go` or `internal/indexer/processor.go` (set it after each `processMomentum` commit)

`lastProgressAt` is a stdlib `atomic.Int64` (Unix seconds) so the watchdog can read it lock-free.

- [ ] **Step 1: Add the field and initialise it in Run**

```go
// internal/indexer/indexer.go — additions

type Indexer struct {
    // ... existing fields ...
    activeClient   atomic.Pointer[rpc_client.RpcClient]
    lastProgressAt atomic.Int64 // Unix seconds; last successful processMomentum commit
}

// Inside Run(), before the goroutines launch:
i.lastProgressAt.Store(time.Now().Unix())
```

- [ ] **Step 2: Update the field after each successful momentum**

In `runSubscriptionSession`, find the existing block (around `indexer.go:306`):
```go
if err := i.processMomentum(ctx, fullMomentum.List[0]); err != nil {
    i.logger.Error("failed to process momentum", ...)
}
```
Replace with:
```go
if err := i.processMomentum(ctx, fullMomentum.List[0]); err != nil {
    i.logger.Error("failed to process momentum", ...)
} else {
    i.lastProgressAt.Store(time.Now().Unix())
}
```

Also update the `sync()` catch-up loop in the same way: after each successful `processMomentum` call inside the batch loop, store the time.

- [ ] **Step 3: Verify it compiles and existing tests pass**

```bash
GOWORK=off go build ./...
GOWORK=off go test ./internal/indexer/...
```

Expected: all tests still PASS. No new test in this task — covered downstream in Task 13's integration test.

- [ ] **Step 4: Commit**

```bash
git add internal/indexer/indexer.go internal/indexer/processor.go
git commit -m "feat(indexer): record lastProgressAt on every successful momentum commit"
```

---

## Task 13: `runSyncWatchdogLoop` + integration in `Indexer`

**Files:**
- Modify: `internal/indexer/watchdog.go` (append the loop)
- Modify: `internal/indexer/indexer.go` (add NodePool field, launch loop in `Run`, expose `swapActiveClient`)

This is the wiring task — composes everything from Tasks 5–11.

- [ ] **Step 1: Add the NodePool field + swap helper**

```go
// internal/indexer/indexer.go — additions
import (
    // existing
    "sync"
)

type Indexer struct {
    // ... existing ...
    nodePool *NodePool

    syncStateMu sync.RWMutex
    syncStateInternal *syncState // protected by syncStateMu
}

// NewIndexerWithNodes is the new constructor used by cmd/indexer/main.go
// once a node pool is built from config. The existing
// NewIndexerWithCron is kept for back-compat with any tests/callers that
// only pass a single client; internally it constructs a 1-node pool.
func NewIndexerWithNodes(
    pool *pgxpool.Pool,
    nodePool *NodePool,
    client *rpc_client.RpcClient,
    logger *zap.Logger,
    cron CronConfig,
) *Indexer {
    i := NewIndexerWithCron(client, pool, logger, cron)
    i.nodePool = nodePool
    i.syncStateInternal = newSyncState(nodePool.Len())
    return i
}

// swapActiveClient builds a fresh SDK client for url, atomically stores it,
// reregisters callbacks, and schedules the old client's Stop after grace.
func (i *Indexer) swapActiveClient(url string) error {
    newClient, err := rpc_client.NewRpcClient(url)
    if err != nil {
        return fmt.Errorf("build client for %q: %w", url, err)
    }
    i.registerCallbacks(newClient)
    old := i.activeClient.Swap(newClient)
    go func() {
        time.Sleep(60 * time.Second)
        if old != nil {
            old.Stop()
        }
    }()
    return nil
}

func (i *Indexer) registerCallbacks(c *rpc_client.RpcClient) {
    c.AddOnConnectionEstablishedCallback(func() {
        i.logger.Info("SDK connection established, signaling subscription restart")
        i.signalSubscriptionRestart()
    })
    c.AddOnConnectionLostCallback(func(err error) {
        i.logger.Warn("SDK connection lost, will auto-reconnect", zap.Error(err))
    })
}
```

Replace the existing callback registration in `Run()` with a single call:
```go
i.registerCallbacks(i.client())
```

- [ ] **Step 2: Implement `runSyncWatchdogLoop`**

```go
// internal/indexer/watchdog.go — append

import (
    "context"

    "github.com/0x3639/nom-indexer-go/internal/models"
)

type watchdogConfig struct {
    Interval              time.Duration
    StallThreshold        time.Duration
    IndexerDriftThreshold int64
    NodeDriftThreshold    int64
    UnhealthyStreak       int
    FailbackStreak        int
}

func (i *Indexer) runSyncWatchdogLoop(ctx context.Context, cfg watchdogConfig) {
    ticker := time.NewTicker(cfg.Interval)
    defer ticker.Stop()

    classifyCfg := classifyConfig{
        StallThreshold:        cfg.StallThreshold,
        IndexerDriftThreshold: cfg.IndexerDriftThreshold,
        NodeDriftThreshold:    cfg.NodeDriftThreshold,
    }
    reactCfg := watchdogReactConfig{
        UnhealthyStreak: cfg.UnhealthyStreak,
        FailbackStreak:  cfg.FailbackStreak,
    }

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
        }
        i.runWatchdogTick(ctx, classifyCfg, reactCfg)
    }
}

func (i *Indexer) runWatchdogTick(ctx context.Context, cCfg classifyConfig, rCfg watchdogReactConfig) {
    i.syncStateMu.RLock()
    activeIdx := i.syncStateInternal.activeIdx
    chainID := i.syncStateInternal.chainIdentifier
    i.syncStateMu.RUnlock()

    probe, probeErr := i.nodePool.Probe(ctx, activeIdx)

    dbHeight, err := i.repos.Momentum.GetLatestHeight(ctx)
    if err != nil {
        i.logger.Warn("watchdog: GetLatestHeight failed", zap.Error(err))
        return
    }

    lastProgress := time.Unix(i.lastProgressAt.Load(), 0)
    now := time.Now()
    class := classify(probe, probeErr, dbHeight, lastProgress, now, cCfg)

    i.syncStateMu.Lock()
    intent := react(i.syncStateInternal, activeIdx, class, rCfg)
    // Record chain id on first successful probe
    if probeErr == nil && i.syncStateInternal.chainIdentifier == "" {
        i.syncStateInternal.chainIdentifier = probe.GenesisHash
        chainID = probe.GenesisHash
    }
    i.syncStateMu.Unlock()

    // Failover on intent
    if intent.failoverIdx != -1 {
        target := selectFailoverTarget(ctx, i.nodePool, activeIdx, chainID, cCfg)
        if target == -1 {
            i.logger.Error("watchdog: no healthy fallback available")
        } else if err := i.swapActiveClient(i.nodePool.Entry(target).URL); err != nil {
            i.logger.Error("watchdog: swap failed", zap.Error(err))
        } else {
            i.syncStateMu.Lock()
            i.syncStateInternal.activeIdx = target
            now := time.Now().Unix()
            i.syncStateInternal.failedOverAt = &now
            // Reset all streaks
            for k := range i.syncStateInternal.streaks {
                i.syncStateInternal.streaks[k] = nodeStreaks{}
            }
            i.syncStateMu.Unlock()
            i.signalSubscriptionRestart()
        }
    }

    // Failback path
    if class == classSynced && activeIdx > 0 && intent.failoverIdx == -1 {
        for candidateIdx := 0; candidateIdx < activeIdx; candidateIdx++ {
            cProbe, err := i.nodePool.Probe(ctx, candidateIdx)
            if err != nil || cProbe.GenesisHash != chainID {
                i.syncStateMu.Lock()
                st := i.syncStateInternal.streaks[candidateIdx]
                st.healthy = 0
                i.syncStateInternal.streaks[candidateIdx] = st
                i.syncStateMu.Unlock()
                continue
            }
            i.syncStateMu.Lock()
            pickedIdx := selectFailback(i.syncStateInternal, candidateIdx, rCfg)
            i.syncStateMu.Unlock()
            if pickedIdx != -1 {
                if err := i.swapActiveClient(i.nodePool.Entry(pickedIdx).URL); err != nil {
                    i.logger.Error("watchdog: failback swap failed", zap.Error(err))
                    break
                }
                i.syncStateMu.Lock()
                i.syncStateInternal.activeIdx = pickedIdx
                i.syncStateInternal.failedOverAt = nil
                for k := range i.syncStateInternal.streaks {
                    i.syncStateInternal.streaks[k] = nodeStreaks{}
                }
                i.syncStateMu.Unlock()
                i.signalSubscriptionRestart()
                break
            }
        }
    }

    if intent.signalRestart {
        i.signalSubscriptionRestart()
    }

    // Publish state to indexer_sync_status
    i.publishSyncStatus(ctx, probe, probeErr, dbHeight, class, activeIdx, now)
}

func (i *Indexer) publishSyncStatus(
    ctx context.Context,
    probe ProbeResult,
    probeErr error,
    dbHeight int64,
    class syncClass,
    activeIdx int,
    now time.Time,
) {
    i.syncStateMu.RLock()
    st := i.syncStateInternal.streaks[activeIdx]
    chainID := i.syncStateInternal.chainIdentifier
    failedOverAt := i.syncStateInternal.failedOverAt
    i.syncStateMu.RUnlock()

    entry := i.nodePool.Entry(activeIdx)
    record := &models.SyncStatus{
        DBHeight:             dbHeight,
        ZnndFrontierHeight:   int64(probe.Frontier),
        ZnndTargetHeight:     int64(probe.Target),
        DriftMomentums:       int64(probe.Frontier) - dbHeight,
        NodeLagMomentums:     int64(probe.Target) - int64(probe.Frontier),
        State:                class.String(),
        ConsecutiveBadChecks: st.unhealthy,
        ActiveNodeURL:        entry.URL,
        ActiveNodeLabel:      entry.Label,
        ChainIdentifier:      chainID,
        FailedOverAt:         failedOverAt,
        LastProgressAt:       i.lastProgressAt.Load(),
        CheckedAt:            now.Unix(),
    }
    if err := i.repos.SyncStatus.Upsert(ctx, record); err != nil {
        i.logger.Warn("watchdog: upsert sync_status failed", zap.Error(err))
    }
    _ = probeErr // probe error is folded into State
}
```

- [ ] **Step 3: Launch the loop from `Run()`**

In `Indexer.Run()`, after the existing `wg.Add(3); go runBridgeSyncLoop/...; go runCachedDataSyncLoop/...; go runCronLoop/...`:

```go
// Launch watchdog only if a NodePool was supplied (NewIndexerWithCron
// path leaves it nil for back-compat with older callers/tests).
if i.nodePool != nil && i.cfg.Watchdog.Enabled {
    wg.Add(1)
    go func() {
        defer wg.Done()
        i.runSyncWatchdogLoop(ctx, watchdogConfig{
            Interval:              i.cfg.Watchdog.Interval,
            StallThreshold:        i.cfg.Watchdog.StallThreshold,
            IndexerDriftThreshold: i.cfg.Watchdog.IndexerDriftThreshold,
            NodeDriftThreshold:    i.cfg.Watchdog.NodeDriftThreshold,
            UnhealthyStreak:       i.cfg.Watchdog.UnhealthyStreak,
            FailbackStreak:        i.cfg.Watchdog.FailbackStreak,
        })
    }()
}
```

Note: this assumes the `Indexer` has access to `WatchdogConfig`. Extend the `CronConfig` (or add a new field) so `NewIndexerWithNodes` accepts it:

```go
type IndexerOptions struct {
    Cron     CronConfig
    Watchdog WatchdogConfigForIndexer
}

type WatchdogConfigForIndexer struct {
    Enabled               bool
    Interval              time.Duration
    StallThreshold        time.Duration
    IndexerDriftThreshold int64
    NodeDriftThreshold    int64
    UnhealthyStreak       int
    FailbackStreak        int
}
```

Then `i.cfg.Watchdog` is read via a stored copy on `Indexer`. Adjust `NewIndexerWithNodes` accordingly.

- [ ] **Step 4: Build and run all tests**

```bash
GOWORK=off go build ./...
GOWORK=off go test ./internal/indexer/...
```

Expected: build succeeds, existing unit tests still pass (integration tests deferred).

- [ ] **Step 5: Commit**

```bash
git add internal/indexer/
git commit -m "feat(indexer): launch sync watchdog goroutine with failover/failback"
```

---

## Task 14: Indexer-side `/healthz` + `/readyz` HTTP server

**Files:**
- Create: `internal/health/server.go`
- Create: `internal/health/server_test.go`

A tiny `net/http` server. Reads a `func() Snapshot` injected by `cmd/indexer/main.go`. No global state.

- [ ] **Step 1: Write the failing test**

```go
// internal/health/server_test.go
package health_test

import (
    "net/http"
    "net/http/httptest"
    "testing"

    "github.com/0x3639/nom-indexer-go/internal/health"
)

func TestHealthzAlwaysOK(t *testing.T) {
    srv := health.NewServer(func() health.Snapshot {
        return health.Snapshot{Ready: false, State: "stalled"}
    })
    rr := httptest.NewRecorder()
    srv.Handler.ServeHTTP(rr, httptest.NewRequest("GET", "/healthz", nil))
    if rr.Code != http.StatusOK {
        t.Fatalf("/healthz code = %d", rr.Code)
    }
}

func TestReadyzReady(t *testing.T) {
    srv := health.NewServer(func() health.Snapshot {
        return health.Snapshot{Ready: true, State: "synced", NodeLabel: "primary"}
    })
    rr := httptest.NewRecorder()
    srv.Handler.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
    if rr.Code != http.StatusOK {
        t.Fatalf("/readyz code = %d", rr.Code)
    }
}

func TestReadyzNotReady(t *testing.T) {
    srv := health.NewServer(func() health.Snapshot {
        return health.Snapshot{Ready: false, State: "node_lagging", NodeLabel: "primary"}
    })
    rr := httptest.NewRecorder()
    srv.Handler.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
    if rr.Code != http.StatusServiceUnavailable {
        t.Fatalf("/readyz code = %d", rr.Code)
    }
}
```

- [ ] **Step 2: Run, confirm fail**

```bash
GOWORK=off go test ./internal/health/
```

Expected: package not found.

- [ ] **Step 3: Implement**

```go
// internal/health/server.go
package health

import (
    "encoding/json"
    "fmt"
    "net/http"
)

type Snapshot struct {
    Ready     bool   `json:"-"`
    State     string `json:"state"`
    NodeLabel string `json:"node,omitempty"`
    Drift     int64  `json:"drift,omitempty"`
}

type Server struct {
    Handler http.Handler
    addr    string
}

func NewServer(snapshot func() Snapshot) *Server {
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
        writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
    })
    mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
        snap := snapshot()
        code := http.StatusOK
        body := map[string]any{
            "status": "ready",
            "state":  snap.State,
            "node":   snap.NodeLabel,
            "drift":  snap.Drift,
        }
        if !snap.Ready {
            code = http.StatusServiceUnavailable
            body["status"] = "draining"
        }
        writeJSON(w, code, body)
    })
    return &Server{Handler: mux}
}

func (s *Server) ListenAndServe(addr string) error {
    return (&http.Server{Addr: addr, Handler: s.Handler}).ListenAndServe()
}

func writeJSON(w http.ResponseWriter, code int, v any) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    if err := json.NewEncoder(w).Encode(v); err != nil {
        fmt.Fprintf(w, `{"error":%q}`, err.Error())
    }
}
```

- [ ] **Step 4: Run tests, confirm pass**

```bash
GOWORK=off go test ./internal/health/
```

Expected: all three PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/health/
git commit -m "feat(health): add tiny HTTP server for /healthz and /readyz"
```

---

## Task 15: Wire watchdog and health server in `cmd/indexer/main.go`

**Files:**
- Modify: `cmd/indexer/main.go`

- [ ] **Step 1: Build the NodePool from config**

After the existing config load and DB connect, replace the single-client construction:

```go
// REPLACED CODE — was:
//   client, err := rpc_client.NewRpcClient(cfg.Node.WebSocketURL)
//   ...
//   idx := indexer.NewIndexerWithCron(client, pool, logger, indexer.CronConfig{...})

if len(cfg.Indexer.Nodes) == 0 {
    logger.Fatal("no nodes configured (set NODE_URL_WS or indexer.nodes)")
}

primaryURL := cfg.Indexer.Nodes[0].URL
client, err := rpc_client.NewRpcClient(primaryURL)
if err != nil {
    logger.Fatal("failed to connect to primary node", zap.Error(err))
}
defer client.Stop()

nodePool := indexer.NewNodePool(toIndexerNodes(cfg.Indexer.Nodes), logger)

idx := indexer.NewIndexerWithNodes(pool, nodePool, client, logger,
    indexer.CronConfig{
        VotingActivityInterval: votingInterval,
        TokenHoldersInterval:   tokenHoldersInterval,
    },
    indexer.WatchdogConfigForIndexer{
        Enabled:               cfg.Indexer.Watchdog.Enabled,
        Interval:              cfg.Indexer.Watchdog.Interval,
        StallThreshold:        cfg.Indexer.Watchdog.StallThreshold,
        IndexerDriftThreshold: cfg.Indexer.Watchdog.IndexerDriftThreshold,
        NodeDriftThreshold:    cfg.Indexer.Watchdog.NodeDriftThreshold,
        UnhealthyStreak:       cfg.Indexer.Watchdog.UnhealthyStreak,
        FailbackStreak:        cfg.Indexer.Watchdog.FailbackStreak,
    },
)
```

Add the `toIndexerNodes` helper at the bottom of main.go:

```go
func toIndexerNodes(in []config.NodeEntry) []indexer.NodeEntry {
    out := make([]indexer.NodeEntry, len(in))
    for i, n := range in {
        out[i] = indexer.NodeEntry{URL: n.URL, Label: n.Label}
    }
    return out
}
```

- [ ] **Step 2: Run the synchronous startup probe before `idx.Run(ctx)`**

Per the spec's Startup section, the indexer probes its primary node
*before* the existing `sync()` catch-up runs. If the primary is dead but
a fallback works, we swap to the fallback before any momentums get
fetched.

```go
// cmd/indexer/main.go — between NewIndexerWithNodes(...) and idx.Run(ctx)

if cfg.Indexer.Watchdog.Enabled {
    probe, err := nodePool.Probe(ctx, 0)
    if err != nil {
        logger.Warn("startup probe of primary failed, trying fallbacks", zap.Error(err))
        swapped := false
        for i := 1; i < nodePool.Len(); i++ {
            if _, err := nodePool.Probe(ctx, i); err == nil {
                if err := idx.StartupSwap(i); err != nil {
                    logger.Error("startup swap failed", zap.Int("idx", i), zap.Error(err))
                    continue
                }
                logger.Info("started on fallback node",
                    zap.String("label", nodePool.Entry(i).Label))
                swapped = true
                break
            }
        }
        if !swapped {
            logger.Fatal("all nodes failed startup probe")
        }
    } else {
        logger.Info("startup probe ok",
            zap.Uint64("frontier", probe.Frontier),
            zap.String("genesis", probe.GenesisHash))
        // First-run chain identity capture: handled lazily by the
        // first watchdog tick after sync() completes.
    }
}
```

Add `StartupSwap(idx int)` to `Indexer` as a tiny wrapper around the
existing `swapActiveClient` that also updates `syncStateInternal.activeIdx`:

```go
// internal/indexer/indexer.go — append

func (i *Indexer) StartupSwap(idx int) error {
    if err := i.swapActiveClient(i.nodePool.Entry(idx).URL); err != nil {
        return err
    }
    i.syncStateMu.Lock()
    i.syncStateInternal.activeIdx = idx
    now := time.Now().Unix()
    i.syncStateInternal.failedOverAt = &now
    i.syncStateMu.Unlock()
    return nil
}
```

- [ ] **Step 3: Launch the health server**

After indexer construction, before `idx.Run(ctx)`:

```go
if cfg.Indexer.Health.Enabled {
    healthSrv := health.NewServer(func() health.Snapshot {
        return idx.HealthSnapshot()
    })
    go func() {
        addr := fmt.Sprintf(":%d", cfg.Indexer.Health.Port)
        logger.Info("starting health server", zap.String("addr", addr))
        if err := healthSrv.ListenAndServe(addr); err != nil && err != http.ErrServerClosed {
            logger.Error("health server crashed", zap.Error(err))
        }
    }()
}
```

Then add `HealthSnapshot()` to `Indexer`:

```go
// internal/indexer/indexer.go — append

import "github.com/0x3639/nom-indexer-go/internal/health"

func (i *Indexer) HealthSnapshot() health.Snapshot {
    i.syncStateMu.RLock()
    defer i.syncStateMu.RUnlock()
    if i.syncStateInternal == nil {
        return health.Snapshot{Ready: true, State: "uninitialised"}
    }
    activeIdx := i.syncStateInternal.activeIdx
    state := "synced"
    streak := i.syncStateInternal.streaks[activeIdx].unhealthy
    if streak > 0 {
        state = "draining"
    }
    return health.Snapshot{
        Ready:     streak < 2, // matches default UnhealthyStreak
        State:     state,
        NodeLabel: i.nodePool.Entry(activeIdx).Label,
    }
}
```

(Pass `UnhealthyStreak` into `Indexer` so the snapshot doesn't hard-code `2`. Add it as a field set in `NewIndexerWithNodes`.)

- [ ] **Step 4: Build the binary**

```bash
GOWORK=off go build ./cmd/indexer
```

Expected: compiles cleanly.

- [ ] **Step 5: Run the indexer in the existing compose stack and sanity-check `/readyz`**

```bash
docker compose --profile local-node up -d --build indexer
# Wait ~5s for the new health server to come up
curl -i http://localhost:9092/healthz
curl -i http://localhost:9092/readyz
```

Expected: both return 200 with JSON bodies. (Port 9092 is internal-only in compose; you need to add `ports: ["9092:9092"]` temporarily for this manual check OR `docker exec` curl from inside the indexer container.)

- [ ] **Step 6: Commit**

```bash
git add cmd/indexer/main.go internal/indexer/indexer.go
git commit -m "feat(indexer): wire NodePool, watchdog, and health server in main"
```

---

## Task 16: Extend API `/readyz` + bump `minSchemaVersion`

**Files:**
- Modify: `internal/api/router/router.go`
- Modify: `internal/api/router/router_test.go`

- [ ] **Step 1: Write the failing tests**

```go
// internal/api/router/router_test.go — append

func TestReadyzDegradedOnIndexerStall(t *testing.T) {
    // pre-seed sync_status row with state=stalled and consecutive_bad_checks=3
    pool := newTestPool(t)
    _, err := pool.Exec(context.Background(),
        `INSERT INTO indexer_sync_status (id, db_height, znnd_frontier_height,
         znnd_target_height, drift_momentums, node_lag_momentums, state,
         consecutive_bad_checks, active_node_url, active_node_label,
         chain_identifier, last_progress_at, checked_at)
         VALUES (1, 100, 100, 100, 0, 0, 'stalled', 3, 'ws://x', 'x', 'g', 0, 0)`)
    if err != nil { t.Fatal(err) }

    r := newTestRouter(t, pool) // helper existing in router tests
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
    if rr.Code != http.StatusServiceUnavailable {
        t.Fatalf("/readyz code = %d, want 503", rr.Code)
    }
}

func TestReadyzReadyOnNoSyncStatusYet(t *testing.T) {
    pool := newTestPool(t)
    // No insert; sync_status table is empty.

    r := newTestRouter(t, pool)
    rr := httptest.NewRecorder()
    r.ServeHTTP(rr, httptest.NewRequest("GET", "/readyz", nil))
    if rr.Code != http.StatusOK {
        t.Fatalf("/readyz code = %d, want 200 on empty row", rr.Code)
    }
}
```

- [ ] **Step 2: Bump `minSchemaVersion` and extend the readyz handler**

In `internal/api/router/router.go`:

```go
const minSchemaVersion = 13 // bumped from 12 — adds indexer_sync_status table

// Inside the existing readyz handler, AFTER the DB ping + schema version check:
sync, err := d.Repos.SyncStatus.Get(r.Context())
if err != nil {
    if errors.Is(err, pgx.ErrNoRows) {
        // First run before watchdog wrote anything — don't fail.
        httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
        return
    }
    httpx.WriteJSON(w, http.StatusServiceUnavailable,
        map[string]string{"status": "degraded", "reason": "sync_status_unreadable"})
    return
}
const unhealthyStreakForReady = 2 // matches indexer default
if sync.State != "synced" && sync.ConsecutiveBadChecks >= unhealthyStreakForReady {
    httpx.WriteJSON(w, http.StatusServiceUnavailable, map[string]any{
        "status": "degraded", "state": sync.State,
        "drift": sync.DriftMomentums, "node": sync.ActiveNodeLabel,
    })
    return
}
httpx.WriteJSON(w, http.StatusOK, map[string]any{
    "status": "ok", "node": sync.ActiveNodeLabel, "drift": sync.DriftMomentums,
})
```

- [ ] **Step 3: Run tests**

```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/api/router/ -run TestReadyz
```

Expected: PASS.

- [ ] **Step 4: Commit**

```bash
git add internal/api/router/
git commit -m "feat(api): readyz reflects indexer_sync_status and bumps minSchema to 13"
```

---

## Task 17: docker-compose + example config

**Files:**
- Modify: `docker-compose.yml`
- Modify: `config.yaml.example`

- [ ] **Step 1: Add indexer healthcheck**

In `docker-compose.yml` under the existing `indexer:` service, add:

```yaml
    expose:
      - "9092"
    healthcheck:
      test:
        - CMD-SHELL
        - wget -q -O- "http://localhost:9092/healthz" | grep -q ok || exit 1
      interval: 10s
      timeout: 3s
      retries: 3
    environment:
      # ... existing ...
      INDEXER_HEALTH_PORT: ${INDEXER_HEALTH_PORT:-9092}
      INDEXER_WATCHDOG_ENABLED: ${INDEXER_WATCHDOG_ENABLED:-false}
      NODE_URL_FALLBACKS: ${NODE_URL_FALLBACKS:-}
```

Note: the indexer image is currently based on alpine and includes wget. If it doesn't, swap for `nc -z localhost 9092` or include `wget` in `Dockerfile`.

- [ ] **Step 2: Document config keys**

In `config.yaml.example`, add:

```yaml
indexer:
  # Ordered priority: nodes[0] is preferred; others are fallbacks.
  # Either WS or HTTP URLs are accepted; nodes[0] should be WS-capable
  # because the subscription requires it.
  nodes:
    - url: ws://znnd:35998
      label: local-znnd
    # - url: wss://my.hc1node.com:35998
    #   label: hc1node-remote
  watchdog:
    enabled: false               # default OFF; opt-in per rollout
    interval: 30s
    stall_threshold: 60s
    indexer_drift_threshold: 3
    node_drift_threshold: 3
    unhealthy_streak: 2
    failback_streak: 5
    tolerate_missing_syncinfo: true
  health:
    enabled: true
    port: 9092
```

- [ ] **Step 3: Verify the indexer container comes up healthy**

```bash
docker compose --profile local-node up -d --build indexer
docker inspect nom-indexer --format='{{json .State.Health}}' | python3 -m json.tool
```

Expected: `"Status": "healthy"` within ~30s.

- [ ] **Step 4: Commit**

```bash
git add docker-compose.yml config.yaml.example
git commit -m "chore(compose): wire indexer healthcheck and watchdog env"
```

---

## Task 18: Integration tests for the watchdog

**Files:**
- Create: `internal/indexer/watchdog_integration_test.go`

Two end-to-end scenarios. Both spin up fake JSON-RPC servers with `fakeJSONRPC`, point the indexer at them, and assert behavior. Both require Postgres.

- [ ] **Step 1: Write the failing tests**

```go
//go:build integration

// internal/indexer/watchdog_integration_test.go
package indexer_test

import (
    "context"
    "encoding/json"
    "net/http"
    "net/http/httptest"
    "sync/atomic"
    "testing"
    "time"

    "github.com/0x3639/znn-sdk-go/rpc_client"
    "go.uber.org/zap"

    "github.com/0x3639/nom-indexer-go/internal/indexer"
)

// fakeNode is a JSON-RPC server whose syncInfo/frontier responses can
// be toggled at runtime via atomics.
type fakeNode struct {
    URL       string
    srv       *httptest.Server
    frontier  atomic.Uint64
    target    atomic.Uint64
    genesis   string
}

func newFakeNode(t *testing.T, frontier, target uint64, genesis string) *fakeNode {
    t.Helper()
    f := &fakeNode{genesis: genesis}
    f.frontier.Store(frontier)
    f.target.Store(target)
    f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        var req struct{ Method string `json:"method"` }
        _ = json.NewDecoder(r.Body).Decode(&req)
        out := map[string]any{"jsonrpc": "2.0", "id": 1}
        switch req.Method {
        case "stats.syncInfo":
            state := 1
            if f.frontier.Load() == f.target.Load() { state = 2 }
            out["result"] = map[string]any{
                "state": state,
                "currentHeight": f.frontier.Load(),
                "targetHeight":  f.target.Load(),
            }
        case "ledger.getFrontierMomentum":
            out["result"] = map[string]any{"height": f.frontier.Load()}
        case "ledger.getMomentumsByHeight":
            out["result"] = map[string]any{
                "count": 1,
                "list":  []any{map[string]any{"height": 1, "hash": f.genesis}},
            }
        default:
            out["error"] = map[string]any{"code": -32601, "message": "method not found"}
        }
        _ = json.NewEncoder(w).Encode(out)
    }))
    f.URL = f.srv.URL
    t.Cleanup(f.srv.Close)
    return f
}

func TestWatchdogFailoverOnNodeLag(t *testing.T) {
    // Primary reports target=1000 ahead of frontier=100 → node_lagging.
    primary  := newFakeNode(t, 100, 1000, "G")
    fallback := newFakeNode(t, 100, 100,  "G")

    pool := newTestPool(t) // existing helper
    nodePool := indexer.NewNodePool([]indexer.NodeEntry{
        {URL: primary.URL,  Label: "primary"},
        {URL: fallback.URL, Label: "fallback"},
    }, zap.NewNop())

    client, err := rpc_client.NewRpcClient(primary.URL)
    if err != nil { t.Fatalf("NewRpcClient: %v", err) }
    defer client.Stop()

    idx := indexer.NewIndexerWithNodes(pool, nodePool, client, zap.NewNop(),
        indexer.CronConfig{},
        indexer.WatchdogConfigForIndexer{
            Enabled:               true,
            Interval:              200 * time.Millisecond,
            StallThreshold:        10 * time.Second,
            IndexerDriftThreshold: 3,
            NodeDriftThreshold:    3,
            UnhealthyStreak:       2,
            FailbackStreak:        5,
        },
    )

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go func() { _ = idx.Run(ctx) }()

    // 4 ticks should be enough (2 to trip unhealthy_streak + 1 for swap + buffer).
    deadline := time.After(2 * time.Second)
    for {
        select {
        case <-deadline:
            t.Fatal("timed out waiting for failover")
        case <-time.After(100 * time.Millisecond):
        }
        snap := idx.HealthSnapshot()
        if snap.NodeLabel == "fallback" {
            return // success
        }
    }
}

func TestWatchdogFailbackAfterPrimaryRecovers(t *testing.T) {
    primary  := newFakeNode(t, 100, 1000, "G") // starts node_lagging
    fallback := newFakeNode(t, 100, 100,  "G")

    pool := newTestPool(t)
    nodePool := indexer.NewNodePool([]indexer.NodeEntry{
        {URL: primary.URL,  Label: "primary"},
        {URL: fallback.URL, Label: "fallback"},
    }, zap.NewNop())

    client, err := rpc_client.NewRpcClient(primary.URL)
    if err != nil { t.Fatalf("NewRpcClient: %v", err) }
    defer client.Stop()

    idx := indexer.NewIndexerWithNodes(pool, nodePool, client, zap.NewNop(),
        indexer.CronConfig{},
        indexer.WatchdogConfigForIndexer{
            Enabled: true, Interval: 100 * time.Millisecond,
            StallThreshold: 10 * time.Second,
            IndexerDriftThreshold: 3, NodeDriftThreshold: 3,
            UnhealthyStreak: 2, FailbackStreak: 3,
        },
    )

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    go func() { _ = idx.Run(ctx) }()

    // Wait for initial failover to fallback.
    waitFor(t, 2*time.Second, func() bool {
        return idx.HealthSnapshot().NodeLabel == "fallback"
    })

    // Now restore the primary.
    primary.target.Store(100)

    // Wait for failback to primary.
    waitFor(t, 3*time.Second, func() bool {
        return idx.HealthSnapshot().NodeLabel == "primary"
    })
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
    t.Helper()
    deadline := time.Now().Add(timeout)
    for time.Now().Before(deadline) {
        if cond() { return }
        time.Sleep(50 * time.Millisecond)
    }
    t.Fatalf("condition not met within %v", timeout)
}
```

These tests assume `Indexer.Run(ctx)` returns once `ctx` is cancelled and that `HealthSnapshot()` is safe to call from another goroutine while the watchdog is ticking — both true given the implementation in earlier tasks. `newTestPool` is the existing repository-test helper (already used in `internal/repository/sync_status_test.go`).

- [ ] **Step 2: Run the tests**

```bash
TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
  GOWORK=off go test -tags integration ./internal/indexer/ -run TestWatchdog
```

Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add internal/indexer/watchdog_integration_test.go
git commit -m "test(indexer): integration tests for failover/failback"
```

---

## Task 19: Documentation updates

**Files:**
- Modify: `docs/architecture/overview.md`
- Create or modify: `docs/operations/watchdog.md`

- [ ] **Step 1: Add a watchdog row to the lane table in `docs/architecture/overview.md`**

The existing table at line ~43 has 5 rows. Add a sixth:

```markdown
| 6 | Sync watchdog | 30s | Drift detection (indexer-vs-znnd, znnd-vs-chain), automatic resubscribe on drift, node failover/failback when configured. Writes `indexer_sync_status` row. |
```

Also add the watchdog goroutine to the WaitGroup note: `A WaitGroup in Indexer.Run makes shutdown wait for the four managed goroutines (bridge, cached data, cron, watchdog) before returning.`

- [ ] **Step 2: Create `docs/operations/watchdog.md`**

```markdown
---
title: Sync watchdog
---

# Sync watchdog

The watchdog detects two failure modes and reacts in-process so the
container doesn't need to restart:

1. **Indexer behind znnd.** Triggers an internal subscription restart
   (the same path the SDK reconnect callback uses) which re-runs the
   batched catch-up.
2. **znnd behind chain** (and/or fully unresponsive). Triggers a
   failover to the next configured node.

## Configuration

See `config.yaml.example` for the full list. Minimum to enable:

```yaml
indexer:
  watchdog:
    enabled: true
  nodes:
    - url: ws://znnd:35998
      label: local-znnd
    - url: wss://my.hc1node.com:35998
      label: hc1node-remote
```

Or via env: set `INDEXER_WATCHDOG_ENABLED=true` and
`NODE_URL_FALLBACKS=wss://my.hc1node.com:35998,...`.

## Health endpoints

- `:9092/healthz` — process alive (always 200).
- `:9092/readyz` — 503 when the watchdog reports `state != synced` for
  more than `unhealthy_streak` consecutive ticks.

The API's `/readyz` also joins `indexer_sync_status` so external
monitoring of API health surfaces indexer-side degradation.

## Observing state

```sql
SELECT state, drift_momentums, node_lag_momentums,
       active_node_label, consecutive_bad_checks,
       to_timestamp(checked_at) AS checked
  FROM indexer_sync_status;
```
```

- [ ] **Step 3: Commit**

```bash
git add docs/
git commit -m "docs: add watchdog operations guide and update architecture lane table"
```

---

## Task 20: Manual smoke verification

**Files:** none

- [ ] **Step 1: Bring up the stack with watchdog enabled**

```bash
# In .env add:
#   INDEXER_WATCHDOG_ENABLED=true
#   NODE_URL_FALLBACKS=wss://my.hc1node.com:35998
docker compose --profile local-node up -d --build
```

- [ ] **Step 2: Confirm baseline health**

```bash
curl -s http://localhost:8080/readyz | python3 -m json.tool
docker exec nom-indexer wget -q -O- http://localhost:9092/readyz
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c 'SELECT state, drift_momentums, active_node_label FROM indexer_sync_status;'
```

Expected: `state=synced`, `active_node_label=local-znnd`, drift ~ 0.

- [ ] **Step 3: Simulate local znnd failure**

```bash
docker stop nom-indexer-znnd
# Wait ~90s (2 watchdog ticks at 30s + swap latency)
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c 'SELECT state, active_node_label, failed_over_at FROM indexer_sync_status;'
```

Expected: `active_node_label=fallback-1` (or whatever label was set),
`failed_over_at` non-null. `/readyz` still 200 (the indexer is healthy,
just on the fallback).

- [ ] **Step 4: Restore znnd and confirm failback**

```bash
docker start nom-indexer-znnd
# Wait ~3 minutes (failback_streak=5 ticks × 30s)
docker exec nom-indexer-postgres psql -U postgres -d nom_indexer \
  -c 'SELECT state, active_node_label, failed_over_at FROM indexer_sync_status;'
```

Expected: `active_node_label=local-znnd`, `failed_over_at` NULL.

- [ ] **Step 5: Final commit and summary**

```bash
git log --oneline feat/continuous-sync-watchdog ^main
```

Verify the commit history is clean (one logical commit per task).

Branch is ready for review. Do NOT push or open a PR until the user explicitly asks — per project workflow, Codex review is the QA gate run via the user's tooling.

---

## Self-review notes (for the implementing engineer)

Quick sanity checks before declaring done:

- [ ] `indexer_sync_status` row has a single non-null `chain_identifier` after one tick. If it stays empty after multiple ticks, `genesisFor` is failing silently — add a `Warn` log.
- [ ] `lastProgressAt` is being updated. Check `SELECT last_progress_at FROM indexer_sync_status;` — it should match recent commit times.
- [ ] Watchdog goroutine doesn't leak on shutdown — add a `t.Run("shutdown")` that cancels ctx and verifies the goroutine returns within 1s.
- [ ] After a failover, the next sync_status row should show streak counters reset to zero.
- [ ] `minSchemaVersion` is 13, not 12 — `/readyz` against an under-migrated DB must 503.
