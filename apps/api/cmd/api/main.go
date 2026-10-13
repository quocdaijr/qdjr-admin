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
	"golang.org/x/time/rate"

	"github.com/quocdaijr/qdjr-admin/apps/api/internal/adminapi"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/auth"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/categories"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/config"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/contact"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/db"
	apphttp "github.com/quocdaijr/qdjr-admin/apps/api/internal/http"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/lessons"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/media"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/migrate"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/posts"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/profile"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/rbac"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/session"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/sitesettings"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/tags"
	"github.com/quocdaijr/qdjr-admin/apps/api/internal/users"
)

func main() {
	if err := run(); err != nil {
		// Bootstrap logger for fatal errors that occur before run() builds its own.
		bootstrap := slog.New(slog.NewJSONHandler(os.Stdout, nil))
		bootstrap.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// newLogger returns a JSON slog.Logger whose level reflects the runtime
// environment: INFO in production (filters debug noise), DEBUG elsewhere.
func newLogger(env string) *slog.Logger {
	level := slog.LevelDebug
	if env == "production" {
		level = slog.LevelInfo
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func run() error {
	cfg, err := config.LoadFromOS()
	if err != nil {
		return err
	}
	log := newLogger(cfg.Env)
	log.Info("config loaded", "env", cfg.Env, "port", cfg.Port)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.RunMigrations {
		if err := db.Retry(ctx, 5, time.Second, func(ctx context.Context) error {
			return migrate.Up(ctx, cfg.MigrationsDatabaseURL)
		}); err != nil {
			return fmt.Errorf("migrate: %w", err)
		}
		log.Info("migrations applied")
	}

	pool, err := db.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("db: %w", err)
	}
	defer pool.Close()

	issuer := auth.NewIssuer([]byte(cfg.JWTSecret), cfg.AccessTTL, cfg.RefreshTTL)

	resolver := rbac.NewStatic(rbac.NewPGStore(pool))

	// Media keys ("media/<uuid>.png") become public URLs under
	// MEDIA_PUBLIC_BASE_URL (R2 r2.dev / custom domain, or the local folder).
	storagePrefix := cfg.MediaPublicBaseURL + "/"
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
	contactAdminRepo := contact.NewAdminRepository(pool)
	contactLimiter := contact.NewLimiter()
	mediaAdminRepo := media.NewAdminRepository(pool, storagePrefix)
	var mediaStorage media.Storage
	localMediaDir := ""
	if cfg.R2Configured() {
		mediaStorage = media.NewR2(cfg.R2AccountID, cfg.R2AccessKeyID, cfg.R2SecretAccessKey, cfg.R2Bucket)
		log.Info("media storage: r2", "bucket", cfg.R2Bucket)
	} else {
		mediaStorage = media.NewLocalStorage(cfg.MediaDir)
		localMediaDir = cfg.MediaDir
		log.Info("media storage: local folder (development only)", "dir", cfg.MediaDir)
	}
	usersRepo := users.NewAdminRepository(pool)
	accounts := users.NewPGAccounts(pool)
	lessonsRepo := lessons.NewAdminRepository(pool)
	loginLimiter := contact.NewLimiterWith(rate.Every(6*time.Second), 10) // ~10/min per IP
	if created, err := users.SeedAdmin(ctx, accounts, usersRepo, cfg.BootstrapAdminEmail, cfg.BootstrapAdminPassword); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	} else if created {
		log.Info("bootstrap admin created", "email", cfg.BootstrapAdminEmail)
	}
	// Background eviction lives with the process; pass context.Background()
	// because RegisterPublic doesn't get the run-loop context.
	go contactLimiter.StartEvictor(context.Background())
	go loginLimiter.StartEvictor(context.Background())

	if cfg.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	r := apphttp.NewRouter(apphttp.RouterDeps{
		Pool:          pool,
		CORSOrigins:   cfg.CORSOrigins,
		LocalMediaDir: localMediaDir,
		Verifier:      issuer,
		RegisterAdmin: func(g *gin.RouterGroup) {
			adminapi.Register(g, adminapi.Deps{
				Resolver:        resolver,
				PostsAdmin:      postsAdminRepo,
				CategoriesAdmin: categoriesAdminRepo,
				TagsAdmin:       tagsAdminRepo,
				ProfileAdmin:    profileAdminRepo,
				SettingsAdmin:   siteAdminRepo,
				ContactAdmin:    contactAdminRepo,
				MediaAdmin:      mediaAdminRepo,
				MediaStorage:    mediaStorage,
				UsersAdmin:      usersRepo,
				UsersAccounts:   accounts,
			})
		},
		RegisterPublic: func(g *gin.RouterGroup) {
			posts.RegisterPublic(g, postsRepo)
			categories.RegisterPublic(g, categoriesRepo, postsRepo)
			tags.RegisterPublic(g, tagsRepo, postsRepo)
			profile.RegisterPublic(g, profileRepo)
			sitesettings.RegisterPublic(g, siteRepo)
			contact.RegisterPublic(g, contactRepo, contactLimiter)
			session.RegisterPublic(g, accounts, issuer, loginLimiter)
		},
	})

	// Lessons is served over Connect (gRPC-compatible) on the same port. The
	// outer mux sends the service prefix to Connect and everything else to Gin,
	// so Gin's CORS and routes are untouched. Auth lives in the RPCAuth
	// interceptor so gRPC and gRPC-Web callers get proper status codes.
	mux := http.NewServeMux()
	lessonsPath, lessonsHandler := lessons.NewConnectHandler(lessonsRepo, apphttp.RPCLog(log), apphttp.RPCAuth(issuer, resolver, "lessons:write"))
	mux.Handle(lessonsPath, apphttp.ConnectCORS(cfg.CORSOrigins)(lessonsHandler))
	mux.Handle("/", r)

	// h2c lets native gRPC clients (grpcurl -plaintext) reach the handler
	// locally. Render terminates TLS and forwards HTTP/1.1, where Connect's
	// unary RPCs still work.
	var protocols http.Protocols
	protocols.SetHTTP1(true)
	protocols.SetUnencryptedHTTP2(true)

	srv := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		Protocols:         &protocols,
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
