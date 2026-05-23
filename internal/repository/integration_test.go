//go:build integration

// Repository integration tests exercise SQL end-to-end against a real Postgres.
// Gated by the `integration` build tag and TEST_DATABASE_URL.
//
// Run with:
//
//	TEST_DATABASE_URL='postgres://postgres:nomIndexerPass123@localhost:5432/nom_indexer_test?sslmode=disable' \
//	    go test -tags integration ./internal/repository/...
package repository

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// testPool is shared across all integration tests in this file. Initialized in
// TestMain to run migrations exactly once and avoid the migrate library's
// advisory-lock contention that arises when each test creates its own pool.
var testPool *pgxpool.Pool

func TestMain(m *testing.M) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		// Allow `go test -tags integration ./...` to no-op for callers without
		// a configured DB; individual tests still skip via newTestDB().
		os.Exit(m.Run())
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		panic("connect: " + err.Error())
	}
	defer pool.Close()

	migrationsPath := os.Getenv("TEST_MIGRATIONS_PATH")
	if migrationsPath == "" {
		wd, _ := os.Getwd()
		migrationsPath = filepath.Join(wd, "..", "..", "migrations")
	}

	// Use a stdlib DB just for migrate, then close it before tests run so
	// migrate's advisory lock is fully released and connections returned.
	db := stdlib.OpenDBFromPool(pool)
	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		panic("migrate driver: " + err.Error())
	}
	migrator, err := migrate.NewWithDatabaseInstance("file://"+migrationsPath, "postgres", driver)
	if err != nil {
		panic("migrate instance: " + err.Error())
	}
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		panic("migrate up: " + err.Error())
	}
	// Close migrator (releases the source); the underlying db wraps the pool
	// and shares its lifecycle so we leave it alone.
	srcErr, dbErr := migrator.Close()
	if srcErr != nil {
		panic("migrate close source: " + srcErr.Error())
	}
	if dbErr != nil {
		panic("migrate close db: " + dbErr.Error())
	}

	testPool = pool
	os.Exit(m.Run())
}

// newTestDB returns the shared pool, truncating all tables first so each test
// starts from a clean slate without re-running migrations.
func newTestDB(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testPool == nil {
		t.Skip("TEST_DATABASE_URL not set; skipping repository integration tests")
	}
	ctx := context.Background()
	_, err := testPool.Exec(ctx, `
		TRUNCATE momentums, accounts, balances, account_blocks, tokens,
		pillars, pillar_updates, sentinels, stakes, projects, project_phases,
		votes, fusions, cumulative_rewards, reward_transactions,
		wrap_token_requests, unwrap_token_requests,
		token_mints, token_burns,
		bridge_networks, bridge_network_tokens, bridge_admin, bridge_guardians,
		bridge_orchestrator_info, bridge_security_info,
		delegations,
		network_stat_histories, token_stat_histories, pillar_stat_histories,
		bridge_stat_histories
		RESTART IDENTITY`)
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return testPool
}

func TestIntegration_Momentum_InsertAndGet(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewMomentumRepository(pool)

	m := &models.Momentum{
		Height:        42,
		Hash:          "0xdeadbeef",
		Timestamp:     1700000000,
		TxCount:       3,
		Producer:      "z1qproducer",
		ProducerOwner: "z1qowner",
		ProducerName:  "alphanet-1",
	}
	if err := repo.Insert(ctx, m); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := repo.GetByHeight(ctx, 42)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Hash != m.Hash || got.TxCount != m.TxCount || got.ProducerName != m.ProducerName {
		t.Errorf("round-trip mismatch: got %+v want %+v", got, m)
	}

	h, err := repo.GetLatestHeight(ctx)
	if err != nil {
		t.Fatalf("latest height: %v", err)
	}
	if h != 42 {
		t.Errorf("latest height = %d, want 42", h)
	}

	if err := repo.Insert(ctx, m); err != nil {
		t.Errorf("re-insert should be idempotent: %v", err)
	}
}

func TestIntegration_Momentum_GetLatestHeight_Empty(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewMomentumRepository(pool)

	h, err := repo.GetLatestHeight(ctx)
	if err != nil {
		t.Fatalf("latest height on empty table: %v", err)
	}
	if h != 0 {
		t.Errorf("expected 0 for empty table, got %d", h)
	}
}

func TestIntegration_AccountBlock_InsertAndUpdate(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewAccountBlockRepository(pool)

	ab := &models.AccountBlock{
		Hash:               "0xblock1",
		MomentumHash:       "0xmom1",
		MomentumTimestamp:  1700000000,
		MomentumHeight:     10,
		BlockType:          2,
		Height:             1,
		Address:            "z1qsender",
		ToAddress:          "z1qrecipient",
		Amount:             1000,
		TokenStandard:      models.ZnnTokenStandard,
		Data:               "",
		PairedAccountBlock: "",
	}
	td := &models.TxData{Method: "Send", Inputs: map[string]string{"k": "v"}}

	if err := repo.Insert(ctx, ab, td); err != nil {
		t.Fatalf("insert: %v", err)
	}

	got, err := repo.GetByHash(ctx, "0xblock1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Method != "Send" {
		t.Errorf("Method = %q, want Send", got.Method)
	}
	if !strings.Contains(string(got.Input), `"k": "v"`) && !strings.Contains(string(got.Input), `"k":"v"`) {
		t.Errorf(`expected Input JSON to contain "k":"v", got %s`, got.Input)
	}

	if err := repo.UpdatePairedBlock(ctx, "0xblock1", "0xpaired"); err != nil {
		t.Fatalf("update paired: %v", err)
	}
	got, _ = repo.GetByHash(ctx, "0xblock1")
	if got.PairedAccountBlock != "0xpaired" {
		t.Errorf("paired = %q, want 0xpaired", got.PairedAccountBlock)
	}

	if err := repo.UpdateDescendantOf(ctx, "0xblock1", "0xparent"); err != nil {
		t.Fatalf("update descendant: %v", err)
	}
	got, _ = repo.GetByHash(ctx, "0xblock1")
	if got.DescendantOf != "0xparent" {
		t.Errorf("descendant_of = %q, want 0xparent", got.DescendantOf)
	}
}

func TestIntegration_AccountBlock_NullByteSanitization(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewAccountBlockRepository(pool)

	td := &models.TxData{
		Method: "Test",
		Inputs: map[string]string{"data": "before\x00after"},
	}
	ab := &models.AccountBlock{Hash: "0xnull", BlockType: 1, Height: 1, Address: "z1qx"}

	if err := repo.Insert(ctx, ab, td); err != nil {
		t.Fatalf("insert with NUL byte should succeed: %v", err)
	}

	got, err := repo.GetByHash(ctx, "0xnull")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if strings.Contains(string(got.Input), "\x00") {
		t.Errorf("stored JSON contains a NUL byte: %q", got.Input)
	}
}

func TestIntegration_Pillar_UpsertAndRevoke(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewPillarRepository(pool)

	p := &models.Pillar{
		OwnerAddress:    "z1qowner",
		ProducerAddress: "z1qproducer",
		WithdrawAddress: "z1qwithdraw",
		Name:            "alphanet-1",
		Rank:            5,
	}
	if err := repo.Upsert(ctx, p); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := repo.GetByName(ctx, "alphanet-1")
	if err != nil {
		t.Fatalf("get by name: %v", err)
	}
	if got.Rank != 5 {
		t.Errorf("rank = %d, want 5", got.Rank)
	}
	if got.IsRevoked {
		t.Error("should not be revoked")
	}

	for i := 0; i < 3; i++ {
		if err := repo.IncrementMomentumCount(ctx, "z1qowner"); err != nil {
			t.Fatalf("inc: %v", err)
		}
	}
	got, _ = repo.GetByName(ctx, "alphanet-1")
	if got.ProducedMomentumCount != 3 {
		t.Errorf("produced count = %d, want 3", got.ProducedMomentumCount)
	}

	if err := repo.SetAsRevoked(ctx, "z1qowner", "alphanet-1", 1700000999); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	got, _ = repo.GetByName(ctx, "alphanet-1")
	if !got.IsRevoked {
		t.Error("expected is_revoked=true")
	}
	if got.RevokeTimestamp != 1700000999 {
		t.Errorf("revoke ts = %d", got.RevokeTimestamp)
	}
	// Historical fields must be preserved so queries against revoked pillars
	// still have meaningful data (producer/withdraw addresses, rank, etc.).
	if got.ProducerAddress != "z1qproducer" {
		t.Errorf("producer wiped on revoke: %q", got.ProducerAddress)
	}
	if got.WithdrawAddress != "z1qwithdraw" {
		t.Errorf("withdraw wiped on revoke: %q", got.WithdrawAddress)
	}
	if got.Rank != 5 {
		t.Errorf("rank wiped on revoke: %d", got.Rank)
	}
	if got.ProducedMomentumCount != 3 {
		t.Errorf("produced count wiped on revoke: %d", got.ProducedMomentumCount)
	}
}

func TestIntegration_Vote_UpsertDedupesPerVoterAndVotingID(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewVoteRepository(pool)

	v1 := &models.Vote{
		MomentumHash:      "0xm1",
		MomentumTimestamp: 1000,
		MomentumHeight:    1,
		VoterAddress:      "z1qvoter",
		ProjectID:         "0xproj",
		PhaseID:           "",
		VotingID:          "0xvoting",
		Vote:              0,
	}
	if err := repo.Insert(ctx, v1); err != nil {
		t.Fatalf("first insert: %v", err)
	}

	v2 := *v1
	v2.MomentumHeight = 2
	v2.MomentumHash = "0xm2"
	v2.Vote = 1
	if err := repo.Insert(ctx, &v2); err != nil {
		t.Fatalf("second insert: %v", err)
	}

	var n int
	var lastVote int16
	var lastHeight int64
	if err := pool.QueryRow(ctx, `
		SELECT COUNT(*), MAX(vote)::smallint, MAX(momentum_height)
		FROM votes WHERE voter_address=$1 AND voting_id=$2`,
		"z1qvoter", "0xvoting").Scan(&n, &lastVote, &lastHeight); err != nil {
		t.Fatalf("query: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 row after dedup, got %d", n)
	}
	if lastVote != 1 || lastHeight != 2 {
		t.Errorf("expected latest vote=1 height=2, got vote=%d height=%d", lastVote, lastHeight)
	}
}

func TestIntegration_Bridge_FinalityHelpers(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewBridgeRepository(pool)

	w := &models.WrapTokenRequest{
		ID:                      "0xwrap1",
		NetworkClass:            2,
		ChainID:                 1,
		ToAddress:               "0xeth",
		TokenStandard:           models.ZnnTokenStandard,
		TokenAddress:            "0xtoken",
		Amount:                  100,
		Fee:                     1,
		Signature:               "sig",
		CreationMomentumHeight:  500,
		ConfirmationsToFinality: 3,
	}
	if err := repo.UpsertWrapRequest(ctx, w); err != nil {
		t.Fatalf("upsert wrap: %v", err)
	}

	exists, finalized, err := repo.IsWrapRequestFinalized(ctx, "0xwrap1")
	if err != nil || !exists || finalized {
		t.Errorf("expected exists=true finalized=false, got exists=%v finalized=%v err=%v", exists, finalized, err)
	}

	w.ConfirmationsToFinality = 0
	if err := repo.UpsertWrapRequest(ctx, w); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	exists, finalized, _ = repo.IsWrapRequestFinalized(ctx, "0xwrap1")
	if !exists || !finalized {
		t.Errorf("expected finalized after upsert, got exists=%v finalized=%v", exists, finalized)
	}

	exists, _, _ = repo.IsWrapRequestFinalized(ctx, "0xmissing")
	if exists {
		t.Errorf("expected exists=false for missing record")
	}

	stop, err := repo.GetWrapSyncStopHeight(ctx)
	if err != nil {
		t.Fatalf("stop height: %v", err)
	}
	if stop != 500 {
		t.Errorf("stop height = %d, want 500", stop)
	}
}

func TestIntegration_Reward_CumulativeRewardsAccumulates(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewRewardRepository(pool)

	for _, amt := range []int64{100, 250, 50} {
		if err := repo.UpdateCumulativeRewards(ctx, "z1qaddr", models.RewardTypePillar, amt, models.ZnnTokenStandard); err != nil {
			t.Fatalf("update: %v", err)
		}
	}

	var got int64
	err := pool.QueryRow(ctx, `
		SELECT amount FROM cumulative_rewards
		WHERE address=$1 AND reward_type=$2 AND token_standard=$3`,
		"z1qaddr", int(models.RewardTypePillar), models.ZnnTokenStandard).Scan(&got)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != 400 {
		t.Errorf("cumulative = %d, want 400", got)
	}
}

func TestIntegration_Account_UpdateDelegate(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(pool)

	a := &models.Account{Address: "z1qaddr", BlockCount: 1, PublicKey: "abcdef"}
	if err := repo.Upsert(ctx, a); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	if err := repo.Upsert(ctx, &models.Account{Address: "z1qaddr", BlockCount: 2, PublicKey: ""}); err != nil {
		t.Fatalf("upsert empty pk: %v", err)
	}
	got, err := repo.GetByAddress(ctx, "z1qaddr")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.PublicKey != "abcdef" {
		t.Errorf("public_key wiped on empty upsert: %q", got.PublicKey)
	}
	if got.BlockCount != 2 {
		t.Errorf("block_count = %d, want 2", got.BlockCount)
	}

	if err := repo.UpdateDelegate(ctx, "z1qaddr", "z1qpillar", 1700000000); err != nil {
		t.Fatalf("update delegate: %v", err)
	}
	got, _ = repo.GetByAddress(ctx, "z1qaddr")
	if got.Delegate != "z1qpillar" || got.DelegationStartTimestamp != 1700000000 {
		t.Errorf("delegate not updated: %+v", got)
	}
}

func TestIntegration_PgxBatch_RollbackOnFailure(t *testing.T) {
	// Sanity check that processMomentum's transactional batch wrapping works:
	// if any batched op fails, the whole momentum's writes roll back.
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewMomentumRepository(pool)

	if err := repo.Insert(ctx, &models.Momentum{Height: 1, Hash: "h", Producer: "p"}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	batch := &pgx.Batch{}
	batch.Queue(`INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		VALUES ($1, $2, 0, 0, $3, '', '')`, int64(2), "h2", "p")
	// Force duplicate-PK error by omitting ON CONFLICT.
	batch.Queue(`INSERT INTO momentums (height, hash, timestamp, tx_count, producer, producer_owner, producer_name)
		VALUES ($1, $2, 0, 0, $3, '', '')`, int64(1), "dup", "p")

	br := tx.SendBatch(ctx, batch)
	var sawErr bool
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			sawErr = true
		}
	}
	_ = br.Close()
	if !sawErr {
		t.Fatal("expected at least one Exec error from duplicate PK")
	}
	_ = tx.Rollback(ctx)

	if _, err := repo.GetByHeight(ctx, 2); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("expected ErrNoRows for height=2 after rollback, got %v", err)
	}
}
