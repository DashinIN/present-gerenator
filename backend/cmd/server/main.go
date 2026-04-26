// @title           FunGreet API
// @version         1.0
// @description     API для генерации персонализированных поздравлений: AI-изображения + песни. Аутентификация через httpOnly cookie (access_token).
// @contact.name    FunGreet Team
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
	"github.com/you/fungreet/internal/config"
	"github.com/you/fungreet/internal/handlers"
	"github.com/you/fungreet/internal/middleware"
	"github.com/you/fungreet/internal/repository"
	"github.com/you/fungreet/internal/services"
	"github.com/you/fungreet/internal/worker"
	_ "github.com/you/fungreet/docs"
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
	if cfg.StorageMode == "local" {
		baseURL := cfg.BaseURL
		if baseURL == "" {
			baseURL = fmt.Sprintf("http://localhost:%s", cfg.AppPort)
		}
		storage, err = services.NewLocalStorage(cfg.StorageLocalDir, baseURL)
		if err != nil {
			slog.Error("storage error", "err", err)
			os.Exit(1)
		}
		slog.Info("storage: local", "dir", cfg.StorageLocalDir)
	} else {
		slog.Error("only local storage mode is supported in mock mode")
		os.Exit(1)
	}

	userRepo := repository.NewUserRepository(db)
	billingRepo := repository.NewBillingRepository(db)
	genRepo := repository.NewGenerationRepository(db)
	sessionRepo := repository.NewSessionRepository(db)

	jwtSvc := services.NewJWTService(cfg.JWTSecret)
	billingSvc := services.NewBillingService(billingRepo)

	var imageGen services.ImageGenerator
	if cfg.KieAPIKey != "" {
		imageGen = services.NewKieImageGenerator(cfg.KieAPIKey, storage)
		slog.Info("image generator: kie.ai gpt-image-2")
	} else {
		imageGen = &services.MockImageGenerator{}
		slog.Info("image generator: mock")
	}
	var songGen services.SongGenerator
	if cfg.SunoAPIKey != "" {
		songGen = services.NewSunoAPIGenerator(cfg.SunoAPIKey)
		slog.Info("song generator: sunoapi.org")
	} else {
		songGen = &services.MockSongGenerator{}
		slog.Info("song generator: mock")
	}

	queue := worker.NewQueue(rdb)
	webhookStore := worker.NewWebhookStore(rdb)

	webhookBase := cfg.BaseURL
	w := worker.New(queue, webhookStore, genRepo, sessionRepo, billingSvc, storage, imageGen, songGen, cfg.WorkerCount, webhookBase)
	if webhookBase != "" {
		slog.Info("worker mode: async webhook", "base_url", webhookBase)
	} else {
		slog.Info("worker mode: polling (set BASE_URL for webhook mode)")
	}

	authH := handlers.NewAuthHandler(userRepo, jwtSvc)
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
		auth.GET("/dev/login", authH.DevLogin)
		auth.POST("/refresh", authH.Refresh)
		auth.POST("/logout", authH.Logout)
	}

	secured := r.Group("/api")
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

	r.GET("/api/files/*key", func(c *gin.Context) {
		key := c.Param("key")[1:]
		filePath := filepath.Join(cfg.StorageLocalDir, filepath.FromSlash(key))
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

	// Раздаём собранный фронтенд в production (когда есть ./web/dist)
	const webDist = "./web/dist"
	if _, statErr := os.Stat(webDist); statErr == nil {
		r.Static("/assets", webDist+"/assets")
		r.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				c.JSON(http.StatusNotFound, gin.H{"error": gin.H{"code": "not_found", "message": "Not found"}})
				return
			}
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
