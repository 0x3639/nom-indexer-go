//go:build integration

package repository

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// sendBatch sends a pgx.Batch on the pool and drains all results, failing the
// test on any error.
func sendBatch(t *testing.T, ctx context.Context, pool batchSender, batch *pgx.Batch) {
	t.Helper()
	br := pool.SendBatch(ctx, batch)
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			t.Fatalf("batch op %d: %v", i, err)
		}
	}
	if err := br.Close(); err != nil {
		t.Fatalf("close batch: %v", err)
	}
}

// batchSender is the subset of pgxpool.Pool we need for sendBatch.
type batchSender interface {
	SendBatch(ctx context.Context, b *pgx.Batch) pgx.BatchResults
}

func TestIntegration_TokenEvents_MintBurnRoundTrip(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewTokenEventRepository(pool)

	mint := &models.TokenMint{
		AccountBlockHash:  "0xmint1",
		MomentumHeight:    10,
		MomentumTimestamp: 1700000000,
		TokenStandard:     models.ZnnTokenStandard,
		Issuer:            models.PillarAddress,
		Receiver:          "z1qrecipient",
		Amount:            5000,
	}
	if err := repo.InsertMint(ctx, mint); err != nil {
		t.Fatalf("insert mint: %v", err)
	}
	if err := repo.InsertMint(ctx, mint); err != nil {
		t.Errorf("re-insert mint should be idempotent: %v", err)
	}

	burn := &models.TokenBurn{
		AccountBlockHash:  "0xburn1",
		MomentumHeight:    11,
		MomentumTimestamp: 1700000001,
		TokenStandard:     models.ZnnTokenStandard,
		Burner:            "z1qburner",
		Amount:            200,
	}
	if err := repo.InsertBurn(ctx, burn); err != nil {
		t.Fatalf("insert burn: %v", err)
	}

	var mintCount, burnCount int64
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM token_mints`).Scan(&mintCount)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM token_burns`).Scan(&burnCount)
	if mintCount != 1 || burnCount != 1 {
		t.Errorf("expected 1 mint & 1 burn, got %d mints / %d burns", mintCount, burnCount)
	}
}

func TestIntegration_TokenEvents_SumDailyMintsBurns(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewTokenEventRepository(pool)

	// 2026-01-15 UTC midnight = 1768435200; next-day midnight = 1768521600.
	const dayStart, dayEnd = int64(1768435200), int64(1768521600)
	mk := func(hash string, ts int64, std string, amount int64) *models.TokenMint {
		return &models.TokenMint{
			AccountBlockHash: hash, MomentumHeight: 1, MomentumTimestamp: ts,
			TokenStandard: std, Issuer: "z1qx", Receiver: "z1qy", Amount: amount,
		}
	}
	for _, m := range []*models.TokenMint{
		mk("0xa", dayStart, models.ZnnTokenStandard, 100),
		mk("0xb", dayStart+1, models.ZnnTokenStandard, 250),
		mk("0xc", dayStart+2, models.QsrTokenStandard, 1),
		// next-day mint, must NOT count toward 2026-01-15.
		mk("0xe", dayEnd, models.ZnnTokenStandard, 99999),
	} {
		if err := repo.InsertMint(ctx, m); err != nil {
			t.Fatalf("insert mint %s: %v", m.AccountBlockHash, err)
		}
	}
	if err := repo.InsertBurn(ctx, &models.TokenBurn{
		AccountBlockHash: "0xd", MomentumHeight: 1, MomentumTimestamp: dayStart + 3,
		TokenStandard: models.ZnnTokenStandard, Burner: "z1qb", Amount: 50,
	}); err != nil {
		t.Fatalf("insert burn: %v", err)
	}

	mints, burns, err := repo.SumDailyMintsBurns(ctx, models.ZnnTokenStandard, "2026-01-15")
	if err != nil {
		t.Fatalf("sum: %v", err)
	}
	if mints != 350 {
		t.Errorf("expected mints=350, got %d", mints)
	}
	if burns != 50 {
		t.Errorf("expected burns=50, got %d", burns)
	}
}

func TestIntegration_BridgeConfig_AdminAndGuardiansAndNetworks(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewBridgeConfigRepository(pool)

	if err := repo.UpsertAdmin(ctx, &models.BridgeAdmin{
		Administrator:        "z1qadmin",
		Halted:               true,
		LastUpdatedTimestamp: 1000,
	}); err != nil {
		t.Fatalf("upsert admin: %v", err)
	}
	got, err := repo.GetAdmin(ctx)
	if err != nil {
		t.Fatalf("get admin: %v", err)
	}
	if got == nil || got.Administrator != "z1qadmin" || !got.Halted {
		t.Errorf("admin round-trip wrong: %+v", got)
	}

	for _, addr := range []string{"z1qg1", "z1qg2"} {
		if err := repo.UpsertGuardian(ctx, &models.BridgeGuardian{
			Address:              addr,
			Nominated:            true,
			Accepted:             true,
			LastUpdatedTimestamp: 1000,
		}); err != nil {
			t.Fatalf("upsert guardian: %v", err)
		}
	}
	// Mark anything older than ts=2000 as no longer active.
	if err := repo.MarkGuardiansAbsent(ctx, 2000); err != nil {
		t.Fatalf("mark absent: %v", err)
	}
	var active int
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*) FROM bridge_guardians WHERE nominated = true OR accepted = true`).Scan(&active)
	if active != 0 {
		t.Errorf("expected 0 active guardians after mark-absent, got %d", active)
	}

	if err := repo.UpsertNetwork(ctx, &models.BridgeNetwork{
		NetworkClass: 2, ChainID: 1, Name: "Ethereum", ContractAddress: "0xeth",
	}); err != nil {
		t.Fatalf("upsert network: %v", err)
	}
	if err := repo.UpsertNetworkToken(ctx, &models.BridgeNetworkToken{
		NetworkClass: 2, ChainID: 1, TokenStandard: models.ZnnTokenStandard,
		TokenAddress: "0xwznn", Bridgeable: true, Redeemable: true,
		MinAmount: 1000, FeePercentage: 25, RedeemDelay: 100,
	}); err != nil {
		t.Fatalf("upsert network token: %v", err)
	}
	var netCount, tokenCount int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM bridge_networks`).Scan(&netCount)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM bridge_network_tokens`).Scan(&tokenCount)
	if netCount != 1 || tokenCount != 1 {
		t.Errorf("expected 1 net & 1 pair, got %d / %d", netCount, tokenCount)
	}
}

func TestIntegration_DelegationHistory_CloseAndOpen(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewDelegationRepository(pool)

	// Open delegation to A at ts=100.
	b := &pgx.Batch{}
	repo.OpenBatch(b, "z1qdelegator", "z1qa", 100)
	sendBatch(t, ctx, pool, b)

	got, err := repo.GetActivePillarFor(ctx, "z1qdelegator")
	if err != nil {
		t.Fatalf("get active: %v", err)
	}
	if got != "z1qa" {
		t.Errorf("expected active pillar z1qa, got %q", got)
	}

	// Switch to B at ts=200: close A, open B.
	b = &pgx.Batch{}
	repo.CloseActiveBatch(b, "z1qdelegator", 200)
	repo.OpenBatch(b, "z1qdelegator", "z1qb", 200)
	sendBatch(t, ctx, pool, b)

	got, _ = repo.GetActivePillarFor(ctx, "z1qdelegator")
	if got != "z1qb" {
		t.Errorf("expected active pillar z1qb after switch, got %q", got)
	}

	var n int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM delegations
		WHERE delegator_address = $1`, "z1qdelegator").Scan(&n)
	if n != 2 {
		t.Errorf("expected 2 history rows, got %d", n)
	}

	if n, _ := repo.CountActiveByPillar(ctx, "z1qa"); n != 0 {
		t.Errorf("expected 0 active delegators for A, got %d", n)
	}
	if n, _ := repo.CountActiveByPillar(ctx, "z1qb"); n != 1 {
		t.Errorf("expected 1 active delegator for B, got %d", n)
	}
}

func TestIntegration_StatHistory_NetworkUpsertIsIdempotent(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewStatHistoryRepository(pool)

	s := &models.NetworkStatHistory{
		Date: "2026-05-23", TotalTx: 100, DailyTx: 5,
		TotalAddresses: 10, DailyAddresses: 1, ActiveAddresses: 3,
		TotalTokens: 7, TotalStakes: 2, TotalFusions: 4,
		TotalPillars: 30, TotalSentinels: 20,
	}
	if err := repo.UpsertNetworkStat(ctx, s); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	s.DailyTx = 99
	if err := repo.UpsertNetworkStat(ctx, s); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var n int
	var dailyTx int64
	_ = pool.QueryRow(ctx, `
		SELECT COUNT(*), MAX(daily_tx) FROM network_stat_histories WHERE date = $1`,
		"2026-05-23").Scan(&n, &dailyTx)
	if n != 1 || dailyTx != 99 {
		t.Errorf("expected single row with daily_tx=99, got rows=%d daily_tx=%d", n, dailyTx)
	}
}

func TestIntegration_Account_AddSendAddReceiveAndActivity(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(pool)

	b := &pgx.Batch{}
	repo.AddSendBatch(b, "z1qsender", models.ZnnTokenStandard, 100, 1000)
	repo.AddSendBatch(b, "z1qsender", models.ZnnTokenStandard, 200, 2000)
	repo.AddReceiveBatch(b, "z1qrecv", models.QsrTokenStandard, 50, 1500)
	// Non-ZNN/QSR — only updates activity.
	repo.AddReceiveBatch(b, "z1qsender", "zts1other", 10, 500)
	sendBatch(t, ctx, pool, b)

	sender, err := repo.GetByAddress(ctx, "z1qsender")
	if err != nil {
		t.Fatalf("get sender: %v", err)
	}
	if sender.ZnnSent != 300 {
		t.Errorf("expected znn_sent=300, got %d", sender.ZnnSent)
	}
	if sender.FirstActiveAt == nil || *sender.FirstActiveAt != 500 {
		t.Errorf("first_active_at should be 500, got %v", sender.FirstActiveAt)
	}
	if sender.LastActiveAt == nil || *sender.LastActiveAt != 2000 {
		t.Errorf("last_active_at should be 2000, got %v", sender.LastActiveAt)
	}

	recv, _ := repo.GetByAddress(ctx, "z1qrecv")
	if recv.QsrReceived != 50 {
		t.Errorf("expected qsr_received=50, got %d", recv.QsrReceived)
	}
}

func TestIntegration_Account_BackfillSeenAndTxCount(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()

	// Seed account_blocks directly: three blocks involving z1qA and z1qB.
	//  block h1: A -> B  @ 1000  (A appears as address, B as to_address)
	//  block h2: receive on B   @ 1500  (B as address)
	//  block h3: B -> EmptyAddress @ 2000 (B as address, EmptyAddress as to_address)
	type row struct {
		hash, addr, to string
		ts             int64
	}
	rows := []row{
		{"h1", "z1qA", "z1qB", 1000},
		{"h2", "z1qB", "", 1500},
		{"h3", "z1qB", models.EmptyAddress, 2000},
	}
	for _, r := range rows {
		if _, err := pool.Exec(ctx, `
			INSERT INTO account_blocks (
				hash, momentum_hash, momentum_timestamp, momentum_height,
				block_type, height, address, to_address, amount, token_standard
			) VALUES ($1::text, 'm', $4::bigint, 1, 0, 1, $2::text, $3::text, 0, '')`,
			r.hash, r.addr, r.to, r.ts); err != nil {
			t.Fatalf("seed %s: %v", r.hash, err)
		}
	}

	// Re-run the backfill from migration 012 to verify it on real rows.
	// The migration itself ran against an empty table during TestMain.
	if _, err := pool.Exec(ctx, `
		INSERT INTO accounts (address, block_count, public_key, first_seen, last_seen, tx_count)
		SELECT addr, 0, '',
			MIN(momentum_timestamp), MAX(momentum_timestamp), COUNT(*)
		FROM (
			SELECT hash, address AS addr, momentum_timestamp FROM account_blocks
			UNION
			SELECT hash, to_address AS addr, momentum_timestamp FROM account_blocks
			WHERE to_address IS NOT NULL AND to_address <> ''
		) appearances
		GROUP BY addr
		ON CONFLICT (address) DO UPDATE SET
			first_seen = EXCLUDED.first_seen,
			last_seen  = EXCLUDED.last_seen,
			tx_count   = EXCLUDED.tx_count`); err != nil {
		t.Fatalf("backfill: %v", err)
	}

	repo := NewAccountRepository(pool)

	a, err := repo.GetByAddress(ctx, "z1qA")
	if err != nil {
		t.Fatalf("get z1qA: %v", err)
	}
	if a.TxCount != 1 || a.FirstSeen == nil || *a.FirstSeen != 1000 || a.LastSeen == nil || *a.LastSeen != 1000 {
		t.Errorf("z1qA: tx=%d first=%v last=%v, want tx=1 first=last=1000",
			a.TxCount, a.FirstSeen, a.LastSeen)
	}

	b, err := repo.GetByAddress(ctx, "z1qB")
	if err != nil {
		t.Fatalf("get z1qB: %v", err)
	}
	// z1qB appears in h1 (as to_address), h2 (as address), h3 (as address) → 3
	if b.TxCount != 3 || b.FirstSeen == nil || *b.FirstSeen != 1000 || b.LastSeen == nil || *b.LastSeen != 2000 {
		t.Errorf("z1qB: tx=%d first=%v last=%v, want tx=3 first=1000 last=2000",
			b.TxCount, b.FirstSeen, b.LastSeen)
	}

	// EmptyAddress appears only as to_address in h3 — backfill stub created.
	e, err := repo.GetByAddress(ctx, models.EmptyAddress)
	if err != nil {
		t.Fatalf("get EmptyAddress: %v", err)
	}
	if e.TxCount != 1 {
		t.Errorf("EmptyAddress: tx=%d, want 1", e.TxCount)
	}
}

func TestIntegration_Account_BumpTxCount(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewAccountRepository(pool)

	b := &pgx.Batch{}
	// Three "appearances" for z1qa across times 1000, 2000, 500.
	repo.BumpTxCountBatch(b, "z1qa", 1000)
	repo.BumpTxCountBatch(b, "z1qa", 2000)
	repo.BumpTxCountBatch(b, "z1qa", 500)
	// One appearance for z1qb at time 1500.
	repo.BumpTxCountBatch(b, "z1qb", 1500)
	sendBatch(t, ctx, pool, b)

	a, err := repo.GetByAddress(ctx, "z1qa")
	if err != nil {
		t.Fatalf("get z1qa: %v", err)
	}
	if a.TxCount != 3 {
		t.Errorf("z1qa.tx_count = %d, want 3", a.TxCount)
	}
	if a.FirstSeen == nil || *a.FirstSeen != 500 {
		t.Errorf("z1qa.first_seen = %v, want 500", a.FirstSeen)
	}
	if a.LastSeen == nil || *a.LastSeen != 2000 {
		t.Errorf("z1qa.last_seen = %v, want 2000", a.LastSeen)
	}

	bAcc, err := repo.GetByAddress(ctx, "z1qb")
	if err != nil {
		t.Fatalf("get z1qb: %v", err)
	}
	if bAcc.TxCount != 1 || bAcc.FirstSeen == nil || *bAcc.FirstSeen != 1500 ||
		bAcc.LastSeen == nil || *bAcc.LastSeen != 1500 {
		t.Errorf("z1qb counters = tx=%d first=%v last=%v, want tx=1 first=last=1500",
			bAcc.TxCount, bAcc.FirstSeen, bAcc.LastSeen)
	}
}

func TestIntegration_Pillar_IsWithdrawAddress(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	pr := NewPillarRepository(pool)
	pur := NewPillarUpdateRepository(pool)

	// Current pillar's withdraw address.
	if err := pr.Upsert(ctx, &models.Pillar{
		OwnerAddress:    "z1qowner",
		ProducerAddress: "z1qproducer",
		WithdrawAddress: "z1qcurrent",
		Name:            "p1",
	}); err != nil {
		t.Fatalf("upsert pillar: %v", err)
	}
	// Historical withdraw address from a pillar_update.
	if err := pur.Insert(ctx, &models.PillarUpdate{
		Name:            "p1",
		OwnerAddress:    "z1qowner",
		ProducerAddress: "z1qproducer",
		WithdrawAddress: "z1qhistoric",
		MomentumHeight:  1,
		MomentumHash:    "h",
	}); err != nil {
		t.Fatalf("insert pillar_update: %v", err)
	}

	for _, tc := range []struct {
		addr string
		want bool
	}{
		{"z1qcurrent", true},
		{"z1qhistoric", true},
		{"z1qunrelated", false},
	} {
		got, err := pr.IsWithdrawAddress(ctx, tc.addr)
		if err != nil {
			t.Fatalf("IsWithdrawAddress(%q): %v", tc.addr, err)
		}
		if got != tc.want {
			t.Errorf("IsWithdrawAddress(%q) = %v, want %v", tc.addr, got, tc.want)
		}
	}
}
