//go:build integration

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

// allOpts returns a ListOpts that asks for every row with whatever ordering
// the method's default is.
func allOpts() ListOpts {
	return ListOpts{Limit: 1000, Offset: 0}
}

func TestIntegration_MomentumRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewMomentumRepository(pool)

	for h := uint64(1); h <= 5; h++ {
		if err := repo.Insert(ctx, &models.Momentum{
			Height: h, Hash: fmt.Sprintf("hash-%d", h), Timestamp: int64(1000 + h*10), TxCount: int(h),
			Producer: "z1qp",
		}); err != nil {
			t.Fatalf("seed momentum %d: %v", h, err)
		}
	}

	// List default = newest first.
	rows, total, err := repo.List(ctx, allOpts())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 5 || len(rows) != 5 {
		t.Fatalf("List = %d rows / total %d, want 5/5", len(rows), total)
	}
	if rows[0].Height != 5 {
		t.Errorf("first row height = %d, want 5 (newest first)", rows[0].Height)
	}

	// ASC reorders.
	asc, _, _ := repo.List(ctx, ListOpts{Limit: 1000, Sort: "asc"})
	if asc[0].Height != 1 {
		t.Errorf("ASC first row height = %d, want 1", asc[0].Height)
	}

	// Pagination: page 2 of 2.
	page2, total2, _ := repo.List(ctx, ListOpts{Limit: 2, Offset: 2})
	if total2 != 5 || len(page2) != 2 {
		t.Errorf("page2: got %d rows / total %d, want 2/5", len(page2), total2)
	}

	// Latest.
	latest, err := repo.GetLatest(ctx)
	if err != nil || latest.Height != 5 {
		t.Errorf("GetLatest: height=%d err=%v, want 5/nil", latest.Height, err)
	}

	// ListByProducer filters.
	prod, total3, _ := repo.ListByProducer(ctx, "z1qp", allOpts())
	if total3 != 5 || len(prod) != 5 {
		t.Errorf("ListByProducer: got %d/%d, want 5/5", len(prod), total3)
	}
	if _, _, err := repo.ListByProducer(ctx, "", allOpts()); err == nil {
		t.Error("expected error on empty producer")
	}
}

func TestIntegration_AccountBlockRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewAccountBlockRepository(pool)

	for i := 1; i <= 4; i++ {
		ab := &models.AccountBlock{
			Hash: fmt.Sprintf("b%d", i), MomentumHeight: int64(100 + i),
			BlockType: 2, Height: int64(i), Address: "z1qsender", ToAddress: "z1qrecv", Amount: int64(i * 10),
		}
		if err := repo.Insert(ctx, ab, nil); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// AccountBlock.List reads pg_class.reltuples for the page total
	// (see comment on the method). reltuples is only refreshed by
	// ANALYZE / autovacuum, so a freshly-truncated table returns 0
	// until we kick it. ANALYZE here keeps the test deterministic.
	if _, err := pool.Exec(ctx, `ANALYZE account_blocks`); err != nil {
		t.Fatalf("analyze: %v", err)
	}

	rows, total, err := repo.List(ctx, allOpts())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 4 {
		t.Errorf("List got %d rows, want 4", len(rows))
	}
	// reltuples is approximate but post-ANALYZE on a 4-row table should
	// land exactly at 4. If autovacuum doesn't grace us, accept ±1.
	if total < 3 || total > 5 {
		t.Errorf("List total = %d, want ~4 (pg_class.reltuples)", total)
	}

	// ListByAddress sources total from accounts.tx_count, so seed it.
	// In production the indexer maintains this in the same transaction as
	// the block insert; here we mirror what BumpTxCountBatch would write.
	if _, err := pool.Exec(ctx, `
		INSERT INTO accounts (address, block_count, public_key, tx_count) VALUES
			($1, 0, '', 4),
			($2, 0, '', 4)
		ON CONFLICT (address) DO UPDATE SET tx_count = EXCLUDED.tx_count`,
		"z1qsender", "z1qrecv"); err != nil {
		t.Fatalf("seed accounts: %v", err)
	}

	byAddr, total2, _ := repo.ListByAddress(ctx, "z1qsender", allOpts())
	if total2 != 4 || len(byAddr) != 4 {
		t.Errorf("ListByAddress(sender) got %d/%d, want 4/4", len(byAddr), total2)
	}
	byAddrRecv, total3, _ := repo.ListByAddress(ctx, "z1qrecv", allOpts())
	if total3 != 4 || len(byAddrRecv) != 4 {
		t.Errorf("ListByAddress(recipient) got %d/%d, want 4/4", len(byAddrRecv), total3)
	}

	// Address with no account row: 0 total (not an error).
	_, totalMissing, err := repo.ListByAddress(ctx, "z1qunseen", allOpts())
	if err != nil {
		t.Fatalf("ListByAddress(unseen): %v", err)
	}
	if totalMissing != 0 {
		t.Errorf("ListByAddress(unseen) total = %d, want 0", totalMissing)
	}

	// Cached-path proof: deliberately desync the counter from the block
	// rows. The handler must return the cached value, not a freshly
	// computed COUNT(*) — that's the whole point of this optimization.
	if _, err := pool.Exec(ctx, `UPDATE accounts SET tx_count = 999 WHERE address = $1`, "z1qsender"); err != nil {
		t.Fatalf("desync: %v", err)
	}
	_, totalCached, _ := repo.ListByAddress(ctx, "z1qsender", allOpts())
	if totalCached != 999 {
		t.Errorf("ListByAddress(cached) total = %d, want 999 (from accounts.tx_count)", totalCached)
	}

	if _, _, err := repo.ListByAddress(ctx, "", allOpts()); err == nil {
		t.Error("expected error on empty address")
	}
}

func TestIntegration_BalanceRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewBalanceRepository(pool)

	for i := 1; i <= 3; i++ {
		if err := repo.Upsert(ctx, &models.Balance{
			Address: fmt.Sprintf("z1qa%d", i), TokenStandard: models.ZnnTokenStandard,
			Balance: int64(i * 100), LastUpdatedTimestamp: 1000,
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}
	// Zero balance is excluded by ListByToken.
	_ = repo.Upsert(ctx, &models.Balance{
		Address: "z1qzero", TokenStandard: models.ZnnTokenStandard, Balance: 0,
	})

	byAddr, _ := repo.ListByAddress(ctx, "z1qa2")
	if len(byAddr) != 1 {
		t.Errorf("ListByAddress got %d, want 1", len(byAddr))
	}

	byTok, total, err := repo.ListByToken(ctx, models.ZnnTokenStandard, allOpts())
	if err != nil {
		t.Fatalf("ListByToken: %v", err)
	}
	if total != 3 || len(byTok) != 3 {
		t.Errorf("ListByToken got %d/%d, want 3/3 (excluding zero)", len(byTok), total)
	}
	// DESC by balance.
	if byTok[0].Balance != 300 {
		t.Errorf("first row balance = %d, want 300", byTok[0].Balance)
	}
}

func TestIntegration_TokenRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewTokenRepository(pool)

	for i := 1; i <= 3; i++ {
		t.Helper()
		if err := repo.Upsert(ctx, &models.Token{
			TokenStandard: fmt.Sprintf("zts1t%d", i), Name: fmt.Sprintf("T%d", i),
			Symbol: fmt.Sprintf("T%d", i), Decimals: 8, Owner: "z1qo",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	rows, total, err := repo.List(ctx, allOpts())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 3 || len(rows) != 3 {
		t.Errorf("List got %d/%d, want 3/3", len(rows), total)
	}

	tok, err := repo.GetByStandard(ctx, "zts1t1")
	if err != nil || tok.Symbol != "T1" {
		t.Errorf("GetByStandard: %+v err=%v", tok, err)
	}
}

func TestIntegration_PillarRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewPillarRepository(pool)

	for i := 1; i <= 3; i++ {
		_ = repo.Upsert(ctx, &models.Pillar{
			OwnerAddress: fmt.Sprintf("z1qo%d", i), ProducerAddress: fmt.Sprintf("z1qp%d", i),
			WithdrawAddress: fmt.Sprintf("z1qw%d", i), Name: fmt.Sprintf("p%d", i), Rank: i,
		})
	}
	_ = repo.SetAsRevoked(ctx, "z1qo3", "p3", 1000)

	active, total, err := repo.List(ctx, false, allOpts())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if total != 2 || len(active) != 2 {
		t.Errorf("List(activeOnly) got %d/%d, want 2/2", len(active), total)
	}

	all, totalAll, _ := repo.List(ctx, true, allOpts())
	if totalAll != 3 || len(all) != 3 {
		t.Errorf("List(includeRevoked) got %d/%d, want 3/3", len(all), totalAll)
	}
}

func TestIntegration_PillarRepo_ListDelegators(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewPillarRepository(pool)
	ar := NewAccountRepository(pool)

	_ = ar.Upsert(ctx, &models.Account{Address: "z1qd1"})
	_ = ar.Upsert(ctx, &models.Account{Address: "z1qd2"})
	_ = ar.UpdateDelegate(ctx, "z1qd1", "z1qp", 1000)
	_ = ar.UpdateDelegate(ctx, "z1qd2", "z1qp", 1500)

	dels, total, err := repo.ListDelegators(ctx, "z1qp", allOpts())
	if err != nil {
		t.Fatalf("ListDelegators: %v", err)
	}
	if total != 2 || len(dels) != 2 {
		t.Errorf("got %d/%d, want 2/2", len(dels), total)
	}
}

func TestIntegration_SentinelRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewSentinelRepository(pool)

	for i := 1; i <= 2; i++ {
		_ = repo.Upsert(ctx, &models.Sentinel{
			Owner: fmt.Sprintf("z1qs%d", i), RegistrationTimestamp: int64(1000 * i),
			IsRevocable: true, RevokeCooldown: "0", Active: true,
		})
	}
	_ = repo.Upsert(ctx, &models.Sentinel{
		Owner: "z1qs3", RegistrationTimestamp: 100, IsRevocable: false, RevokeCooldown: "0", Active: false,
	})

	active, total, _ := repo.List(ctx, true, allOpts())
	if total != 2 || len(active) != 2 {
		t.Errorf("List(activeOnly): got %d/%d, want 2/2", len(active), total)
	}
	all, totalAll, _ := repo.List(ctx, false, allOpts())
	if totalAll != 3 || len(all) != 3 {
		t.Errorf("List(all): got %d/%d, want 3/3", len(all), totalAll)
	}
}

func TestIntegration_StakeFusionRepos_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	sr := NewStakeRepository(pool)
	fr := NewFusionRepository(pool)

	for i := 1; i <= 2; i++ {
		_ = sr.Insert(ctx, &models.Stake{
			ID: fmt.Sprintf("s%d", i), Address: "z1qstaker",
			StartTimestamp: int64(1000 * i), ExpirationTimestamp: int64(2000 * i),
			ZnnAmount: int64(i * 100), DurationInSec: 100, IsActive: true, CancelID: fmt.Sprintf("c%d", i),
		})
		_ = fr.Insert(ctx, &models.Fusion{
			ID: fmt.Sprintf("f%d", i), Address: "z1qfuser", Beneficiary: "z1qbene",
			MomentumHash: "mh", MomentumTimestamp: int64(1000 * i), MomentumHeight: int64(i),
			QsrAmount: int64(i * 50), ExpirationHeight: int64(100 * i), IsActive: true, CancelID: fmt.Sprintf("fc%d", i),
		})
	}

	stakes, stakeTotal, _ := sr.List(ctx, true, allOpts())
	if stakeTotal != 2 || len(stakes) != 2 {
		t.Errorf("Stake.List got %d/%d", len(stakes), stakeTotal)
	}
	stakesByAddr, _, _ := sr.ListByAddress(ctx, "z1qstaker", true, allOpts())
	if len(stakesByAddr) != 2 {
		t.Errorf("Stake.ListByAddress got %d, want 2", len(stakesByAddr))
	}

	fusions, fusionTotal, _ := fr.List(ctx, true, allOpts())
	if fusionTotal != 2 || len(fusions) != 2 {
		t.Errorf("Fusion.List got %d/%d", len(fusions), fusionTotal)
	}
	fusionsByBene, _, _ := fr.ListByAddress(ctx, "z1qbene", true, allOpts())
	if len(fusionsByBene) != 2 {
		t.Errorf("Fusion.ListByAddress(beneficiary) got %d, want 2", len(fusionsByBene))
	}
}

func TestIntegration_ProjectVoteRepos_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	pr := NewProjectRepository(pool)
	pph := NewProjectPhaseRepository(pool)
	vr := NewVoteRepository(pool)

	_ = pr.Upsert(ctx, &models.Project{ID: "p1", VotingID: "v1", Owner: "z1qo", Name: "P1", CreationTimestamp: 1000})
	_ = pr.Upsert(ctx, &models.Project{ID: "p2", VotingID: "v2", Owner: "z1qo", Name: "P2", CreationTimestamp: 2000})

	_ = pph.Upsert(ctx, &models.ProjectPhase{ID: "ph1", ProjectID: "p1", VotingID: "phv1", Name: "phase 1", CreationTimestamp: 1100})
	_ = pph.Upsert(ctx, &models.ProjectPhase{ID: "ph2", ProjectID: "p1", VotingID: "phv2", Name: "phase 2", CreationTimestamp: 1200})

	_ = vr.Insert(ctx, &models.Vote{MomentumHash: "mh", MomentumHeight: 1, VoterAddress: "z1qv", ProjectID: "p1", VotingID: "v1"})
	_ = vr.Insert(ctx, &models.Vote{MomentumHash: "mh", MomentumHeight: 2, VoterAddress: "z1qv2", ProjectID: "p1", VotingID: "v1b"})

	rows, total, _ := pr.List(ctx, allOpts())
	if total != 2 || len(rows) != 2 {
		t.Errorf("Project.List got %d/%d", len(rows), total)
	}

	got, err := pr.GetByID(ctx, "p1")
	if err != nil || got.Name != "P1" {
		t.Errorf("Project.GetByID: %+v err=%v", got, err)
	}

	phases, _ := pph.ListByProject(ctx, "p1")
	if len(phases) != 2 {
		t.Errorf("ProjectPhase.ListByProject got %d, want 2", len(phases))
	}

	votes, voteTotal, _ := vr.ListByProject(ctx, "p1", allOpts())
	if voteTotal != 2 || len(votes) != 2 {
		t.Errorf("Vote.ListByProject got %d/%d", len(votes), voteTotal)
	}
}

func TestIntegration_RewardRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewRewardRepository(pool)

	for i := 1; i <= 3; i++ {
		_ = repo.UpdateCumulativeRewards(ctx, "z1qrcv", models.RewardTypePillar, int64(i*100), models.ZnnTokenStandard)
		_ = repo.InsertRewardTransaction(ctx, &models.RewardTransaction{
			Hash: fmt.Sprintf("rt%d", i), Address: "z1qrcv", RewardType: models.RewardTypePillar,
			MomentumTimestamp: int64(1000 * i), MomentumHeight: int64(i), AccountHeight: int64(i),
			Amount: int64(i * 100), TokenStandard: models.ZnnTokenStandard, SourceAddress: models.PillarAddress,
		})
	}

	cum, err := repo.CumulativeByAddress(ctx, "z1qrcv")
	if err != nil {
		t.Fatalf("CumulativeByAddress: %v", err)
	}
	if len(cum) != 1 || cum[0].Amount != 600 {
		t.Errorf("CumulativeByAddress = %+v, want one row amount=600 (100+200+300)", cum)
	}

	hist, total, _ := repo.HistoryByAddress(ctx, "z1qrcv", allOpts())
	if total != 3 || len(hist) != 3 {
		t.Errorf("HistoryByAddress got %d/%d, want 3/3", len(hist), total)
	}
}

func TestIntegration_BridgeRepo_List(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewBridgeRepository(pool)

	for i := 1; i <= 2; i++ {
		_ = repo.UpsertWrapRequest(ctx, &models.WrapTokenRequest{
			ID: fmt.Sprintf("w%d", i), NetworkClass: 2, ChainID: 1,
			ToAddress: "0xeth", TokenStandard: models.ZnnTokenStandard,
			TokenAddress: "0xt", Amount: int64(i * 100), Fee: 1,
			CreationMomentumHeight: int64(i * 10),
		})
		_ = repo.UpsertUnwrapRequest(ctx, &models.UnwrapTokenRequest{
			TransactionHash: fmt.Sprintf("0xtx%d", i), LogIndex: 0, NetworkClass: 2, ChainID: 1,
			ToAddress: "z1qrcv", TokenStandard: models.ZnnTokenStandard,
			TokenAddress: "0xt", Amount: int64(i * 50),
			RegistrationMomentumHeight: int64(i * 10),
		})
	}

	wraps, wrapTotal, _ := repo.ListWraps(ctx, allOpts())
	if wrapTotal != 2 || len(wraps) != 2 {
		t.Errorf("ListWraps got %d/%d", len(wraps), wrapTotal)
	}
	wrapsByAddr, _, _ := repo.ListWrapsByAddress(ctx, "0xeth", allOpts())
	if len(wrapsByAddr) != 2 {
		t.Errorf("ListWrapsByAddress got %d", len(wrapsByAddr))
	}

	unwraps, unwrapTotal, _ := repo.ListUnwraps(ctx, allOpts())
	if unwrapTotal != 2 || len(unwraps) != 2 {
		t.Errorf("ListUnwraps got %d/%d", len(unwraps), unwrapTotal)
	}
	unwrapsByAddr, _, _ := repo.ListUnwrapsByAddress(ctx, "z1qrcv", allOpts())
	if len(unwrapsByAddr) != 2 {
		t.Errorf("ListUnwrapsByAddress got %d", len(unwrapsByAddr))
	}
}

// TestIntegration_Pagination_TotalBeyondLastPage confirms the fallback
// COUNT(*) path: when OFFSET skips past every row, the COUNT(*) OVER ()
// in the paged SELECT returns no rows, so total used to come back as 0
// for a non-empty table. The fallback runs an explicit COUNT(*) in
// that case.
func TestIntegration_Pagination_TotalBeyondLastPage(t *testing.T) {
	pool := newTestDB(t)
	ctx := context.Background()
	repo := NewMomentumRepository(pool)

	for h := uint64(1); h <= 3; h++ {
		if err := repo.Insert(ctx, &models.Momentum{
			Height: h, Hash: fmt.Sprintf("h-%d", h), Producer: "z1qp",
		}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	// Asking for page 100 (offset 99) of a 3-row table returns zero
	// rows but must still report total=3.
	rows, total, err := repo.List(ctx, ListOpts{Limit: 1, Offset: 99})
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(rows) != 0 {
		t.Errorf("got %d rows, want 0", len(rows))
	}
	if total != 3 {
		t.Errorf("total = %d, want 3 (fallback count missed)", total)
	}

	// Same edge case via the filtered variant.
	rows, total, err = repo.ListByProducer(ctx, "z1qp", ListOpts{Limit: 1, Offset: 99})
	if err != nil {
		t.Fatalf("ListByProducer: %v", err)
	}
	if len(rows) != 0 || total != 3 {
		t.Errorf("ListByProducer fallback: rows=%d total=%d, want 0/3", len(rows), total)
	}
}
