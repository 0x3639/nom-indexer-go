package router

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"sort"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"go.uber.org/zap"

	"github.com/0x3639/nom-indexer-go/internal/auth"
	"github.com/0x3639/nom-indexer-go/internal/models"
	"github.com/0x3639/nom-indexer-go/internal/repository"
)

// TestRouterMatchesOpenAPISpec asserts that every path documented in
// docs/api/openapi.yaml is registered on the chi router, and vice
// versa. This is the drift-detection backstop the plan calls for —
// without it, the spec and the implementation can diverge silently.
//
// When this test fails, either:
//   - the new endpoint hasn't been wired into router.New (add it there), or
//   - the new endpoint is missing from docs/api/openapi.yaml (add a path
//     stanza with operationId, parameters, and responses).
func TestRouterMatchesOpenAPISpec(t *testing.T) {
	spec := loadSpec(t)
	router := newTestRouter(t)

	specPaths := normalizePaths(specPathsOf(spec))
	routerPaths := normalizePaths(routerPathsOf(t, router))

	specSet := toSet(specPaths)
	routerSet := toSet(routerPaths)

	for _, p := range specPaths {
		if !routerSet[p] {
			t.Errorf("openapi path %q has no matching route in router.New", p)
		}
	}
	for _, p := range routerPaths {
		if !specSet[p] {
			t.Errorf("router path %q is not documented in docs/api/openapi.yaml", p)
		}
	}
}

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	_, thisFile, _, _ := runtime.Caller(0)
	specPath := filepath.Join(filepath.Dir(thisFile), "..", "..", "..", "docs", "api", "openapi.yaml")
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile(specPath)
	if err != nil {
		t.Fatalf("loading openapi.yaml at %s: %v", specPath, err)
	}
	return doc
}

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	signer, err := auth.NewSigner("test-secret-32-bytes-or-longer-okok")
	if err != nil {
		t.Fatalf("auth.NewSigner: %v", err)
	}
	return New(Deps{
		Repos:              &repository.Repositories{}, // handlers aren't called; nil fields are OK
		Signer:             signer,
		Logger:             zap.NewNop(),
		RateLimitPerMinute: 60,
		Version:            "test",
		Now:                func() time.Time { return time.Unix(0, 0) },
	})
}

func specPathsOf(doc *openapi3.T) []string {
	var out []string
	for path, item := range doc.Paths.Map() {
		if item == nil {
			continue
		}
		// Only consider paths that expose at least one GET — those are the
		// ones we actually serve today.
		if item.Get != nil {
			out = append(out, path)
		}
	}
	return out
}

func routerPathsOf(t *testing.T, h http.Handler) []string {
	t.Helper()
	r, ok := h.(chi.Routes)
	if !ok {
		t.Fatalf("router does not implement chi.Routes")
	}
	var out []string
	walkErr := chi.Walk(r, func(method string, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if method != http.MethodGet {
			return nil
		}
		out = append(out, route)
		return nil
	})
	if walkErr != nil {
		t.Fatalf("chi.Walk: %v", walkErr)
	}
	return out
}

// normalizePaths strips trailing slashes (other than "/" itself) so a
// route registered via r.Route("/x", …) compares equal to a spec path
// written as "/x".
func normalizePaths(in []string) []string {
	out := make([]string, 0, len(in))
	for _, p := range in {
		if len(p) > 1 && p[len(p)-1] == '/' {
			p = p[:len(p)-1]
		}
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

func toSet(in []string) map[string]bool {
	out := make(map[string]bool, len(in))
	for _, s := range in {
		out[s] = true
	}
	return out
}

// stubSyncStatus is a syncStatusGetter for tests that lets each test pick
// the response or error to return without touching Postgres.
type stubSyncStatus struct {
	row *models.SyncStatus
	err error
}

func (s *stubSyncStatus) Get(_ context.Context) (*models.SyncStatus, error) {
	return s.row, s.err
}

// TestReadyzNilPoolReturnsOK exercises the short-circuit branch used by
// the test router: with no pool, readyz responds 200 without consulting
// the sync getter (so a nil getter is also safe here).
func TestReadyzNilPoolReturnsOK(t *testing.T) {
	h := readyz(nil, nil)
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("readyz code = %d, want 200", rr.Code)
	}
}

// TestReadyzNilPoolWithStubGetterIgnored confirms the pool == nil branch
// returns ready even when a stub sync getter is wired in — the pool gate
// runs first.
func TestReadyzNilPoolWithStubGetterIgnored(t *testing.T) {
	h := readyz(nil, &stubSyncStatus{err: pgx.ErrNoRows})
	rr := httptest.NewRecorder()
	h(rr, httptest.NewRequest(http.MethodGet, "/readyz", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("readyz code = %d, want 200", rr.Code)
	}
}

// TestReadyzDriftBranchCoverage is deferred to the watchdog integration
// suite (T18). Exercising the drift branch requires a live pool that
// passes Ping and schema_migrations checks AND a stubbed sync_status
// row, which is most ergonomic against a real Postgres harness.
func TestReadyzDriftBranchCoverage(t *testing.T) {
	t.Skip("requires non-nil pool with successful Ping AND a stub sync status — defer to T18 integration coverage")
}
