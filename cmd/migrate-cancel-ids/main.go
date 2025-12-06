package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/0x3639/znn-sdk-go/embedded"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/zenon-network/go-zenon/common/types"
)

// getStakeCancelID computes the cancel ID for a stake
func getStakeCancelID(id string) (string, error) {
	hash, err := types.HexToHash(id)
	if err != nil {
		return "", fmt.Errorf("invalid hash %s: %w", id, err)
	}

	encoded, err := embedded.Stake.EncodeFunction("Cancel", []interface{}{hash})
	if err != nil {
		return "", fmt.Errorf("encode failed: %w", err)
	}

	decoded, err := embedded.Stake.DecodeFunction(encoded)
	if err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	if len(decoded) > 0 {
		if h, ok := decoded[0].(types.Hash); ok {
			return h.String(), nil
		}
	}

	return id, nil
}

// getFusionCancelID computes the cancel ID for a fusion
func getFusionCancelID(id string) (string, error) {
	hash, err := types.HexToHash(id)
	if err != nil {
		return "", fmt.Errorf("invalid hash %s: %w", id, err)
	}

	encoded, err := embedded.Plasma.EncodeFunction("CancelFuse", []interface{}{hash})
	if err != nil {
		return "", fmt.Errorf("encode failed: %w", err)
	}

	decoded, err := embedded.Plasma.DecodeFunction(encoded)
	if err != nil {
		return "", fmt.Errorf("decode failed: %w", err)
	}

	if len(decoded) > 0 {
		if h, ok := decoded[0].(types.Hash); ok {
			return h.String(), nil
		}
	}

	return id, nil
}

func main() {
	// Get database connection string from environment
	dbHost := getEnv("DATABASE_ADDRESS", "localhost")
	dbPort := getEnv("DATABASE_PORT", "5432")
	dbName := getEnv("DATABASE_NAME", "nom_indexer")
	dbUser := getEnv("DATABASE_USERNAME", "postgres")
	dbPass := getEnv("DATABASE_PASSWORD", "")

	connStr := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		dbUser, dbPass, dbHost, dbPort, dbName)

	ctx := context.Background()

	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	log.Println("Connected to database")

	// Migrate stakes
	if err := migrateStakes(ctx, pool); err != nil {
		log.Fatalf("Failed to migrate stakes: %v", err)
	}

	// Migrate fusions
	if err := migrateFusions(ctx, pool); err != nil {
		log.Fatalf("Failed to migrate fusions: %v", err)
	}

	log.Println("Migration completed successfully!")
}

func migrateStakes(ctx context.Context, pool *pgxpool.Pool) error {
	log.Println("Migrating stakes...")

	// Get all stakes
	rows, err := pool.Query(ctx, "SELECT id, cancel_id FROM stakes")
	if err != nil {
		return fmt.Errorf("query stakes: %w", err)
	}
	defer rows.Close()

	var updates []struct {
		id       string
		cancelID string
	}

	for rows.Next() {
		var id, currentCancelID string
		if err := rows.Scan(&id, &currentCancelID); err != nil {
			return fmt.Errorf("scan stake: %w", err)
		}

		newCancelID, err := getStakeCancelID(id)
		if err != nil {
			log.Printf("Warning: could not compute cancel_id for stake %s: %v", id, err)
			continue
		}

		if newCancelID != currentCancelID {
			updates = append(updates, struct {
				id       string
				cancelID string
			}{id, newCancelID})
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate stakes: %w", err)
	}

	log.Printf("Found %d stakes to update", len(updates))

	// Update in batches
	for i, u := range updates {
		_, err := pool.Exec(ctx, "UPDATE stakes SET cancel_id = $1 WHERE id = $2", u.cancelID, u.id)
		if err != nil {
			return fmt.Errorf("update stake %s: %w", u.id, err)
		}

		if (i+1)%1000 == 0 {
			log.Printf("Updated %d/%d stakes", i+1, len(updates))
		}
	}

	log.Printf("Updated %d stakes", len(updates))
	return nil
}

func migrateFusions(ctx context.Context, pool *pgxpool.Pool) error {
	log.Println("Migrating fusions...")

	// Get all fusions
	rows, err := pool.Query(ctx, "SELECT id, cancel_id FROM fusions")
	if err != nil {
		return fmt.Errorf("query fusions: %w", err)
	}
	defer rows.Close()

	var updates []struct {
		id       string
		cancelID string
	}

	for rows.Next() {
		var id, currentCancelID string
		if err := rows.Scan(&id, &currentCancelID); err != nil {
			return fmt.Errorf("scan fusion: %w", err)
		}

		newCancelID, err := getFusionCancelID(id)
		if err != nil {
			log.Printf("Warning: could not compute cancel_id for fusion %s: %v", id, err)
			continue
		}

		if newCancelID != currentCancelID {
			updates = append(updates, struct {
				id       string
				cancelID string
			}{id, newCancelID})
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate fusions: %w", err)
	}

	log.Printf("Found %d fusions to update", len(updates))

	// Update in batches
	for i, u := range updates {
		_, err := pool.Exec(ctx, "UPDATE fusions SET cancel_id = $1 WHERE id = $2", u.cancelID, u.id)
		if err != nil {
			return fmt.Errorf("update fusion %s: %w", u.id, err)
		}

		if (i+1)%1000 == 0 {
			log.Printf("Updated %d/%d fusions", i+1, len(updates))
		}
	}

	log.Printf("Updated %d fusions", len(updates))
	return nil
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}
