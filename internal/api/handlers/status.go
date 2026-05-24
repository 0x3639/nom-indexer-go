package handlers

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	"github.com/0x3639/nom-indexer-go/internal/models"
)

// statusMomentumRepo is the read surface the status handler needs from
// the momentum repository. Defined as an interface so handler tests can
// pass a fake without spinning up Postgres.
type statusMomentumRepo interface {
	GetLatest(ctx context.Context) (*models.Momentum, error)
}

// Status returns the indexer's current sync state. now() is injected so
// tests can pin time; production passes time.Now.
func Status(repo statusMomentumRepo, version string, now func() time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m, err := repo.GetLatest(r.Context())
		if errors.Is(err, pgx.ErrNoRows) {
			// No momentums yet — return zeroes rather than 404; the
			// service is alive, it just has nothing to report.
			httpx.WriteJSON(w, http.StatusOK, &dto.Status{Version: version})
			return
		}
		if err != nil {
			writeRepoError(w, err)
			return
		}
		lag := now().Unix() - m.Timestamp
		if lag < 0 {
			lag = 0
		}
		httpx.WriteJSON(w, http.StatusOK, &dto.Status{
			LatestHeight:      m.Height,
			LatestTimestamp:   m.Timestamp,
			IndexerLagSeconds: lag,
			Version:           version,
		})
	}
}
