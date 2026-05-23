// backfill-rewards re-derives reward_transactions and cumulative_rewards from
// existing account_blocks rows, populating data that was missed when the
// indexer's reward-detection branch had the wrong BlockType literals.
//
// The reward pattern, expressed against our schema:
//
//	receive block (block_type = UserReceive = 3)
//	    .paired_account_block -> send block (block_type = ContractSend = 4)
//	        .address ∈ {Pillar, Sentinel, Stake}
//	receive.to_address      = EmptyAddress
//	receive.token_standard  = EmptyTokenStandard
//
// Or, for liquidity rewards:
//
//	receive block (block_type = UserReceive = 3)
//	    .paired_account_block -> send block whose .address = LiquidityTreasuryAddress
//
// Usage:
//
//	DATABASE_PASSWORD=<password> go run ./scripts/backfill-rewards
//
// The script only updates cumulative_rewards after inserting a new
// reward_transactions row. Existing reward hashes are skipped, so re-running
// the script will not double-count rows it already inserted.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

const (
	blockTypeUserReceive  = 3
	blockTypeContractSend = 4
)

func main() {
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, buildConnString())
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	insertedRT, addedCumulative, err := backfill(ctx, pool)
	if err != nil {
		log.Fatalf("backfill: %v", err)
	}
	log.Printf("done. reward_transactions inserted: %d, cumulative_rewards rows updated: %d",
		insertedRT, addedCumulative)
}

func backfill(ctx context.Context, pool *pgxpool.Pool) (insertedRT, addedCum int64, err error) {
	// Resolve reward source -> RewardType inside the SQL via CASE.
	// liquidity rewards are detected when the send block's address is the
	// LiquidityTreasuryAddress regardless of the embedded-contract list.
	q := `
WITH receives AS (
    SELECT ab.hash               AS receive_hash,
           ab.address            AS receive_address,
           ab.momentum_height    AS momentum_height,
           ab.momentum_timestamp AS momentum_timestamp,
           ab.height             AS account_height,
           paired.address        AS source_address,
           paired.amount         AS reward_amount,
           paired.token_standard AS token_standard,
           paired.block_type     AS paired_block_type
    FROM account_blocks ab
    JOIN account_blocks paired ON paired.hash = ab.paired_account_block
    WHERE ab.block_type = $1
      AND (
        -- Liquidity reward: source is the LP treasury, no other constraints.
        paired.address = $2
        OR (
            paired.block_type = $3
            AND ab.to_address = $4
            AND ab.token_standard = $5
            AND paired.address IN ($6, $7, $8)
        )
      )
)
SELECT receive_hash, receive_address, momentum_height, momentum_timestamp,
       account_height, source_address, reward_amount, token_standard
FROM receives
ORDER BY momentum_height
`
	rows, err := pool.Query(ctx, q,
		blockTypeUserReceive,
		models.LiquidityTreasuryAddress,
		blockTypeContractSend,
		models.EmptyAddress,
		models.EmptyTokenStandard,
		models.PillarAddress,
		models.SentinelAddress,
		models.StakeAddress,
	)
	if err != nil {
		return 0, 0, fmt.Errorf("scan candidate receives: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var (
			receiveHash       string
			receiveAddress    string
			momentumHeight    int64
			momentumTimestamp int64
			accountHeight     int64
			sourceAddress     string
			rewardAmountStr   string
			tokenStandard     string
		)
		if err := rows.Scan(&receiveHash, &receiveAddress, &momentumHeight,
			&momentumTimestamp, &accountHeight, &sourceAddress,
			&rewardAmountStr, &tokenStandard); err != nil {
			return insertedRT, addedCum, fmt.Errorf("scan row: %w", err)
		}

		// account_blocks.amount is BIGINT; pgx scans it as int64 already, but
		// we read it as text to avoid issues if a future migration widens the
		// column. Fall back to 0 on parse error rather than crash the script.
		amount, perr := strconv.ParseInt(rewardAmountStr, 10, 64)
		if perr != nil {
			log.Printf("warn: bad amount %q for %s: %v", rewardAmountStr, receiveHash, perr)
			continue
		}

		rewardType := determineRewardType(sourceAddress)
		if rewardType == models.RewardTypeUnknown {
			continue
		}

		ct, err := pool.Exec(ctx, `
			INSERT INTO reward_transactions (hash, address, reward_type,
				momentum_timestamp, momentum_height, account_height,
				amount, token_standard, source_address)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
			ON CONFLICT (hash) DO NOTHING`,
			receiveHash, receiveAddress, int(rewardType),
			momentumTimestamp, momentumHeight, accountHeight,
			amount, tokenStandard, sourceAddress)
		if err != nil {
			return insertedRT, addedCum, fmt.Errorf("insert reward_transaction %s: %w", receiveHash, err)
		}
		if ct.RowsAffected() == 0 {
			// Already present from the live indexer — don't touch cumulative.
			continue
		}
		insertedRT++

		if _, err := pool.Exec(ctx, `
			INSERT INTO cumulative_rewards (address, reward_type, amount, token_standard)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (address, reward_type, token_standard) DO UPDATE SET
				amount = cumulative_rewards.amount + $3`,
			receiveAddress, int(rewardType), amount, tokenStandard); err != nil {
			return insertedRT, addedCum, fmt.Errorf("upsert cumulative %s: %w", receiveAddress, err)
		}
		addedCum++

		if insertedRT%1000 == 0 {
			log.Printf("progress: %d reward_transactions inserted", insertedRT)
		}
	}

	return insertedRT, addedCum, rows.Err()
}

func determineRewardType(sourceAddress string) models.RewardType {
	switch sourceAddress {
	case models.PillarAddress:
		return models.RewardTypePillar
	case models.SentinelAddress:
		return models.RewardTypeSentinel
	case models.StakeAddress:
		return models.RewardTypeStake
	case models.LiquidityTreasuryAddress, models.LiquidityAddress:
		return models.RewardTypeLiquidity
	default:
		return models.RewardTypeUnknown
	}
}

func buildConnString() string {
	user := envOrDefault("DATABASE_USERNAME", "postgres")
	pass := envOrDefault("DATABASE_PASSWORD", "")
	host := envOrDefault("DATABASE_ADDRESS", "localhost")
	port := envOrDefault("DATABASE_PORT", "5432")
	db := envOrDefault("DATABASE_NAME", "nom_indexer")
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, pass, host, port, db)
}

func envOrDefault(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
