// @title           FunBaza API
// @version         1.0
// @description     API для генерации персонализированных поздравлений: AI-изображения + песни. Аутентификация через httpOnly cookie (access_token).
// @contact.name    FunBaza Team
// @license.name    MIT
// @BasePath        /api
// @securityDefinitions.apikey CookieAuth
// @in cookie
// @name access_token
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	_ "github.com/you/funbaza/docs"
	"github.com/you/funbaza/internal/config"
	"github.com/you/funbaza/internal/handlers"
	"github.com/you/funbaza/internal/middleware"
	"github.com/you/funbaza/internal/repository"
	"github.com/you/funbaza/internal/services"
	"github.com/you/funbaza/internal/worker"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config error", "err", err)
		os.Exit(1)
	}

	db, err := repository.NewDB(cfg.DatabaseURL)
	if err != nil {
		slog.Error("database error", "err", err)
		os.Exit(1)
	}
	defer db.Close()
	slog.Info("database connected")

	if err := repository.RunMigrations(db, "migrations"); err != nil {
		slog.Error("migration error", "err", err)
		os.Exit(1)
	}
	slog.Info("migrations applied")

	rdbOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		slog.Error("redis url error", "err", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(rdbOpts)
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("redis connection error", "err", err)
		os.Exit(1)
	}
	defer rdb.Close()
	slog.Info("redis connected")

	var storage services.StorageService
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = fmt.Sprintf("http://localhost:%s", cfg.AppPort)
	}
	if cfg.StorageMode == "local" {
		storage, err = services.NewLocalStorage(cfg.StorageLocalDir, baseURL)
		if err != nil {
			slog.Error("storage error", "err", err)
			os.Exit(1)
		}
		slog.Info("storage: local", "dir", cfg.StorageLocalDir)
	} else if cfg.StorageMode == "s3" {
		storage, err = services.NewS3Storage(context.Background(), services.S3StorageConfig{
			BaseURL:        baseURL,
			Endpoint:       cfg.S3Endpoint,
			PublicEndpoint: cfg.S3PublicEndpoint,
			Region:         cfg.S3Region,
			AccessKey:      cfg.S3AccessKey,
			SecretKey:      cfg.S3SecretKey,
			Bucket:         cfg.S3Bucket,
			UsePathStyle:   cfg.S3UsePathStyle,
		})
		if err != nil {
			slog.Error("storage error", "err", err)
			os.Exit(1)
		}
		slog.Info("storage: s3", "endpoint", cfg.S3Endpoint, "bucket", cfg.S3Bucket)
	} else {
		slog.Error("unsupported storage mode", "mode", cfg.StorageMode)
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	genRepo := repository.NewGenerationRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	jwtSvc := services.NewJWTService(cfg.JWTSecret)
	billingSvc := services.NewBillingService(billingRepo)
	googleOAuthSvc := services.NewGoogleOAuthService(cfg.GoogleClientID, cfg.GoogleClientSecret, cfg.GoogleRedirectURI)
	oauthStateSvc := services.NewOAuthStateService(cfg.JWTSecret)

	var imageGen services.ImageGenerator
	if cfg.KieAPIKey != "" {
		imageGen = services.NewKieImageGenerator(cfg.KieAPIKey, storage)
		slog.Info("image generator: kie.ai gpt-image-2")
	} else {
		imageGen = &services.MockImageGenerator{}
		slog.Info("image generator: mock")
	}
	var songGen services.SongGenerator
	if cfg.KieAPIKey != "" {
		songGen = services.NewKieSongGenerator(cfg.KieAPIKey, storage, cfg.FFmpegPath)
		slog.Info("song generator: kie.ai")
	} else {
		songGen = &services.MockSongGenerator{}
		slog.Info("song generator: mock")
	}

	queue := worker.NewQueue(rdb)
	webhookStore := worker.NewWebhookStore(rdb)

	webhookBase := cfg.BaseURL
	w := worker.New(queue, webhookStore, genRepo, sessionRepo, billingSvc, storage, imageGen, songGen, cfg.WorkerCount, webhookBase)
	if worker.IsPublicWebhookBase(webhookBase) {
		slog.Info("worker mode: async webhook", "base_url", webhookBase)
	} else if webhookBase != "" {
		slog.Info("worker mode: polling, webhook base is not public", "base_url", webhookBase)
	} else {
		slog.Info("worker mode: polling (set BASE_URL for webhook mode)")
	}

	authH := handlers.NewAuthHandler(userRepo, jwtSvc, billingSvc, googleOAuthSvc, oauthStateSvc, cfg.IsDev())
	billingH := handlers.NewBillingHandler(billingSvc)
	genH := handlers.NewGenerationHandler(genRepo, sessionRepo, billingSvc, storage, queue, songGen)
	sessionH := handlers.NewSessionHandler(sessionRepo, genRepo, storage)
	webhookH := handlers.NewWebhookHandler(webhookStore, genRepo, sessionRepo, billingSvc, storage)

	if !cfg.IsDev() {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(middleware.Recovery())
	r.Use(middleware.Logger())

	r.POST("/api/webhooks/kie", webhookH.KieCallback)
	r.POST("/api/webhooks/suno", webhookH.SunoCallback)

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok", "time": time.Now().UTC()})
	})

	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	auth := r.Group("/api/auth")
	{
		auth.GET("/google/login", authH.GoogleLogin)
		auth.GET("/google/callback", authH.GoogleCallback)
		auth.POST("/refresh", authH.Refresh)
		auth.POST("/logout", authH.Logout)
	}

	secured := r.Group("/api/v1")
	secured.Use(middleware.AuthRequired(jwtSvc))
	{
		secured.GET("/user/me", authH.Me)

		secured.GET("/billing/balance", billingH.Balance)
		secured.GET("/billing/tariff", billingH.Tariff)
		secured.GET("/billing/estimate", billingH.Estimate)
		secured.GET("/billing/transactions", billingH.Transactions)

		secured.GET("/sessions", sessionH.List)
		secured.GET("/sessions/:id", sessionH.Get)
		secured.PATCH("/sessions/:id", sessionH.UpdateTitle)

		secured.POST("/generations/lyrics", genH.GenerateLyrics)
		secured.POST("/generations", genH.Create)
		secured.GET("/generations", genH.List)
		secured.GET("/generations/:id", genH.Get)
		secured.GET("/generations/:id/status", genH.Status)

		secured.POST("/uploads", genH.Upload)
	}

	if cfg.StorageMode == "local" {
		r.GET("/api/files/*key", func(c *gin.Context) {
			key := c.Param("key")[1:]
			filePath, err := services.SafeLocalStoragePath(cfg.StorageLocalDir, key)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": gin.H{"code": "invalid_file_key", "message": "Invalid file key"}})
				return
			}
			f, err := os.Open(filePath)
			if err != nil {
				c.Status(http.StatusNotFound)
				return
			}
			defer f.Close()
			stat, _ := f.Stat()
			filename := filepath.Base(filePath)
			c.Writer.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
			http.ServeContent(c.Writer, c.Request, filename, stat.ModTime(), f)
		})
	}

	// Раздаём собранный фронтенд в production (когда есть ./web/dist)
	const webDist = "./web/dist"
	if _, statErr := os.Stat(webDist); statErr == nil {
		r.Static("/assets", webDist+"/assets")
		r.Static("/locales", webDist+"/locales")
		serveSVG := func(c *gin.Context) {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(webDist + c.Request.URL.Path)
		}
		r.GET("/favicon.svg", serveSVG)
		r.HEAD("/favicon.svg", func(c *gin.Context) {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(webDist + "/favicon.svg")
		})
		r.GET("/icons.svg", serveSVG)
		r.HEAD("/icons.svg", func(c *gin.Context) {
			c.Header("Content-Type", "image/svg+xml")
			c.Header("Cache-Control", "public, max-age=86400")
			c.File(webDist + "/icons.svg")
		})
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "Not found"}})
				return
			}
			// Always revalidate the HTML shell so new frontend asset hashes are picked up after deploys.
			c.Header("Cache-Control", "no-store, no-cache, must-revalidate")
			c.Header("Pragma", "no-cache")
			c.Header("Expires", "0")
			c.File(webDist + "/index.html")
		})
		slog.Info("serving frontend", "path", webDist)
	}

	srv := &http.Server{Addr: ":" + cfg.AppPort, Handler: r}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	go w.Run(workerCtx)

	go func() {
		slog.Info("server starting", "port", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down...")
	cancelWorker()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	slog.Info("server stopped")
}
