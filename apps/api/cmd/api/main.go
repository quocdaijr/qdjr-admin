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
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/categories"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/config"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/contact"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/db"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/profile"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/sitesettings"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/tags"
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

	// Public posts API: prefix media storage paths with the Supabase public
	// storage URL so the FE can render thumbnails directly.
	storagePrefix := cfg.SupabaseURL + "/storage/v1/object/public/"
	postsRepo := posts.NewRepository(pool, storagePrefix)
	postsAdminRepo := posts.NewAdminRepository(pool)
	categoriesRepo := categories.NewRepository(pool)
	categoriesAdminRepo := categories.NewAdminRepository(pool)
	tagsRepo := tags.NewRepository(pool)
	tagsAdminRepo := tags.NewAdminRepository(pool)
	profileRepo := profile.NewRepository(pool, storagePrefix)
	profileAdminRepo := profile.NewAdminRepository(pool, storagePrefix)
	siteRepo := sitesettings.NewRepository(pool)
	siteAdminRepo := sitesettings.NewAdminRepository(pool)
	contactRepo := contact.NewRepository(pool)
	contactLimiter := contact.NewLimiter()
	// Background eviction lives with the process; pass context.Background()
	// because RegisterPublic doesn't get the run-loop context.
	go contactLimiter.StartEvictor(context.Background())

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := apphttp.NewRouter(apphttp.RouterDeps{
		Pool:        pool,
		CORSOrigins: cfg.CORSOrigins,
		Verifier:    verifier,
		RegisterAdmin: func(g *gin.RouterGroup) {
			adminapi.Register(g, adminapi.Deps{
				Resolver:        resolver,
				PostsAdmin:      postsAdminRepo,
				CategoriesAdmin: categoriesAdminRepo,
				TagsAdmin:       tagsAdminRepo,
				ProfileAdmin:    profileAdminRepo,
				SettingsAdmin:   siteAdminRepo,
			})
		},
		RegisterPublic: func(g *gin.RouterGroup) {
			posts.RegisterPublic(g, postsRepo)
			categories.RegisterPublic(g, categoriesRepo, postsRepo)
			tags.RegisterPublic(g, tagsRepo, postsRepo)
			profile.RegisterPublic(g, profileRepo)
			sitesettings.RegisterPublic(g, siteRepo)
			contact.RegisterPublic(g, contactRepo, contactLimiter)
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
