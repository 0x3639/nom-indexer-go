package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// minSchemaVersion is the lowest golang-migrate version the MCP server
// can serve against. Mirrors the REST API's gate (router.minSchemaVersion)
// because both processes read the same tables. Bump this in the same PR
// that adds a migration the MCP server depends on.
const minSchemaVersion = 11

// Healthz reports that the process is alive. Always 200; no DB ping.
// Use as the k8s liveness probe.
func Healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// Readyz verifies the database is reachable AND the indexer schema has
// migrated far enough for every tool to succeed. Mirrors the REST API's
// /readyz semantics exactly — same minSchemaVersion gate, same error
// codes — so an operator's existing alerting on either service applies.
//
// A nil pool (test mode) is treated as ready.
func Readyz(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			writeProblem(w, http.StatusServiceUnavailable, "db_unavailable", err.Error())
			return
		}

		var (
			version int64
			dirty   bool
		)
		err := pool.QueryRow(ctx,
			`SELECT version, dirty FROM schema_migrations LIMIT 1`).Scan(&version, &dirty)
		if err != nil {
			if strings.Contains(err.Error(), "schema_migrations") {
				writeProblem(w, http.StatusServiceUnavailable, "schema_not_migrated",
					"schema_migrations table missing — start the indexer container so migrations run")
				return
			}
			writeProblem(w, http.StatusServiceUnavailable, "db_unavailable", err.Error())
			return
		}
		if dirty {
			writeProblem(w, http.StatusServiceUnavailable, "schema_dirty",
				fmt.Sprintf("schema_migrations marks version %d dirty — manual repair needed", version))
			return
		}
		if version < minSchemaVersion {
			writeProblem(w, http.StatusServiceUnavailable, "schema_not_migrated",
				fmt.Sprintf("schema_migrations.version = %d, MCP requires >= %d",
					version, minSchemaVersion))
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

// writeProblem emits a small {error, message} body alongside the HTTP
// status. We do not pull in the API's httpx.WriteProblem to keep the
// MCP server independent of the REST package.
func writeProblem(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, map[string]string{"error": code, "message": message})
}
