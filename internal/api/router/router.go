// Package router composes the chi router and middleware stack for the
// nom-indexer-go HTTP API. Routes are declared once in New() so the
// router_test.go drift check can compare them against docs/api/openapi.yaml.
//
// Hand the router a fully-built Deps struct; New() does not own the
// lifecycle of the pool, repos, or logger.
package router

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/api/handlers"
	"github.com/0x3639/nom-indexer-go/internal/api/httpx"
	apimw "github.com/0x3639/nom-indexer-go/internal/api/middleware"
	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// Deps bundles everything router.New needs. Build it in cmd/api/main.go;
// tests pass a fake-repos variant.
type Deps struct {
	Repos              *repository.Repositories
	Signer             *auth.Signer
	Logger             *zap.Logger
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

	// Unauthenticated routes.
	r.Get("/healthz", healthz)

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
