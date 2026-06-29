// Command server runs the demo platform service. A single binary serves two roles,
// selected by APP_MODE: an HTTP API (server) that exercises Aurora + ElastiCache, and a
// queue worker (worker) that drains a Redis list so KEDA has a backlog to scale on.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/RamiroCuenca/eks-platform-demo-app/internal/cache"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/config"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/db"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/httpapi"
	"github.com/RamiroCuenca/eks-platform-demo-app/internal/worker"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}

	// SIGINT/SIGTERM cancel the root context so both roles shut down gracefully
	// (Kubernetes sends SIGTERM before the grace period elapses).
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	switch cfg.Mode {
	case config.ModeWorker:
		runWorker(ctx, cfg, logger)
	default:
		runServer(ctx, cfg, logger)
	}
}

func runServer(ctx context.Context, cfg config.Config, logger *slog.Logger) {
	// Pools are lazy: construction never blocks on the data tier, so a transient
	// Aurora/Redis outage degrades /db and /cache to 503 rather than crash-looping the pod.
	database, err := db.New(ctx, cfg.DB)
	if err != nil {
		logger.Error("init db", "err", err)
		os.Exit(1)
	}
	defer database.Close()

	c, err := cache.New(cfg.Redis)
	if err != nil {
		logger.Error("init cache", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           httpapi.NewRouter(database, c, logger),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		logger.Info("http server listening", "addr", srv.Addr, "mode", string(cfg.Mode))
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "err", err)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown", "err", err)
	}
}

func runWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) {
	c, err := cache.New(cfg.Redis)
	if err != nil {
		logger.Error("init cache", "err", err)
		os.Exit(1)
	}
	defer c.Close()

	// A minimal server exposes /metrics and /healthz so the worker is scrapeable and probeable.
	mux := http.NewServeMux()
	mux.Handle("GET /metrics", promhttp.Handler())
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("worker metrics server", "err", err)
		}
	}()

	worker.Run(ctx, c, logger)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(shutdownCtx)
}
