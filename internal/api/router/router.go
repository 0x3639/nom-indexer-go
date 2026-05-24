// Package router composes the chi router and middleware stack for the
// nom-indexer-go HTTP API. Routes are declared once in New() so the
// router_test.go drift check can compare them against docs/api/openapi.yaml.
//
// Hand the router a fully-built Deps struct; New() does not own the
// lifecycle of the pool, repos, or logger.
package router

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/dto"
	"github.com/0x3639/nom-indexer-go/internal/api/handlers"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	apimw "github.com/0x3639/nom-indexer-go/internal/api/middleware"
	"github.com/0x3639/nom-indexer-go/internal/api/stream"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// MetricsMiddleware is an optional Prometheus-style middleware applied
// before any other middleware so its label set includes the chi route
// pattern. Nil disables observation entirely (used by tests).
type MetricsMiddleware func(http.Handler) http.Handler

// Deps bundles everything router.New needs. Build it in cmd/api/main.go;
// tests pass a fake-repos variant.
type Deps struct {
	Repos              *repository.Repositories
	Signer             *auth.Signer
	Logger             *zap.Logger
	Pool               *pgxpool.Pool                  // used by /readyz to ping the DB; may be nil in tests
	Hub                *stream.Hub[*dto.Momentum]     // optional; required for /api/v1/momentums/stream
	TxHub              *stream.Hub[*dto.AccountBlock] // optional; required for /api/v1/transactions/stream
	Metrics            MetricsMiddleware
	CORSAllowedOrigins []string
	RateLimitPerMinute int
	Version            string
	Now                func() time.Time // injected for testability; falls back to time.Now
}

// New builds the chi router with the full middleware stack and route
// table. It is the single source of truth for which paths exist.
func New(d Deps) http.Handler {
	if d.Now == nil {
		d.Now = time.Now
	}
	if d.Version == "" {
		d.Version = "dev"
	}

	r := chi.NewRouter()

	r.Use(apimw.RequestID)
	r.Use(apimw.Logger(d.Logger))
	r.Use(apimw.Recover(d.Logger))
	r.Use(apimw.CORS(d.CORSAllowedOrigins))
	if d.Metrics != nil {
		r.Use(d.Metrics)
	}

	// Unauthenticated routes.
	r.Get("/healthz", healthz)
	r.Get("/readyz", readyz(d.Pool))

	// WebSocket stream — registered at the top level so it bypasses
	// the /api/v1 chi Auth middleware. The handler does its own auth
	// (header OR ?token= query) because browsers can't set custom
	// headers on a WS upgrade. Registered only if a Hub is wired in;
	// otherwise the route returns 503.
	r.Get("/api/v1/momentums/stream", func(w http.ResponseWriter, r *http.Request) {
		if d.Hub == nil {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, "stream_disabled",
				"momentum stream hub is not configured on this instance")
			return
		}
		handlers.MomentumsStream(d.Signer, d.Hub, d.Repos.Momentum)(w, r)
	})
	r.Get("/api/v1/transactions/stream", func(w http.ResponseWriter, r *http.Request) {
		if d.TxHub == nil {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, "stream_disabled",
				"transactions stream hub is not configured on this instance")
			return
		}
		handlers.TransactionsStream(d.Signer, d.TxHub, d.Repos.AccountBlock, d.Repos.Momentum)(w, r)
	})

	// Authenticated /api/v1 subtree.
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(apimw.Auth(d.Signer))
		r.Use(apimw.RateLimit(d.RateLimitPerMinute))

		r.Get("/status", handlers.Status(d.Repos.Momentum, d.Version, d.Now))

		// Flat routes (no Route() subgroup) so chi walks them with the
		// exact paths advertised in openapi.yaml — see router_test.go.
		r.Get("/momentums", handlers.MomentumsList(d.Repos.Momentum))
		r.Get("/momentums/latest", handlers.MomentumsLatest(d.Repos.Momentum))
		r.Get("/momentums/{height}", handlers.MomentumsGetByHeight(d.Repos.Momentum))

		r.Get("/accounts/{address}", handlers.AccountsGet(d.Repos.Account))
		r.Get("/accounts/{address}/balances", handlers.AccountsBalances(d.Repos.Balance))
		r.Get("/accounts/{address}/transactions", handlers.AccountBlocksByAddress(d.Repos.AccountBlock))

		r.Get("/account_blocks", handlers.AccountBlocksList(d.Repos.AccountBlock))
		r.Get("/account_blocks/{hash}", handlers.AccountBlocksGet(d.Repos.AccountBlock))

		r.Get("/tokens", handlers.TokensList(d.Repos.Token))
		r.Get("/tokens/{token_standard}", handlers.TokensGet(d.Repos.Token))
		r.Get("/tokens/{token_standard}/holders", handlers.TokensHolders(d.Repos.Balance))

		r.Get("/pillars", handlers.PillarsList(d.Repos.Pillar))
		r.Get("/pillars/{name}", handlers.PillarsGetByName(d.Repos.Pillar))
		r.Get("/pillars/{name}/delegators", handlers.PillarsDelegators(d.Repos.Pillar))
		r.Get("/pillars/{name}/voting-report", handlers.PillarsVotingHistory(d.Repos.Vote))

		r.Get("/sentinels", handlers.SentinelsList(d.Repos.Sentinel))

		r.Get("/stakes", handlers.StakesList(d.Repos.Stake))
		r.Get("/accounts/{address}/stakes", handlers.StakesByAddress(d.Repos.Stake))

		r.Get("/fusions", handlers.FusionsList(d.Repos.Fusion))
		r.Get("/accounts/{address}/fusions", handlers.FusionsByAddress(d.Repos.Fusion))

		r.Get("/accounts/{address}/rewards", handlers.RewardsHistory(d.Repos.Reward))
		r.Get("/accounts/{address}/rewards/cumulative", handlers.RewardsCumulative(d.Repos.Reward))

		r.Get("/projects", handlers.ProjectsList(d.Repos.Project))
		r.Get("/projects/{id}", handlers.ProjectsGet(d.Repos.Project))
		r.Get("/projects/{id}/phases", handlers.ProjectsPhases(d.Repos.ProjectPhase))
		r.Get("/projects/{id}/votes", handlers.ProjectsVotes(d.Repos.Vote))
		r.Get("/projects/{id}/voting-report", handlers.ProjectsVotingReport(d.Repos.Vote))

		r.Get("/bridge/wraps", handlers.BridgeWraps(d.Repos.Bridge))
		r.Get("/bridge/unwraps", handlers.BridgeUnwraps(d.Repos.Bridge))
		r.Get("/accounts/{address}/bridge/wraps", handlers.BridgeWrapsByAddress(d.Repos.Bridge))
		r.Get("/accounts/{address}/bridge/unwraps", handlers.BridgeUnwrapsByAddress(d.Repos.Bridge))
	})

	return r
}

func healthz(w http.ResponseWriter, _ *http.Request) {
	httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// minSchemaVersion is the lowest golang-migrate version the API can serve
// against. Bump this in the same PR that adds a new migrations/NNN_*.sql
// file IF the new migration touches a table the API reads. Failing to
// bump means /readyz reports ready against an under-migrated DB and
// some /api/v1/* endpoint will 500 on a missing table; bumping too
// aggressively (i.e. before the migration actually ships in operators'
// indexer image) means /readyz stays 503 after a deploy. Today the API
// reads bridge/config/delegation/stat-history tables added through 011.
const minSchemaVersion = 11

// readyz verifies the database is reachable AND that the indexer schema
// has been migrated far enough for every endpoint to serve. A bare Ping
// is insufficient: a fresh container would report healthy while every
// /api/v1/* call 500s, and a DB stuck on an early migration would 500
// on tables added later (bridge_config, daily_stat_histories, etc.).
//
// We query golang-migrate's schema_migrations table (one row, version
// + dirty) and require version >= minSchemaVersion AND dirty=false.
// schema_migrations missing → migrations never ran. dirty=true → the
// last migration failed mid-run and someone must intervene.
//
// A nil pool (test mode) is treated as ready since there's nothing to
// verify.
func readyz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, "db_unavailable", err.Error())
			return
		}

		var (
			version int64
			dirty   bool
		)
		err := pool.QueryRow(ctx,
			`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
		if err != nil {
			// Most likely: schema_migrations relation does not exist
			// because migrations never ran. Surface a stable code so
			// operators can grep for it without parsing pg messages.
			if strings.Contains(err.Error(), "schema_migrations") {
				httpx.WriteProblem(w, http.StatusServiceUnavailable, "schema_not_migrated",
					"schema_migrations table missing — start the indexer container so migrations run")
				return
			}
			httpx.WriteProblem(w, http.StatusServiceUnavailable, "db_unavailable", err.Error())
			return
		}
		if dirty {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, "schema_dirty",
				fmt.Sprintf("schema_migrations marks version %d dirty — manual repair needed", version))
			return
		}
		if version < minSchemaVersion {
			httpx.WriteProblem(w, http.StatusServiceUnavailable, "schema_not_migrated",
				fmt.Sprintf("schema_migrations.version = %d, API requires >= %d",
					version, minSchemaVersion))
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}
