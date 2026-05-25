package repository

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/0x3639/nom-indexer-go/internal/models"
)

type VoteRepository struct {
	pool *pgxpool.Pool
}

func NewVoteRepository(pool *pgxpool.Pool) *VoteRepository {
	return &VoteRepository{pool: pool}
}

// Insert inserts or updates a vote (keeps the latest vote per voter+voting_id)
func (r *VoteRepository) Insert(ctx context.Context, v *models.Vote) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO votes (momentum_hash, momentum_timestamp, momentum_height,
			voter_address, project_id, phase_id, voting_id, vote)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (voter_address, voting_id) DO UPDATE SET
			momentum_hash = EXCLUDED.momentum_hash,
			momentum_timestamp = EXCLUDED.momentum_timestamp,
			momentum_height = EXCLUDED.momentum_height,
			vote = EXCLUDED.vote`,
		v.MomentumHash, v.MomentumTimestamp, v.MomentumHeight,
		v.VoterAddress, v.ProjectID, v.PhaseID, v.VotingID, v.Vote)
	return err
}

// InsertBatch adds a vote upsert to a batch (keeps the latest vote per voter+voting_id)
func (r *VoteRepository) InsertBatch(batch *pgx.Batch, v *models.Vote) {
	batch.Queue(`
		INSERT INTO votes (momentum_hash, momentum_timestamp, momentum_height,
			voter_address, project_id, phase_id, voting_id, vote)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (voter_address, voting_id) DO UPDATE SET
			momentum_hash = EXCLUDED.momentum_hash,
			momentum_timestamp = EXCLUDED.momentum_timestamp,
			momentum_height = EXCLUDED.momentum_height,
			vote = EXCLUDED.vote`,
		v.MomentumHash, v.MomentumTimestamp, v.MomentumHeight,
		v.VoterAddress, v.ProjectID, v.PhaseID, v.VotingID, v.Vote)
}

// GetVoteCountForProjects counts distinct projects voted on by a voter
func (r *VoteRepository) GetVoteCountForProjects(ctx context.Context, voterAddress string, projectIDs []string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT project_id) FROM votes
		WHERE voter_address = $1 AND project_id = ANY($2) AND phase_id = ''`,
		voterAddress, projectIDs).Scan(&count)
	return count, err
}

// ListByProject returns votes targeting either the project directly or
// any of its phases, ordered by momentum_height descending.
func (r *VoteRepository) ListByProject(ctx context.Context, projectID string, opts ListOpts) ([]*models.Vote, int64, error) {
	if projectID == "" {
		return nil, 0, fmt.Errorf("project_id is required")
	}
	rows, err := r.pool.Query(ctx, `
		SELECT id, momentum_hash, momentum_timestamp, momentum_height,
			voter_address, project_id, phase_id, voting_id, vote,
			COUNT(*) OVER () AS total
		FROM votes WHERE project_id = $1
		ORDER BY momentum_height DESC
		LIMIT $2 OFFSET $3`, projectID, opts.Limit, opts.Offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var (
		out   []*models.Vote
		total int64
	)
	for rows.Next() {
		var v models.Vote
		if err := rows.Scan(&v.ID, &v.MomentumHash, &v.MomentumTimestamp, &v.MomentumHeight,
			&v.VoterAddress, &v.ProjectID, &v.PhaseID, &v.VotingID, &v.Vote, &total); err != nil {
			return nil, 0, err
		}
		out = append(out, &v)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	if len(out) == 0 && opts.Offset > 0 {
		var err error
		total, err = fallbackCount(ctx, r.pool,
			`SELECT COUNT(*) FROM votes WHERE project_id = $1`, projectID)
		if err != nil {
			return nil, 0, err
		}
	}
	return out, total, nil
}

// ProjectVotingReport returns one server-aggregated report covering a
// project's own vote AND every phase's vote, with the active-pillar
// set as the denominator. The caller gets a complete snapshot in one
// trip — no enumerating pillars × proposals client-side.
//
// Composed from four small queries (project, phases, active pillars,
// votes for the relevant voting_ids) which are split into helpers so
// each one's error handling stays local. The query pattern is
// bounded: |proposals| <= 1 + |phases| (typically <10), |active
// pillars| ~70, |votes| <= |proposals| × |pillars|.
func (r *VoteRepository) ProjectVotingReport(ctx context.Context, projectID string) (*ProjectVotingReportRow, error) {
	if projectID == "" {
		return nil, fmt.Errorf("project_id is required")
	}

	projectName, projectVotingID, err := r.loadProjectIdentity(ctx, projectID)
	if err != nil {
		return nil, err
	}
	phases, err := r.loadPhaseMetas(ctx, projectID)
	if err != nil {
		return nil, err
	}
	pillarNames, err := r.loadActivePillarNames(ctx)
	if err != nil {
		return nil, err
	}

	votingIDs := make([]string, 0, 1+len(phases))
	votingIDs = append(votingIDs, projectVotingID)
	for _, p := range phases {
		votingIDs = append(votingIDs, p.votingID)
	}
	votesByVotingID, err := r.loadVotesByVotingID(ctx, votingIDs)
	if err != nil {
		return nil, err
	}

	out := &ProjectVotingReportRow{
		ProjectID:         projectID,
		ProjectName:       projectName,
		ActivePillarCount: len(pillarNames),
		Project: ProposalTallyRow{
			VotingID: projectVotingID,
			ByPillar: votesByVotingID[projectVotingID],
		},
	}
	out.Project.SetNoVote(pillarNames)
	for _, p := range phases {
		t := ProposalTallyRow{
			VotingID: p.votingID,
			ByPillar: votesByVotingID[p.votingID],
		}
		t.SetNoVote(pillarNames)
		out.Phases = append(out.Phases, PhaseTallyRow{
			PhaseID:   p.id,
			PhaseName: p.name,
			Tally:     t,
		})
	}
	return out, nil
}

// phaseMeta is the minimum data ProjectVotingReport needs about each
// phase. Internal to this file; the public PhaseTallyRow carries the
// same identity fields plus the tally.
type phaseMeta struct {
	id, votingID, name string
}

func (r *VoteRepository) loadProjectIdentity(ctx context.Context, projectID string) (name, votingID string, err error) {
	err = r.pool.QueryRow(ctx,
		`SELECT name, voting_id FROM projects WHERE id = $1`, projectID,
	).Scan(&name, &votingID)
	return name, votingID, err
}

func (r *VoteRepository) loadPhaseMetas(ctx context.Context, projectID string) ([]phaseMeta, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, voting_id, name
		   FROM project_phases
		  WHERE project_id = $1
		  ORDER BY creation_timestamp ASC`, projectID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var phases []phaseMeta
	for rows.Next() {
		var p phaseMeta
		if err := rows.Scan(&p.id, &p.votingID, &p.name); err != nil {
			return nil, err
		}
		phases = append(phases, p)
	}
	return phases, rows.Err()
}

// loadActivePillarNames returns the ordered name list used as the
// denominator for no-vote calculations.
func (r *VoteRepository) loadActivePillarNames(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT name
		   FROM pillars
		  WHERE is_revoked = false
		  ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pillarNames []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		pillarNames = append(pillarNames, name)
	}
	return pillarNames, rows.Err()
}

// loadVotesByVotingID fetches every vote covering the given voting_ids
// and groups them by voting_id → active pillar name → vote code.
//
// VoteByName rows store owner_address. VoteByProdAddress rows store a
// producer address, so the query resolves those through the historical
// pillar_updates row at the vote height, with a current-producer fallback
// for old data that predates the update log.
func (r *VoteRepository) loadVotesByVotingID(ctx context.Context, votingIDs []string) (map[string]map[string]int16, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT v.voting_id,
		       COALESCE(owner_p.name, hist_p.name, current_producer_p.name) AS pillar_name,
		       v.vote
		  FROM votes v
		  LEFT JOIN pillars owner_p
		    ON owner_p.owner_address = v.voter_address
		   AND owner_p.is_revoked = false
		  LEFT JOIN LATERAL (
		      SELECT pu.owner_address
		        FROM pillar_updates pu
		       WHERE pu.producer_address = v.voter_address
		         AND pu.momentum_height <= v.momentum_height
		       ORDER BY pu.id DESC
		       LIMIT 1
		  ) hist ON owner_p.owner_address IS NULL
		  LEFT JOIN pillars hist_p
		    ON hist_p.owner_address = hist.owner_address
		   AND hist_p.is_revoked = false
		  LEFT JOIN LATERAL (
		      SELECT p.name
		        FROM pillars p
		       WHERE p.producer_address = v.voter_address
		         AND p.is_revoked = false
		       ORDER BY p.rank ASC
		       LIMIT 1
		  ) current_producer_p ON owner_p.owner_address IS NULL
		                      AND hist.owner_address IS NULL
		 WHERE v.voting_id = ANY($1)
		   AND COALESCE(owner_p.name, hist_p.name, current_producer_p.name) IS NOT NULL`,
		votingIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]map[string]int16, len(votingIDs))
	for _, vid := range votingIDs {
		out[vid] = make(map[string]int16)
	}
	for rows.Next() {
		var (
			votingID string
			name     string
			vote     int16
		)
		if err := rows.Scan(&votingID, &name, &vote); err != nil {
			return nil, err
		}
		out[votingID][name] = vote
	}
	return out, rows.Err()
}

// PillarVotingHistory returns one named pillar's complete voting record
// with project + phase names joined server-side. Resolves the pillar
// name to its owner_address first (one row), then issues the joined
// query — two trips, one bounded by the pillar's lifetime vote count
// (a few hundred at most for the most-active pillars).
func (r *VoteRepository) PillarVotingHistory(ctx context.Context, pillarName string) (*PillarVotingHistoryRow, error) {
	if pillarName == "" {
		return nil, fmt.Errorf("pillar name is required")
	}

	var owner, currentProducer string
	if err := r.pool.QueryRow(ctx,
		`SELECT owner_address, producer_address FROM pillars WHERE name = $1`, pillarName,
	).Scan(&owner, &currentProducer); err != nil {
		return nil, err
	}

	rows, err := r.pool.Query(ctx, `
		SELECT v.voting_id, v.vote, v.momentum_height, v.momentum_timestamp,
		       v.project_id, v.phase_id,
		       COALESCE(p.name, '')  AS project_name,
		       COALESCE(pp.name, '') AS phase_name
		  FROM votes v
		  LEFT JOIN projects p        ON p.id  = v.project_id
		  LEFT JOIN project_phases pp ON pp.id = v.phase_id
		 WHERE v.voter_address = $1
		    OR $1 = (
		       SELECT pu.owner_address
		         FROM pillar_updates pu
		        WHERE pu.producer_address = v.voter_address
		          AND pu.momentum_height <= v.momentum_height
		        ORDER BY pu.id DESC
		        LIMIT 1
		    )
		    OR (
		       v.voter_address = $2
		       AND NOT EXISTS (
		           SELECT 1
		             FROM pillar_updates pu
		            WHERE pu.producer_address = v.voter_address
		              AND pu.momentum_height <= v.momentum_height
		       )
		    )
		 ORDER BY v.momentum_timestamp DESC`, owner, currentProducer)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := &PillarVotingHistoryRow{
		PillarName:  pillarName,
		PillarOwner: owner,
	}
	for rows.Next() {
		var e PillarVoteRow
		if err := rows.Scan(&e.VotingID, &e.Vote, &e.MomentumHeight, &e.MomentumTimestamp,
			&e.ProjectID, &e.PhaseID, &e.ProjectName, &e.PhaseName); err != nil {
			return nil, err
		}
		out.Votes = append(out.Votes, e)
	}
	return out, rows.Err()
}

// ProjectVotingReportRow / PillarVotingHistoryRow are the raw row
// shapes returned by this repository — the api/dto package wraps them
// into the public DTOs. Keeping the raw types here lets tests build
// fixtures without importing dto, and lets the dto wrappers do the
// int16 → string vote translation in one place.
type ProjectVotingReportRow struct {
	ProjectID         string
	ProjectName       string
	ActivePillarCount int
	Project           ProposalTallyRow
	Phases            []PhaseTallyRow
}

type ProposalTallyRow struct {
	VotingID      string
	ByPillar      map[string]int16
	NoVotePillars []string
}

// SetNoVote populates NoVotePillars from the full active-pillar list
// minus any name found in ByPillar.
func (t *ProposalTallyRow) SetNoVote(activePillars []string) {
	for _, name := range activePillars {
		if _, voted := t.ByPillar[name]; !voted {
			t.NoVotePillars = append(t.NoVotePillars, name)
		}
	}
}

type PhaseTallyRow struct {
	PhaseID   string
	PhaseName string
	Tally     ProposalTallyRow
}

type PillarVotingHistoryRow struct {
	PillarName  string
	PillarOwner string
	Votes       []PillarVoteRow
}

type PillarVoteRow struct {
	VotingID          string
	Vote              int16
	MomentumHeight    int64
	MomentumTimestamp int64
	ProjectID         string
	PhaseID           string
	ProjectName       string
	PhaseName         string
}

// GetVoteCountForPhases counts distinct phases voted on by a voter
func (r *VoteRepository) GetVoteCountForPhases(ctx context.Context, voterAddress string, phaseIDs []string) (int, error) {
	var count int
	err := r.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT phase_id) FROM votes
		WHERE voter_address = $1 AND phase_id = ANY($2)`,
		voterAddress, phaseIDs).Scan(&count)
	return count, err
}
