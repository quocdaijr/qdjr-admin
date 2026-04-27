// apps/api/cmd/api/main.go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/adminapi"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/config"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/db"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(log); err != nil {
		log.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run(log *slog.Logger) error {
	cfg, err := config.LoadFromOS()
	if err != nil {
		return err
	}
	log.Info("config loaded", "env", cfg.Env, "port", cfg.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()

	var verifier auth.Verifier
	switch {
	case cfg.SupabaseJWKSURL != "":
		verifier, err = auth.NewJWKSVerifier(cfg.SupabaseJWKSURL)
		if err != nil {
			return fmt.Errorf("jwks: %w", err)
		}
	default:
		verifier = auth.NewHS256Verifier([]byte(cfg.SupabaseJWTSecret))
	}

	resolver := rbac.NewStatic(rbac.NewPGStore(pool))

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := apphttp.NewRouter(apphttp.RouterDeps{
		Pool:        pool,
		CORSOrigins: cfg.CORSOrigins,
		Verifier:    verifier,
		RegisterAdmin: func(g *gin.RouterGroup) {
			adminapi.Register(g, resolver)
		},
	})

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	errCh := make(chan error, 1)
	go func() {
		log.Info("listening", "addr", srv.Addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	select {
	case err := <-errCh:
		return err
	case <-sigCh:
		log.Info("shutting down")
		sCtx, sCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer sCancel()
		return srv.Shutdown(sCtx)
	}
}
