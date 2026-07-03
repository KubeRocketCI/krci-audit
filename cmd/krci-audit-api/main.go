// Command krci-audit-api serves the read-only audit API: initiator lookup + the general
// audit-events query over the audit event store. It connects to PostgreSQL as the
// least-privilege audit_reader role, so the process structurally cannot mutate the trail.
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httplog/v2"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/KubeRocketCI/krci-audit/internal/api"
	"github.com/KubeRocketCI/krci-audit/internal/config"
	"github.com/KubeRocketCI/krci-audit/internal/dsn"
)

func main() {
	logger := initLogger()

	cfg, err := config.LoadAPI()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logger.Info("Starting the krci-audit read API server", "port", cfg.Port)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer stop()

	// pgx.Connect wants the postgres:// scheme; the resolved DSN uses pgx5://.
	pool, err := pgxpool.New(ctx, dsn.ToPostgresScheme(cfg.DSN))
	if err != nil {
		logger.Error("failed to create database pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// Do not hard-fail on the initial ping. The read API must become Ready independently of
	// the database being migrated yet: the audit_reader role and its LOGIN password are
	// provisioned by the migration Job, which runs as a Helm post-install/post-upgrade hook —
	// i.e. only AFTER `--wait` observes this Deployment as Ready. A fatal ping here would
	// deadlock the install (API never Ready → hook never runs → role never created) and would
	// also crash-loop the API on any transient database outage. pgxpool connects lazily, so a
	// warning is enough; the pool authenticates on the first query once the role exists.
	if err = pool.Ping(ctx); err != nil {
		logger.Warn("initial database ping failed; the pool will connect on demand", "error", err)
	}

	r := chi.NewMux()
	r.Use(middleware.RequestID)
	r.Use(httplog.RequestLogger(logger))
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(middleware.Heartbeat("/healthz"))

	server := &http.Server{
		Handler:           api.HandlerFromMux(api.BuildHandler(pool), r),
		Addr:              ":" + cfg.Port,
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	stop() // stop receiving further signals, restoring default behavior on repeat signal

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}
}

func initLogger() *httplog.Logger {
	l := httplog.NewLogger("krci-audit-api", httplog.Options{
		JSON:            true,
		LogLevel:        slog.LevelInfo,
		Concise:         true,
		TimeFieldFormat: time.RFC3339,
		QuietDownRoutes: []string{"/", "/healthz"},
		QuietDownPeriod: 10 * time.Second,
		SourceFieldName: "source",
	})
	slog.SetDefault(l.Logger)
	return l
}
