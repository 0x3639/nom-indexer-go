// phase-outreach generates a prioritized list of pillars to contact for voting
// on a specific AZ phase. Output is printed to stdout and saved as a markdown
// file in the current directory.
//
// Usage:
//
//	DATABASE_PASSWORD=<password> go run scripts/phase-outreach/main.go <phase_id>
//
// It reads the same environment variables as the indexer (DATABASE_ADDRESS,
// DATABASE_PORT, DATABASE_NAME, DATABASE_USERNAME, DATABASE_PASSWORD).
package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type pillarEntry struct {
	Name           string
	Note           string
	ProposalsVoted int
	YesVotes       int
	NoVotes        int
	AbstainVotes   int
}

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: go run scripts/phase-outreach/main.go <phase_id>")
	}
	phaseID := os.Args[1]

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, buildConnString())
	if err != nil {
		log.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	// 1. Look up the phase and its parent project
	var phaseName, projectID, projectName string
	var phaseStatus int
	err = pool.QueryRow(ctx, `
		SELECT pp.name, pp.project_id, p.name, pp.status
		FROM project_phases pp
		JOIN projects p ON p.id = pp.project_id
		WHERE pp.id = $1`, phaseID).Scan(&phaseName, &projectID, &projectName, &phaseStatus)
	if err != nil {
		log.Fatalf("phase %s not found: %v", phaseID, err)
	}

	if phaseStatus != 0 {
		statusLabel := map[int]string{1: "Accepted", 2: "Rejected"}[phaseStatus]
		if statusLabel == "" {
			statusLabel = fmt.Sprintf("Unknown(%d)", phaseStatus)
		}
		log.Fatalf("phase is already %s (status=%d), no outreach needed", statusLabel, phaseStatus)
	}

	// 2. Get current vote counts for this phase
	var totalVotes, yesVotes, noVotes, abstainVotes, uniqueVoters int
	err = pool.QueryRow(ctx, `
		SELECT COUNT(*),
			COUNT(*) FILTER (WHERE vote = 0),
			COUNT(*) FILTER (WHERE vote = 1),
			COUNT(*) FILTER (WHERE vote = 2),
			COUNT(DISTINCT voter_address)
		FROM votes
		WHERE phase_id = $1`, phaseID).Scan(&totalVotes, &yesVotes, &noVotes, &abstainVotes, &uniqueVoters)
	if err != nil {
		log.Fatalf("failed to get vote counts: %v", err)
	}

	// 3. Count active pillars
	var activePillars int
	err = pool.QueryRow(ctx, `SELECT COUNT(*) FROM pillars WHERE is_revoked = false`).Scan(&activePillars)
	if err != nil {
		log.Fatalf("failed to count pillars: %v", err)
	}

	quorum := activePillars / 3
	if quorum == 0 {
		quorum = 1
	}
	votesNeeded := quorum - uniqueVoters
	if votesNeeded < 0 {
		votesNeeded = 0
	}

	// 4. Get pillars who already voted on this phase (for display)
	type phaseVote struct {
		Name string
		Vote string
	}
	var currentVotes []phaseVote
	cvRows, err := pool.Query(ctx, `
		SELECT DISTINCT ON (p.name) p.name,
			CASE v.vote WHEN 0 THEN 'Yes' WHEN 1 THEN 'No' WHEN 2 THEN 'Abstain' END
		FROM votes v
		JOIN pillars p ON p.owner_address = v.voter_address
		WHERE v.phase_id = $1
		ORDER BY p.name`, phaseID)
	if err != nil {
		log.Fatalf("failed to query current votes: %v", err)
	}
	for cvRows.Next() {
		var pv phaseVote
		_ = cvRows.Scan(&pv.Name, &pv.Vote)
		currentVotes = append(currentVotes, pv)
	}
	cvRows.Close()

	// 5. TIER 1: Voted YES on project, haven't voted on this phase
	var tier1 []pillarEntry
	t1Rows, err := pool.Query(ctx, `
		SELECT DISTINCT p.name
		FROM votes v
		JOIN pillars p ON p.owner_address = v.voter_address AND p.is_revoked = false
		WHERE v.project_id = $1 AND v.phase_id = '' AND v.vote = 0
			AND v.voter_address NOT IN (
				SELECT voter_address FROM votes WHERE phase_id = $2
			)
		ORDER BY p.name`, projectID, phaseID)
	if err != nil {
		log.Fatalf("failed to query tier 1: %v", err)
	}
	for t1Rows.Next() {
		var name string
		_ = t1Rows.Scan(&name)
		tier1 = append(tier1, pillarEntry{Name: name, Note: "Voted YES on project"})
	}
	t1Rows.Close()

	// 6. TIER 2: Voted No/Abstain on project, haven't voted on this phase
	var tier2 []pillarEntry
	t2Rows, err := pool.Query(ctx, `
		SELECT DISTINCT p.name,
			CASE v.vote WHEN 1 THEN 'No' WHEN 2 THEN 'Abstain' END
		FROM votes v
		JOIN pillars p ON p.owner_address = v.voter_address AND p.is_revoked = false
		WHERE v.project_id = $1 AND v.phase_id = '' AND v.vote IN (1, 2)
			AND v.voter_address NOT IN (
				SELECT voter_address FROM votes WHERE phase_id = $2
			)
			AND v.voter_address NOT IN (
				SELECT voter_address FROM votes
				WHERE project_id = $1 AND phase_id = '' AND vote = 0
			)
		ORDER BY p.name`, projectID, phaseID)
	if err != nil {
		log.Fatalf("failed to query tier 2: %v", err)
	}
	for t2Rows.Next() {
		var name, projectVote string
		_ = t2Rows.Scan(&name, &projectVote)
		tier2 = append(tier2, pillarEntry{Name: name, Note: fmt.Sprintf("Voted %s on project", projectVote)})
	}
	t2Rows.Close()

	// 7. TIER 3: Active voters who didn't vote on this project or phase
	var tier3 []pillarEntry
	t3Rows, err := pool.Query(ctx, `
		WITH project_voters AS (
			SELECT DISTINCT voter_address FROM votes
			WHERE project_id = $1 AND phase_id = ''
		)
		SELECT p.name,
			COUNT(DISTINCT v.voting_id) as proposals_voted,
			COUNT(*) FILTER (WHERE v.vote = 0) as yes_votes,
			COUNT(*) FILTER (WHERE v.vote = 1) as no_votes,
			COUNT(*) FILTER (WHERE v.vote = 2) as abstain_votes
		FROM votes v
		JOIN pillars p ON p.owner_address = v.voter_address AND p.is_revoked = false
		WHERE v.voter_address NOT IN (SELECT voter_address FROM votes WHERE phase_id = $2)
			AND v.voter_address NOT IN (SELECT voter_address FROM project_voters)
		GROUP BY p.name
		HAVING COUNT(DISTINCT v.voting_id) >= 5
		ORDER BY COUNT(DISTINCT v.voting_id) DESC`, projectID, phaseID)
	if err != nil {
		log.Fatalf("failed to query tier 3: %v", err)
	}
	for t3Rows.Next() {
		var e pillarEntry
		_ = t3Rows.Scan(&e.Name, &e.ProposalsVoted, &e.YesVotes, &e.NoVotes, &e.AbstainVotes)
		tier3 = append(tier3, e)
	}
	t3Rows.Close()

	// --- Write output to both stdout and markdown file ---
	timestamp := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("phase-outreach-%s.md", timestamp)
	f, err := os.Create(filename)
	if err != nil {
		log.Fatalf("failed to create %s: %v", filename, err)
	}
	defer f.Close()

	w := io.MultiWriter(os.Stdout, f)

	fmt.Fprintf(w, "# Phase Outreach: %s\n\n", phaseName)
	fmt.Fprintf(w, "**Project:** %s\n", projectName)
	fmt.Fprintf(w, "**Phase:** %s\n", phaseName)
	fmt.Fprintf(w, "**Phase ID:** `%s`\n", phaseID)
	fmt.Fprintf(w, "**Status:** Pending (not yet approved)\n")
	fmt.Fprintf(w, "**Generated:** %s\n\n", time.Now().UTC().Format("2006-01-02 15:04 UTC"))

	fmt.Fprintf(w, "## Current Votes\n\n")
	fmt.Fprintf(w, "| Metric | Value |\n")
	fmt.Fprintf(w, "|--------|-------|\n")
	fmt.Fprintf(w, "| Phase Votes | %d Yes / %d No / %d Abstain (%d unique voters) |\n",
		yesVotes, noVotes, abstainVotes, uniqueVoters)
	fmt.Fprintf(w, "| Active Pillars | %d |\n", activePillars)
	fmt.Fprintf(w, "| Quorum | %d (1/3 of active pillars) |\n", quorum)
	if votesNeeded > 0 {
		fmt.Fprintf(w, "| **Votes Needed** | **%d more to reach quorum** |\n", votesNeeded)
	} else {
		fmt.Fprintf(w, "| **Votes Needed** | **Quorum reached! (%d >= %d)** |\n", uniqueVoters, quorum)
	}

	fmt.Fprintf(w, "\n### Pillars who already voted\n\n")
	fmt.Fprintf(w, "| Pillar | Vote |\n")
	fmt.Fprintf(w, "|--------|------|\n")
	for _, pv := range currentVotes {
		fmt.Fprintf(w, "| %s | %s |\n", pv.Name, pv.Vote)
	}

	fmt.Fprintf(w, "\n## Tier 1: Voted YES on project, haven't voted on this phase\n\n")
	fmt.Fprintf(w, "> Most likely to vote YES — they already approved the project.\n\n")
	if len(tier1) == 0 {
		fmt.Fprintf(w, "_(none)_\n")
	} else {
		fmt.Fprintf(w, "| # | Pillar | Note |\n")
		fmt.Fprintf(w, "|---|--------|------|\n")
		for i, e := range tier1 {
			fmt.Fprintf(w, "| %d | %s | %s |\n", i+1, e.Name, e.Note)
		}
	}

	fmt.Fprintf(w, "\n## Tier 2: Voted on project (No/Abstain), haven't voted on this phase\n\n")
	fmt.Fprintf(w, "> Engaged with this project but didn't vote Yes.\n\n")
	if len(tier2) == 0 {
		fmt.Fprintf(w, "_(none)_\n")
	} else {
		fmt.Fprintf(w, "| # | Pillar | Note |\n")
		fmt.Fprintf(w, "|---|--------|------|\n")
		for i, e := range tier2 {
			fmt.Fprintf(w, "| %d | %s | %s |\n", i+1, e.Name, e.Note)
		}
	}

	fmt.Fprintf(w, "\n## Tier 3: Active voters who didn't vote on this project or phase\n\n")
	fmt.Fprintf(w, "> Ranked by number of unique proposals voted on (min 5).\n\n")
	if len(tier3) == 0 {
		fmt.Fprintf(w, "_(none with 5+ proposals voted)_\n")
	} else {
		fmt.Fprintf(w, "| # | Pillar | Proposals Voted | Yes | No | Abstain |\n")
		fmt.Fprintf(w, "|---|--------|---------------:|----:|---:|--------:|\n")
		for i, e := range tier3 {
			fmt.Fprintf(w, "| %d | %s | %d | %d | %d | %d |\n",
				i+1, e.Name, e.ProposalsVoted, e.YesVotes, e.NoVotes, e.AbstainVotes)
		}
	}

	fmt.Fprintf(w, "\n## Summary\n\n")
	total := len(tier1) + len(tier2) + len(tier3)
	fmt.Fprintf(w, "- **%d Tier 1** + **%d Tier 2** + **%d Tier 3** = **%d pillars** to contact\n",
		len(tier1), len(tier2), len(tier3), total)
	if votesNeeded > 0 {
		fmt.Fprintf(w, "- **%d more votes** needed for quorum\n", votesNeeded)
	}

	log.Printf("saved to %s", filename)
}

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
