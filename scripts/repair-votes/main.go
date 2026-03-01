// repair-votes re-decodes vote data from account_blocks using the fixed SDK
// and replaces all records in the votes table with correctly decoded values.
//
// Usage:
//
//	DATABASE_PASSWORD=<password> go run scripts/repair-votes/main.go
//
// It reads the same environment variables as the indexer (DATABASE_ADDRESS,
// DATABASE_PORT, DATABASE_NAME, DATABASE_USERNAME, DATABASE_PASSWORD).
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/0x3639/znn-sdk-go/abi"
	"github.com/0x3639/znn-sdk-go/embedded"
	"github.com/jackc/pgx/v5/pgxpool"
)

const acceleratorAddress = "z1qxemdeddedxaccelerat0rxxxxxxxxxxp4tk22"

// accountBlockRow holds the fields we need from account_blocks.
type accountBlockRow struct {
	Hash               string
	MomentumHash       string
	MomentumTimestamp  int64
	MomentumHeight     int64
	Address            string
	Data               string // hex-encoded
	Method             string
	PairedAccountBlock string
}

func main() {
	ctx := context.Background()

	connStr := buildConnString()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("failed to ping database: %v", err)
	}
	log.Println("connected to database")

	// Backup the votes table before making changes
	if err := backupVotesTable(); err != nil {
		log.Fatalf("failed to backup votes table: %v", err)
	}

	// Load pillar name -> owner address mapping
	pillarOwners, err := loadPillarOwners(ctx, pool)
	if err != nil {
		log.Fatalf("failed to load pillar owners: %v", err)
	}
	log.Printf("loaded %d pillar name->owner mappings", len(pillarOwners))

	// Query all vote account blocks
	rows, err := pool.Query(ctx, `
		SELECT hash, momentum_hash, momentum_timestamp, momentum_height,
			address, data, method, paired_account_block
		FROM account_blocks
		WHERE to_address = $1
			AND method IN ('VoteByName', 'VoteByProdAddress')
			AND paired_account_block != ''
		ORDER BY momentum_height ASC`,
		acceleratorAddress)
	if err != nil {
		log.Fatalf("failed to query vote blocks: %v", err)
	}
	defer rows.Close()

	var blocks []accountBlockRow
	for rows.Next() {
		var b accountBlockRow
		if err := rows.Scan(&b.Hash, &b.MomentumHash, &b.MomentumTimestamp,
			&b.MomentumHeight, &b.Address, &b.Data, &b.Method, &b.PairedAccountBlock); err != nil {
			log.Fatalf("failed to scan row: %v", err)
		}
		blocks = append(blocks, b)
	}
	if err := rows.Err(); err != nil {
		log.Fatalf("row iteration error: %v", err)
	}
	log.Printf("found %d vote account blocks to re-decode", len(blocks))

	if len(blocks) == 0 {
		log.Println("nothing to do")
		return
	}

	// Re-decode each block and collect new vote records
	type voteRecord struct {
		MomentumHash      string
		MomentumTimestamp int64
		MomentumHeight    int64
		VoterAddress      string
		ProjectID         string
		PhaseID           string
		VotingID          string
		Vote              int16
	}

	var votes []voteRecord
	var decodeErrors int

	for _, b := range blocks {
		data, err := hex.DecodeString(b.Data)
		if err != nil {
			log.Printf("WARN: block %s: invalid hex data: %v", b.Hash, err)
			decodeErrors++
			continue
		}

		decoded := decodeFromAbi(data, embedded.Accelerator)
		if decoded == nil {
			decoded = decodeFromAbi(data, embedded.Common)
		}
		if decoded == nil {
			log.Printf("WARN: block %s: failed to decode ABI data", b.Hash)
			decodeErrors++
			continue
		}

		votingID := decoded["id"]
		voteValueStr := decoded["vote"]
		voteValue, err := strconv.Atoi(voteValueStr)
		if err != nil {
			log.Printf("WARN: block %s: invalid vote value %q: %v", b.Hash, voteValueStr, err)
			voteValue = 0
		}

		if votingID == "" {
			log.Printf("WARN: block %s: empty votingID, skipping", b.Hash)
			decodeErrors++
			continue
		}

		// Resolve voter address
		voterAddress := resolveVoterAddress(b, decoded, pillarOwners)

		// Resolve project/phase IDs
		projectID, phaseID := resolveProjectPhase(ctx, pool, votingID)

		votes = append(votes, voteRecord{
			MomentumHash:      b.MomentumHash,
			MomentumTimestamp: b.MomentumTimestamp,
			MomentumHeight:    b.MomentumHeight,
			VoterAddress:      voterAddress,
			ProjectID:         projectID,
			PhaseID:           phaseID,
			VotingID:          votingID,
			Vote:              int16(voteValue),
		})
	}

	log.Printf("decoded %d votes (%d errors)", len(votes), decodeErrors)

	// Print summary of vote value distribution
	voteCounts := map[int16]int{}
	for _, v := range votes {
		voteCounts[v.Vote]++
	}
	for val, count := range voteCounts {
		label := "Unknown"
		switch val {
		case 0:
			label = "Yes"
		case 1:
			label = "No"
		case 2:
			label = "Abstain"
		}
		log.Printf("  vote=%d (%s): %d", val, label, count)
	}

	// Truncate and re-insert in a transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		log.Fatalf("failed to begin transaction: %v", err)
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	result, err := tx.Exec(ctx, "DELETE FROM votes")
	if err != nil {
		log.Fatalf("failed to delete existing votes: %v", err)
	}
	log.Printf("deleted %d existing vote records", result.RowsAffected())

	inserted := 0
	for _, v := range votes {
		_, err := tx.Exec(ctx, `
			INSERT INTO votes (momentum_hash, momentum_timestamp, momentum_height,
				voter_address, project_id, phase_id, voting_id, vote)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
			v.MomentumHash, v.MomentumTimestamp, v.MomentumHeight,
			v.VoterAddress, v.ProjectID, v.PhaseID, v.VotingID, v.Vote)
		if err != nil {
			log.Printf("WARN: failed to insert vote (votingID=%s, voter=%s): %v",
				v.VotingID, v.VoterAddress, err)
			continue
		}
		inserted++
	}

	if err := tx.Commit(ctx); err != nil {
		log.Fatalf("failed to commit transaction: %v", err)
	}

	log.Printf("done: inserted %d votes", inserted)
}

// backupVotesTable dumps the votes table via Docker pg_dump before we modify it.
func backupVotesTable() error {
	containerName := envOrDefault("POSTGRES_CONTAINER", "nom-indexer-postgres")
	dbName := envOrDefault("DATABASE_NAME", "nom_indexer")
	dbUser := envOrDefault("DATABASE_USERNAME", "postgres")

	// Check if the container is running
	checkCmd := exec.Command("docker", "ps", "--format", "{{.Names}}")
	out, err := checkCmd.Output()
	if err != nil {
		return fmt.Errorf("docker not available or not running: %w", err)
	}
	if !bytes.Contains(out, []byte(containerName)) {
		return fmt.Errorf("container %q is not running — start it with: docker-compose up -d postgres", containerName)
	}

	// Create backup directory
	backupDir := filepath.Join("scripts", "backups")
	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return fmt.Errorf("failed to create backup dir: %w", err)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupFile := filepath.Join(backupDir, fmt.Sprintf("votes_backup_%s.sql.gz", timestamp))

	// Dump only the votes table
	dumpCmd := exec.Command("docker", "exec", containerName,
		"pg_dump", "-U", dbUser, "-d", dbName, "-t", "votes")
	dumpOut, err := dumpCmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}
	dumpCmd.Stderr = os.Stderr

	if err := dumpCmd.Start(); err != nil {
		return fmt.Errorf("failed to start pg_dump: %w", err)
	}

	// Write gzipped output to file
	f, err := os.Create(backupFile)
	if err != nil {
		return fmt.Errorf("failed to create backup file: %w", err)
	}
	defer f.Close()

	gzw := gzip.NewWriter(f)
	if _, err := io.Copy(gzw, dumpOut); err != nil {
		return fmt.Errorf("failed to write backup: %w", err)
	}
	if err := gzw.Close(); err != nil {
		return fmt.Errorf("failed to close gzip writer: %w", err)
	}

	if err := dumpCmd.Wait(); err != nil {
		return fmt.Errorf("pg_dump failed: %w", err)
	}

	info, _ := os.Stat(backupFile)
	log.Printf("backed up votes table to %s (%d bytes)", backupFile, info.Size())
	return nil
}

// buildConnString builds a PostgreSQL connection string from environment variables.
func buildConnString() string {
	host := envOrDefault("DATABASE_ADDRESS", "localhost")
	port := envOrDefault("DATABASE_PORT", "5432")
	name := envOrDefault("DATABASE_NAME", "nom_indexer")
	user := envOrDefault("DATABASE_USERNAME", "postgres")
	password := os.Getenv("DATABASE_PASSWORD")
	if password == "" {
		log.Fatal("DATABASE_PASSWORD environment variable is required")
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		user, password, host, port, name)
}

func envOrDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

// loadPillarOwners loads pillar name -> owner_address from the pillars table.
func loadPillarOwners(ctx context.Context, pool *pgxpool.Pool) (map[string]string, error) {
	rows, err := pool.Query(ctx, `SELECT name, owner_address FROM pillars`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	owners := make(map[string]string)
	for rows.Next() {
		var name, owner string
		if err := rows.Scan(&name, &owner); err != nil {
			return nil, err
		}
		owners[name] = owner
	}
	return owners, rows.Err()
}

// resolveVoterAddress determines the voter address.
// For VoteByName: looks up pillar name -> owner address.
// For VoteByProdAddress: uses the paired account block's address (already stored
// in the address column since the paired block is the sender).
func resolveVoterAddress(b accountBlockRow, decoded map[string]string, pillarOwners map[string]string) string {
	if b.Method == "VoteByName" {
		pillarName := decoded["name"]
		if pillarName != "" {
			if owner, ok := pillarOwners[pillarName]; ok {
				return owner
			}
			log.Printf("WARN: block %s: pillar %q not found in pillars table, using block address", b.Hash, pillarName)
		}
	}
	// For VoteByProdAddress, or fallback: look up the paired account block's address
	return b.Address
}

// resolveProjectPhase looks up project and phase IDs from a voting ID.
func resolveProjectPhase(ctx context.Context, pool *pgxpool.Pool, votingID string) (string, string) {
	// Try project first
	var projectID string
	err := pool.QueryRow(ctx,
		`SELECT id FROM projects WHERE voting_id = $1`, votingID).Scan(&projectID)
	if err == nil && projectID != "" {
		return projectID, ""
	}

	// Try phase
	var phaseID string
	err = pool.QueryRow(ctx,
		`SELECT project_id, id FROM project_phases WHERE voting_id = $1`, votingID).Scan(&projectID, &phaseID)
	if err == nil {
		return projectID, phaseID
	}

	log.Printf("WARN: voting_id %s not found in projects or project_phases", votingID)
	return "", ""
}

// decodeFromAbi decodes ABI-encoded transaction data and returns input name->value pairs.
func decodeFromAbi(data []byte, contractAbi *abi.Abi) map[string]string {
	if contractAbi == nil || len(data) < 4 {
		return nil
	}

	methodSig := data[:4]

	for _, entry := range contractAbi.Entries {
		if entry.Type != abi.Function {
			continue
		}

		entrySig := entry.EncodeSignature()
		if len(entrySig) < 4 {
			continue
		}

		if bytes.Equal(methodSig, entrySig[:4]) {
			result := make(map[string]string)
			result["_method"] = entry.Name

			if len(entry.Inputs) > 0 && len(data) > 4 {
				args, err := contractAbi.DecodeFunction(data)
				if err != nil {
					return nil
				}
				for idx, param := range entry.Inputs {
					if idx < len(args) {
						result[param.Name] = formatArg(args[idx])
					}
				}
			}
			return result
		}
	}
	return nil
}

// formatArg converts a decoded ABI argument to string.
func formatArg(arg interface{}) string {
	switch v := arg.(type) {
	case []byte:
		return string(v)
	case string:
		return v
	case json.Number:
		return v.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmsgprefix)
	log.SetPrefix("[repair-votes] ")

	// Ensure UTC for consistent timestamps
	time.Local = time.UTC
}
