// Package httpapi wires the HTTP routes for the server role.
package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/RamiroCuenca/eks-platform-demo-app/internal/cache"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/db"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/obs"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type api struct {
	db     *db.DB
	cache  *cache.Cache
	logger *slog.Logger
}

// NewRouter returns the fully wired HTTP handler, instrumented with Prometheus metrics.
func NewRouter(database *db.DB, c *cache.Cache, logger *slog.Logger) http.Handler {
	a := &api{db: database, cache: c, logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", a.healthz)
	mux.HandleFunc("GET /readyz", a.readyz)
	mux.HandleFunc("GET /db", a.dbRead)
	mux.HandleFunc("GET /cache", a.cacheRoundTrip)
	mux.HandleFunc("POST /enqueue", a.enqueue)
	mux.Handle("GET /metrics", promhttp.Handler())

	return obs.Instrument(mux)
}

// healthz is a liveness probe: the process is up. It does not touch the data tier.
func (a *api) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// readyz is a readiness probe: reports 503 unless both Aurora and Redis are reachable.
func (a *api) readyz(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()

	status := map[string]string{"db": "ok", "redis": "ok"}
	code := http.StatusOK
	if err := a.db.Ping(ctx); err != nil {
		status["db"] = "unavailable"
		code = http.StatusServiceUnavailable
	}
	if err := a.cache.Ping(ctx); err != nil {
		status["redis"] = "unavailable"
		code = http.StatusServiceUnavailable
	}
	writeJSON(w, code, status)
}

func (a *api) dbRead(w http.ResponseWriter, r *http.Request) {
	v, err := a.db.Version(r.Context())
	if err != nil {
		a.logger.Error("db read", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "database unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"source": "aurora-postgresql", "version": v})
}

func (a *api) cacheRoundTrip(w http.ResponseWriter, r *http.Request) {
	val := time.Now().UTC().Format(time.RFC3339Nano)
	got, err := a.cache.RoundTrip(r.Context(), "demo:last-seen", val)
	if err != nil {
		a.logger.Error("cache round-trip", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cache unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"source": "elasticache-redis", "value": got})
}

func (a *api) enqueue(w http.ResponseWriter, r *http.Request) {
	n, err := a.cache.Enqueue(r.Context(), time.Now().UTC().Format(time.RFC3339Nano))
	if err != nil {
		a.logger.Error("enqueue", "err", err)
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "cache unavailable"})
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]int64{"queue_length": n})
}

func writeJSON(w http.ResponseWriter, code int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}
