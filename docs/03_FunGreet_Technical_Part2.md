# FunGreet — Техническая документация (часть 2)

Продолжение: структура кода, инфраструктура, тестирование, план разработки.

---

## 8. Структура кода — backend

### 8.1. Организация проекта

```
backend/
├── cmd/
│   ├── server/
│   │   └── main.go                    # Entry point для HTTP + Worker
│   └── migrate/
│       └── main.go                    # CLI для миграций
│
├── internal/                           # Внутренние пакеты (не импортятся извне)
│   ├── config/
│   │   └── config.go                  # Загрузка env, валидация
│   │
│   ├── domain/                        # Бизнес-модели (domain entities)
│   │   ├── user.go
│   │   ├── identity.go
│   │   ├── generation.go
│   │   └── errors.go                  # Общие бизнес-ошибки
│   │
│   ├── repository/                    # Доступ к БД
│   │   ├── postgres.go                # Connection pool
│   │   ├── users.go
│   │   ├── identities.go
│   │   ├── generations.go
│   │   ├── credits.go
│   │   └── migrations/                # SQL файлы миграций
│   │       ├── 001_init.up.sql
│   │       ├── 001_init.down.sql
│   │       └── ...
│   │
│   ├── service/                       # Бизнес-логика
│   │   ├── auth/
│   │   │   ├── service.go
│   │   │   ├── jwt.go
│   │   │   ├── oauth/
│   │   │   │   ├── provider.go        # Interface
│   │   │   │   ├── google.go
│   │   │   │   ├── vk.go
│   │   │   │   └── yandex.go
│   │   │   └── telegram.go            # initData validation
│   │   ├── credits/
│   │   │   └── service.go
│   │   ├── generation/
│   │   │   └── service.go
│   │   └── storage/
│   │       └── r2.go                  # Cloudflare R2 client
│   │
│   ├── ai/                            # AI интеграции
│   │   ├── nanobnn/
│   │   │   ├── client.go
│   │   │   ├── prompts.go             # Шаблоны промптов
│   │   │   └── safety.go              # Обработка safety filter
│   │   ├── suno/
│   │   │   ├── client.go
│   │   │   └── types.go
│   │   └── audio/
│   │       └── preprocessor.go        # FFmpeg wrapper
│   │
│   ├── worker/
│   │   ├── worker.go                  # Main loop
│   │   ├── queue.go                   # Redis queue abstraction
│   │   └── pipeline.go                # Generation pipeline
│   │
│   ├── handler/                       # HTTP handlers
│   │   ├── auth.go
│   │   ├── user.go
│   │   ├── generation.go
│   │   ├── upload.go
│   │   ├── health.go
│   │   └── error.go                   # Общий error handler
│   │
│   ├── middleware/
│   │   ├── auth.go                    # JWT validation
│   │   ├── csrf.go
│   │   ├── cors.go
│   │   ├── ratelimit.go
│   │   ├── logger.go
│   │   └── recovery.go
│   │
│   └── server/
│       ├── router.go                  # Gin routing setup
│       └── server.go                  # HTTP server lifecycle
│
├── pkg/                               # Можно переиспользовать в других проектах
│   └── telegram/
│       └── initdata.go                # Если решим опубликовать
│
├── scripts/
│   ├── setup-dev.sh                   # Локальная настройка
│   └── check-suno-cookie.sh           # Проверка cookie
│
├── deployments/
│   ├── docker-compose.yml             # Dev
│   ├── docker-compose.prod.yml        # Production
│   ├── Dockerfile
│   └── nginx.conf
│
├── .env.example
├── .env.dev
├── .gitignore
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

### 8.2. Entry point — cmd/server/main.go

```go
package main

import (
    "context"
    "log/slog"
    "os"
    "os/signal"
    "syscall"
    "time"
    
    "github.com/you/fungreet/internal/config"
    "github.com/you/fungreet/internal/server"
    "github.com/you/fungreet/internal/worker"
)

func main() {
    // 1. Инициализация логгера
    logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
        Level: slog.LevelInfo,
    }))
    slog.SetDefault(logger)
    
    // 2. Загрузка конфига
    cfg, err := config.Load()
    if err != nil {
        logger.Error("config load failed", "error", err)
        os.Exit(1)
    }
    
    // 3. Создание приложения
    app, err := NewApp(cfg)
    if err != nil {
        logger.Error("app init failed", "error", err)
        os.Exit(1)
    }
    
    // 4. Context для graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // 5. Запуск HTTP сервера
    go func() {
        if err := app.Server.Start(); err != nil {
            logger.Error("server stopped", "error", err)
            cancel()
        }
    }()
    
    // 6. Запуск воркера
    go func() {
        if err := app.Worker.Start(ctx); err != nil {
            logger.Error("worker stopped", "error", err)
            cancel()
        }
    }()
    
    logger.Info("fungreet started", "version", cfg.Version)
    
    // 7. Ждём сигнал завершения
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    
    select {
    case sig := <-sigCh:
        logger.Info("shutdown signal", "signal", sig)
    case <-ctx.Done():
    }
    
    // 8. Graceful shutdown (30 секунд на завершение текущих задач)
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer shutdownCancel()
    
    if err := app.Shutdown(shutdownCtx); err != nil {
        logger.Error("shutdown failed", "error", err)
        os.Exit(1)
    }
    
    logger.Info("fungreet stopped")
}
```

### 8.3. Config — internal/config/config.go

```go
package config

import (
    "fmt"
    "os"
    "strconv"
    "time"
    
    "github.com/joho/godotenv"
)

type Config struct {
    Version string
    Env     string // "development", "production"
    
    Server   ServerConfig
    Database DatabaseConfig
    Redis    RedisConfig
    Auth     AuthConfig
    AI       AIConfig
    Storage  StorageConfig
    Worker   WorkerConfig
}

type ServerConfig struct {
    Port        string
    FrontendURL string
    AllowedOrigins []string
}

type DatabaseConfig struct {
    URL                string
    MaxOpenConnections int
    MaxIdleConnections int
}

type RedisConfig struct {
    URL string
}

type AuthConfig struct {
    JWTSecret     string
    AccessTTL     time.Duration
    RefreshTTL    time.Duration
    CookieDomain  string
    Google        OAuthProviderConfig
    VK            OAuthProviderConfig
    Yandex        OAuthProviderConfig
    TelegramBotToken string
}

type OAuthProviderConfig struct {
    ClientID     string
    ClientSecret string
    RedirectURL  string
}

type AIConfig struct {
    GeminiAPIKey  string
    GeminiModel   string
    SunoAPIURL    string
}

type StorageConfig struct {
    R2AccountID       string
    R2AccessKeyID     string
    R2SecretAccessKey string
    R2Bucket          string
    R2PublicURL       string
}

type WorkerConfig struct {
    Concurrency int
    QueueName   string
}

func Load() (*Config, error) {
    // В dev-режиме подхватываем .env
    _ = godotenv.Load()
    
    cfg := &Config{
        Version: getEnv("VERSION", "dev"),
        Env:     getEnv("ENV", "development"),
        Server: ServerConfig{
            Port:           getEnv("PORT", "8080"),
            FrontendURL:    mustEnv("FRONTEND_URL"),
            AllowedOrigins: []string{mustEnv("FRONTEND_URL")},
        },
        Database: DatabaseConfig{
            URL:                mustEnv("DATABASE_URL"),
            MaxOpenConnections: getEnvInt("DB_MAX_OPEN_CONNS", 25),
            MaxIdleConnections: getEnvInt("DB_MAX_IDLE_CONNS", 5),
        },
        Redis: RedisConfig{
            URL: mustEnv("REDIS_URL"),
        },
        Auth: AuthConfig{
            JWTSecret:        mustEnv("JWT_SECRET"),
            AccessTTL:        getEnvDuration("JWT_ACCESS_TTL", 15*time.Minute),
            RefreshTTL:       getEnvDuration("JWT_REFRESH_TTL", 30*24*time.Hour),
            CookieDomain:     mustEnv("COOKIE_DOMAIN"),
            TelegramBotToken: mustEnv("TG_BOT_TOKEN"),
            Google: OAuthProviderConfig{
                ClientID:     mustEnv("GOOGLE_CLIENT_ID"),
                ClientSecret: mustEnv("GOOGLE_CLIENT_SECRET"),
                RedirectURL:  mustEnv("GOOGLE_REDIRECT_URL"),
            },
            VK: OAuthProviderConfig{
                ClientID:     mustEnv("VK_CLIENT_ID"),
                ClientSecret: mustEnv("VK_CLIENT_SECRET"),
                RedirectURL:  mustEnv("VK_REDIRECT_URL"),
            },
            Yandex: OAuthProviderConfig{
                ClientID:     mustEnv("YANDEX_CLIENT_ID"),
                ClientSecret: mustEnv("YANDEX_CLIENT_SECRET"),
                RedirectURL:  mustEnv("YANDEX_REDIRECT_URL"),
            },
        },
        AI: AIConfig{
            GeminiAPIKey: mustEnv("GEMINI_API_KEY"),
            GeminiModel:  getEnv("GEMINI_MODEL", "gemini-2.5-flash-image"),
            SunoAPIURL:   mustEnv("SUNO_API_URL"),
        },
        Storage: StorageConfig{
            R2AccountID:       mustEnv("R2_ACCOUNT_ID"),
            R2AccessKeyID:     mustEnv("R2_ACCESS_KEY_ID"),
            R2SecretAccessKey: mustEnv("R2_SECRET_ACCESS_KEY"),
            R2Bucket:          mustEnv("R2_BUCKET"),
            R2PublicURL:       mustEnv("R2_PUBLIC_URL"),
        },
        Worker: WorkerConfig{
            Concurrency: getEnvInt("WORKER_CONCURRENCY", 2),
            QueueName:   getEnv("WORKER_QUEUE", "gen_queue"),
        },
    }
    
    return cfg, nil
}

func getEnv(key, fallback string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return fallback
}

func mustEnv(key string) string {
    value := os.Getenv(key)
    if value == "" {
        panic(fmt.Sprintf("env var %s is required", key))
    }
    return value
}

func getEnvInt(key string, fallback int) int {
    if value := os.Getenv(key); value != "" {
        if n, err := strconv.Atoi(value); err == nil {
            return n
        }
    }
    return fallback
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
    if value := os.Getenv(key); value != "" {
        if d, err := time.ParseDuration(value); err == nil {
            return d
        }
    }
    return fallback
}
```

### 8.4. Domain model — internal/domain/user.go

```go
package domain

import "time"

type User struct {
    ID             int64
    Email          *string
    DisplayName    string
    AvatarURL      *string
    FreeCredits    int
    PaidCredits    int
    ReferralCode   string
    ReferredByID   *int64
    IsBlocked      bool
    CreatedAt      time.Time
    UpdatedAt      time.Time
}

func (u *User) TotalCredits() int {
    return u.FreeCredits + u.PaidCredits
}

func (u *User) CanSpendCredit() bool {
    return !u.IsBlocked && u.TotalCredits() > 0
}

type Identity struct {
    ID           int64
    UserID       int64
    Provider     string
    ProviderID   string
    Email        *string
    ProfileData  map[string]any
    CreatedAt    time.Time
}
```

### 8.5. Пример handler — internal/handler/auth.go

```go
package handler

import (
    "net/http"
    
    "github.com/gin-gonic/gin"
    "github.com/you/fungreet/internal/service/auth"
)

type AuthHandler struct {
    authService *auth.Service
}

func NewAuthHandler(authService *auth.Service) *AuthHandler {
    return &AuthHandler{authService: authService}
}

func (h *AuthHandler) RegisterRoutes(r *gin.RouterGroup) {
    r.GET("/:provider/login", h.OAuthLogin)
    r.GET("/:provider/callback", h.OAuthCallback)
    r.POST("/telegram", h.TelegramAuth)
    r.POST("/refresh", h.Refresh)
    r.POST("/logout", h.Logout)
}

func (h *AuthHandler) OAuthLogin(c *gin.Context) {
    provider := c.Param("provider")
    linkMode := c.Query("link") == "true"
    
    var existingUserID *int64
    if linkMode {
        if uid, ok := getUserID(c); ok {
            existingUserID = &uid
        }
    }
    
    authURL, err := h.authService.StartOAuth(c, provider, existingUserID)
    if err != nil {
        handleError(c, err)
        return
    }
    
    c.Redirect(http.StatusFound, authURL)
}

func (h *AuthHandler) OAuthCallback(c *gin.Context) {
    provider := c.Param("provider")
    code := c.Query("code")
    state := c.Query("state")
    
    if code == "" || state == "" {
        handleError(c, ErrInvalidInput)
        return
    }
    
    result, err := h.authService.CompleteOAuth(c, provider, code, state)
    if err != nil {
        handleError(c, err)
        return
    }
    
    // Устанавливаем cookies
    h.setAuthCookies(c, result.Tokens)
    
    // Редирект на фронт
    c.Redirect(http.StatusFound, h.authService.FrontendURL()+"/")
}

func (h *AuthHandler) TelegramAuth(c *gin.Context) {
    initData := extractTMAHeader(c)
    if initData == "" {
        handleError(c, ErrUnauthorized)
        return
    }
    
    result, err := h.authService.AuthTelegram(c, initData)
    if err != nil {
        handleError(c, err)
        return
    }
    
    h.setAuthCookies(c, result.Tokens)
    c.JSON(http.StatusOK, gin.H{"user": result.User})
}

func (h *AuthHandler) setAuthCookies(c *gin.Context, tokens *auth.TokenPair) {
    c.SetSameSite(http.SameSiteNoneMode)
    
    // Access token
    c.SetCookie("access_token", tokens.Access,
        int(tokens.AccessTTL.Seconds()),
        "/", h.authService.CookieDomain(),
        true, true)
    
    // Refresh token (ограниченный path)
    c.SetCookie("refresh_token", tokens.Refresh,
        int(tokens.RefreshTTL.Seconds()),
        "/api/auth/refresh", h.authService.CookieDomain(),
        true, true)
    
    // CSRF token (доступен JavaScript)
    c.SetCookie("csrf_token", tokens.CSRF,
        int(tokens.AccessTTL.Seconds()),
        "/", h.authService.CookieDomain(),
        true, false)
    
    // Добавляем Partitioned вручную
    cookies := c.Writer.Header().Values("Set-Cookie")
    for i, cookie := range cookies {
        cookies[i] = cookie + "; Partitioned"
    }
    c.Writer.Header()["Set-Cookie"] = cookies
}
```

### 8.6. Nano Banana клиент — internal/ai/nanobnn/client.go

```go
package nanobnn

import (
    "bytes"
    "context"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type Client struct {
    apiKey string
    model  string
    http   *http.Client
}

func NewClient(apiKey, model string) *Client {
    return &Client{
        apiKey: apiKey,
        model:  model,
        http: &http.Client{
            Timeout: 60 * time.Second,
        },
    }
}

type generateRequest struct {
    Contents []content `json:"contents"`
}

type content struct {
    Parts []part `json:"parts"`
}

type part struct {
    Text       string      `json:"text,omitempty"`
    InlineData *inlineData `json:"inline_data,omitempty"`
}

type inlineData struct {
    MimeType string `json:"mime_type"`
    Data     string `json:"data"`  // base64
}

type generateResponse struct {
    Candidates []struct {
        Content struct {
            Parts []part `json:"parts"`
        } `json:"content"`
        FinishReason string `json:"finishReason"`
    } `json:"candidates"`
    PromptFeedback struct {
        BlockReason string `json:"blockReason"`
    } `json:"promptFeedback"`
}

var (
    ErrSafetyFilter    = fmt.Errorf("rejected by safety filter")
    ErrRateLimited     = fmt.Errorf("rate limit exceeded")
    ErrServiceUnavail  = fmt.Errorf("service unavailable")
)

func (c *Client) GenerateImage(
    ctx context.Context,
    prompt string,
    refImage []byte,
    mimeType string,
) ([]byte, error) {
    req := generateRequest{
        Contents: []content{{
            Parts: []part{
                {Text: prompt},
                {InlineData: &inlineData{
                    MimeType: mimeType,
                    Data:     base64.StdEncoding.EncodeToString(refImage),
                }},
            },
        }},
    }
    
    body, _ := json.Marshal(req)
    url := fmt.Sprintf(
        "https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent",
        c.model,
    )
    
    httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    httpReq.Header.Set("Content-Type", "application/json")
    httpReq.Header.Set("x-goog-api-key", c.apiKey)
    
    resp, err := c.http.Do(httpReq)
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 429 {
        return nil, ErrRateLimited
    }
    if resp.StatusCode >= 500 {
        return nil, ErrServiceUnavail
    }
    
    respBody, _ := io.ReadAll(resp.Body)
    var result generateResponse
    if err := json.Unmarshal(respBody, &result); err != nil {
        return nil, fmt.Errorf("parse response: %w", err)
    }
    
    if result.PromptFeedback.BlockReason != "" {
        return nil, fmt.Errorf("%w: %s", ErrSafetyFilter, result.PromptFeedback.BlockReason)
    }
    
    for _, cand := range result.Candidates {
        if cand.FinishReason == "SAFETY" {
            return nil, ErrSafetyFilter
        }
        for _, p := range cand.Content.Parts {
            if p.InlineData != nil {
                return base64.StdEncoding.DecodeString(p.InlineData.Data)
            }
        }
    }
    
    return nil, fmt.Errorf("no image in response")
}
```

### 8.7. Prompts catalog — internal/ai/nanobnn/prompts.go

```go
package nanobnn

import (
    "fmt"
    "strings"
)

var occasionTemplates = map[string]string{
    "birthday": `Create a vibrant, funny birthday greeting image. Place the person from the uploaded photo %s. Keep the face clearly recognizable. Add festive elements: balloons, confetti, cake. %s Style: warm colors, slightly cartoonish but realistic face.`,
    
    "new_year": `Create a festive New Year greeting image. The person from the photo should be depicted %s. Add sparklers, champagne, snow, lights. %s Style: sparkling, magical atmosphere.`,
    
    "roast": `Create a hilarious but friendly roast image. Take the person from the photo and place them %s. Make it funny and absurd but not offensive. %s Style: meme-worthy, bright colors.`,
    
    "love": `Create a romantic greeting image. Show the person from the photo %s. Add hearts, flowers, warm lighting. %s Style: dreamy, romantic.`,
    
    "custom": `Create a greeting image. %s Use the person's face from the uploaded photo, keeping it recognizable. %s Style: vibrant, high quality.`,
}

// BuildPrompt собирает финальный промпт из шаблона + пользовательских данных
func BuildPrompt(occasion, userDesc, recipientName string) string {
    template, ok := occasionTemplates[occasion]
    if !ok {
        template = occasionTemplates["custom"]
    }
    
    userContext := fmt.Sprintf("as %s", userDesc)
    nameHint := ""
    if recipientName != "" {
        nameHint = fmt.Sprintf("The greeting is for someone named %s.", recipientName)
    }
    
    return fmt.Sprintf(template, userContext, nameHint)
}

// SimplifyPrompt упрощает промпт если safety filter отклонил
func SimplifyPrompt(original string) string {
    // Убираем слова, которые могут триггерить фильтр
    replacements := map[string]string{
        "roast":  "celebration",
        "hilarious": "fun",
        "absurd": "creative",
        "face":   "person",
    }
    
    result := original
    for old, new := range replacements {
        result = strings.ReplaceAll(result, old, new)
    }
    
    return result
}
```

---

## 9. Структура кода — frontend

### 9.1. Организация проекта

```
frontend/
├── public/
│   ├── favicon.ico
│   └── og-image.png
│
├── src/
│   ├── components/
│   │   ├── ui/                         # shadcn/ui компоненты (автогенерация)
│   │   │   ├── button.tsx
│   │   │   ├── card.tsx
│   │   │   ├── input.tsx
│   │   │   ├── form.tsx
│   │   │   ├── progress.tsx
│   │   │   ├── skeleton.tsx
│   │   │   ├── sonner.tsx
│   │   │   ├── avatar.tsx
│   │   │   ├── dropdown-menu.tsx
│   │   │   └── ...
│   │   │
│   │   ├── creation/
│   │   │   ├── CreationStepper.tsx     # Главный stepper
│   │   │   ├── StepRecipient.tsx       # Шаг 1
│   │   │   ├── StepPhotos.tsx          # Шаг 2
│   │   │   ├── StepMusic.tsx           # Шаг 3
│   │   │   ├── StepReview.tsx          # Шаг 4
│   │   │   ├── PhotoUploader.tsx
│   │   │   └── AudioUploader.tsx
│   │   │
│   │   ├── results/
│   │   │   ├── GenerationProgress.tsx
│   │   │   ├── ImageGallery.tsx
│   │   │   ├── AudioPlayer.tsx
│   │   │   └── ShareButtons.tsx
│   │   │
│   │   ├── auth/
│   │   │   ├── LoginButtons.tsx
│   │   │   ├── ProviderIcon.tsx
│   │   │   └── AuthGuard.tsx
│   │   │
│   │   └── layout/
│   │       ├── Header.tsx
│   │       ├── UserMenu.tsx
│   │       └── PageContainer.tsx
│   │
│   ├── pages/
│   │   ├── HomePage.tsx                # /
│   │   ├── LoginPage.tsx               # /login
│   │   ├── CreatePage.tsx              # /create
│   │   ├── ResultPage.tsx              # /results/:id
│   │   ├── HistoryPage.tsx             # /history
│   │   ├── ProfilePage.tsx             # /profile
│   │   ├── TermsPage.tsx               # /terms
│   │   └── PrivacyPage.tsx             # /privacy
│   │
│   ├── hooks/
│   │   ├── useAuth.ts
│   │   ├── useCurrentUser.ts
│   │   ├── useCredits.ts
│   │   ├── useGeneration.ts
│   │   └── useGenerations.ts
│   │
│   ├── lib/
│   │   ├── api.ts                      # fetch wrapper + auto refresh
│   │   ├── utils.ts                    # cn() и shadcn helpers
│   │   └── constants.ts                # OCCASIONS, STYLES и т.д.
│   │
│   ├── providers/
│   │   ├── AuthProvider.tsx
│   │   └── QueryProvider.tsx
│   │
│   ├── App.tsx                         # Роутинг
│   ├── main.tsx                        # Entry point
│   └── index.css                       # Tailwind + shadcn variables
│
├── components.json                     # shadcn config
├── tailwind.config.ts
├── tsconfig.json
├── vite.config.ts
├── package.json
└── README.md
```

### 9.2. API клиент — src/lib/api.ts

```typescript
const API_BASE = import.meta.env.VITE_API_URL || '/api';

export class APIError extends Error {
  constructor(
    public code: string,
    public status: number,
    message: string,
  ) {
    super(message);
  }
}

function getCsrfToken(): string {
  const match = document.cookie.match(/csrf_token=([^;]+)/);
  return match ? match[1] : '';
}

// Главная функция — все запросы идут через неё
export async function apiRequest<T>(
  path: string,
  options: RequestInit = {},
): Promise<T> {
  const method = options.method || 'GET';
  const needsCSRF = !['GET', 'HEAD', 'OPTIONS'].includes(method);
  
  const response = await fetch(`${API_BASE}${path}`, {
    ...options,
    credentials: 'include',  // отправлять cookies
    headers: {
      'Content-Type': 'application/json',
      ...(needsCSRF && { 'X-CSRF-Token': getCsrfToken() }),
      ...options.headers,
    },
  });

  // 401 → пытаемся обновить токен
  if (response.status === 401 && !path.includes('/auth/')) {
    const refreshed = await refreshAccessToken();
    if (refreshed) {
      return apiRequest(path, options);  // retry
    }
    // refresh не удался — разлогиниваемся
    window.location.href = '/login';
    throw new APIError('unauthorized', 401, 'Session expired');
  }

  const data = await response.json();

  if (!response.ok) {
    throw new APIError(
      data.error?.code || 'unknown',
      response.status,
      data.error?.message || 'Unknown error',
    );
  }

  return data;
}

async function refreshAccessToken(): Promise<boolean> {
  try {
    const response = await fetch(`${API_BASE}/auth/refresh`, {
      method: 'POST',
      credentials: 'include',
    });
    return response.ok;
  } catch {
    return false;
  }
}

// Upload helper (отдельно, т.к. multipart/form-data)
export async function uploadFiles(files: File[]): Promise<{ key: string; mime_type: string; size: number }[]> {
  const formData = new FormData();
  files.forEach((f) => formData.append('files', f));

  const response = await fetch(`${API_BASE}/uploads`, {
    method: 'POST',
    credentials: 'include',
    headers: {
      'X-CSRF-Token': getCsrfToken(),
    },
    body: formData,
  });

  if (!response.ok) {
    throw new APIError('upload_failed', response.status, 'Upload failed');
  }

  const data = await response.json();
  return data.uploads;
}
```

### 9.3. Auth hook — src/hooks/useAuth.ts

```typescript
import { createContext, useContext, useEffect, useState, ReactNode } from 'react';
import { apiRequest } from '@/lib/api';

interface User {
  id: number;
  email: string | null;
  display_name: string;
  avatar_url: string | null;
  free_credits: number;
  paid_credits: number;
}

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  logout: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  useEffect(() => {
    // Пытаемся получить текущего пользователя
    apiRequest<{ user: User }>('/user/me')
      .then((data) => setUser(data.user))
      .catch(() => setUser(null))
      .finally(() => setIsLoading(false));
  }, []);

  const logout = async () => {
    await apiRequest('/auth/logout', { method: 'POST' }).catch(() => {});
    setUser(null);
    window.location.href = '/login';
  };

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        logout,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error('useAuth must be used inside AuthProvider');
  }
  return ctx;
}
```

---

## 10. Инфраструктура и деплой

### 10.1. Docker Compose для production

```yaml
version: "3.8"

services:
  # === Traefik reverse proxy с автоматическим SSL ===
  traefik:
    image: traefik:v3.1
    command:
      - --providers.docker=true
      - --providers.docker.exposedbydefault=false
      - --entrypoints.web.address=:80
      - --entrypoints.websecure.address=:443
      - --entrypoints.web.http.redirections.entrypoint.to=websecure
      - --entrypoints.web.http.redirections.entrypoint.scheme=https
      - --certificatesresolvers.le.acme.email=you@example.com
      - --certificatesresolvers.le.acme.storage=/letsencrypt/acme.json
      - --certificatesresolvers.le.acme.tlschallenge=true
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
      - traefik-letsencrypt:/letsencrypt
    restart: unless-stopped

  # === Frontend (static nginx) ===
  frontend:
    image: ghcr.io/you/fungreet-frontend:latest
    labels:
      - traefik.enable=true
      - traefik.http.routers.frontend.rule=Host(`fungreet.app`)
      - traefik.http.routers.frontend.tls.certresolver=le
      - traefik.http.services.frontend.loadbalancer.server.port=80
    restart: unless-stopped

  # === Backend ===
  backend:
    image: ghcr.io/you/fungreet-backend:latest
    env_file: .env.production
    environment:
      - DATABASE_URL=postgres://fungreet:${DB_PASSWORD}@postgres:5432/fungreet?sslmode=disable
      - REDIS_URL=redis://redis:6379
      - SUNO_API_URL=http://suno-api:3000
    labels:
      - traefik.enable=true
      - traefik.http.routers.backend.rule=Host(`api.fungreet.app`)
      - traefik.http.routers.backend.tls.certresolver=le
      - traefik.http.services.backend.loadbalancer.server.port=8080
    depends_on:
      - postgres
      - redis
      - suno-api
    restart: unless-stopped

  # === Suno API Proxy ===
  suno-api:
    build: ./suno-api
    environment:
      - SUNO_COOKIE=${SUNO_COOKIE}
      - TWOCAPTCHA_KEY=${TWOCAPTCHA_KEY}
      - BROWSER=chromium
      - BROWSER_HEADLESS=true
    restart: unless-stopped

  # === PostgreSQL ===
  postgres:
    image: postgres:16-alpine
    environment:
      - POSTGRES_DB=fungreet
      - POSTGRES_USER=fungreet
      - POSTGRES_PASSWORD=${DB_PASSWORD}
    volumes:
      - pg-data:/var/lib/postgresql/data
    restart: unless-stopped

  # === Redis ===
  redis:
    image: redis:7-alpine
    command: redis-server --appendonly yes
    volumes:
      - redis-data:/data
    restart: unless-stopped

  # === Backups ===
  postgres-backup:
    image: prodrigestivill/postgres-backup-local
    environment:
      - POSTGRES_HOST=postgres
      - POSTGRES_DB=fungreet
      - POSTGRES_USER=fungreet
      - POSTGRES_PASSWORD=${DB_PASSWORD}
      - SCHEDULE=0 3 * * *   # каждый день в 3 ночи
      - BACKUP_KEEP_DAYS=7
      - BACKUP_KEEP_WEEKS=4
    volumes:
      - ./backups:/backups
    depends_on:
      - postgres
    restart: unless-stopped

volumes:
  pg-data:
  redis-data:
  traefik-letsencrypt:
```

### 10.2. GitHub Actions CI/CD

```yaml
# .github/workflows/deploy.yml
name: Deploy

on:
  push:
    branches: [main]

jobs:
  build-and-deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Build and push backend
        uses: docker/build-push-action@v5
        with:
          context: ./backend
          push: true
          tags: ghcr.io/${{ github.repository_owner }}/fungreet-backend:latest

      - name: Build and push frontend
        uses: docker/build-push-action@v5
        with:
          context: ./frontend
          push: true
          tags: ghcr.io/${{ github.repository_owner }}/fungreet-frontend:latest

      - name: Deploy to VPS
        uses: appleboy/ssh-action@v1.0.0
        with:
          host: ${{ secrets.VPS_HOST }}
          username: ${{ secrets.VPS_USER }}
          key: ${{ secrets.VPS_SSH_KEY }}
          script: |
            cd /home/fungreet
            docker-compose pull
            docker-compose up -d
            docker image prune -f
```

### 10.3. .env.production пример

```env
ENV=production
VERSION=1.0.0

# Server
PORT=8080
FRONTEND_URL=https://fungreet.app
COOKIE_DOMAIN=.fungreet.app

# Database
DATABASE_URL=postgres://fungreet:SECRET@postgres:5432/fungreet?sslmode=disable

# Redis
REDIS_URL=redis://redis:6379

# Auth
JWT_SECRET=<32+ random bytes>
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h

# OAuth
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URL=https://api.fungreet.app/api/auth/google/callback

VK_CLIENT_ID=...
VK_CLIENT_SECRET=...
VK_REDIRECT_URL=https://api.fungreet.app/api/auth/vk/callback

YANDEX_CLIENT_ID=...
YANDEX_CLIENT_SECRET=...
YANDEX_REDIRECT_URL=https://api.fungreet.app/api/auth/yandex/callback

# Telegram (для будущей итерации)
TG_BOT_TOKEN=...

# AI
GEMINI_API_KEY=...
GEMINI_MODEL=gemini-2.5-flash-image
SUNO_API_URL=http://suno-api:3000

# Storage (R2)
R2_ACCOUNT_ID=...
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=fungreet-storage
R2_PUBLIC_URL=https://pub-xxx.r2.dev

# Suno
SUNO_COOKIE=<из браузера>
TWOCAPTCHA_KEY=...

# Worker
WORKER_CONCURRENCY=2
WORKER_QUEUE=gen_queue

# Sentry
SENTRY_DSN=...
```

---

## 11. Мониторинг и логирование

### 11.1. Логи

Всё в stdout в JSON формате (`log/slog`). Docker собирает, Docker Compose даёт `docker-compose logs -f`.

Что логировать:
- Каждый HTTP запрос (method, path, status, duration, user_id)
- Каждая генерация (start, step transitions, complete/fail, timing)
- Ошибки с контекстом (stack trace, user_id, request_id)
- AI запросы (провайдер, модель, время ответа, статус)
- Auth события (login, logout, refresh, failed attempts)

Что не логировать:
- Содержимое initData / JWT (только hash)
- OAuth tokens
- Пароли и секреты (их и не должно быть)
- Полные промпты (опционально, можно за флагом DEBUG)

### 11.2. Метрики

Для MVP достаточно:
- UptimeRobot: проверка `/api/health` каждые 5 минут
- Sentry: error tracking с алертами в Telegram
- Простой админ-скрипт (cron): проверка баланса Suno, остатка кредитов, 2Captcha

При росте подключить Prometheus + Grafana в отдельном Docker Compose.

### 11.3. Алерты

Критические (немедленный push в Telegram):
- Suno cookie invalid
- Suno credits < 500 (осталось меньше чем на день)
- 2Captcha balance < $5
- Backend health check fail 3 раза подряд
- Error rate > 5% за 5 минут

Предупреждения (email):
- Suno credits < 2000
- Gemini free tier остаток < 50
- Rate of failed generations > 10%

---

## 12. Тестирование

### 12.1. Стратегия

Уровни тестов:
1. **Unit тесты** — бизнес-логика (credits service, JWT, preprocessor)
2. **Integration тесты** — БД + репозитории через testcontainers-go
3. **E2E smoke тесты** — полный user flow через HTTP API (опционально)
4. **Ручное тестирование** — UI и реальные AI-интеграции

### 12.2. Что обязательно покрыть

- JWT issue/verify
- Credits service: списание, возврат, атомарность при конкурентных запросах
- FindOrCreateByOAuth: все 3 сценария (identity есть / email совпал / новый)
- FFmpeg preprocessor: каждый уровень фильтра
- Parser initData Telegram

### 12.3. Пример теста

```go
// internal/service/credits/service_test.go
func TestChargeCredit_ConcurrentRequests(t *testing.T) {
    ctx := context.Background()
    db := setupTestDB(t)
    service := credits.New(db)
    
    // Создаём пользователя с 1 кредитом
    userID := createTestUser(t, db, 1)
    
    // Запускаем 10 параллельных попыток списания
    var wg sync.WaitGroup
    successes := atomic.Int32{}
    
    for i := 0; i < 10; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            if err := service.Charge(ctx, userID, 1, "test"); err == nil {
                successes.Add(1)
            }
        }()
    }
    
    wg.Wait()
    
    // Ровно одна должна пройти
    assert.Equal(t, int32(1), successes.Load())
    
    // Баланс должен быть 0
    user := getUser(t, db, userID)
    assert.Equal(t, 0, user.TotalCredits())
}
```

### 12.4. Makefile для тестов

```makefile
.PHONY: test test-unit test-integration

test: test-unit test-integration

test-unit:
	go test -short -race ./...

test-integration:
	go test -race -tags=integration ./...

test-coverage:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
```

---

## 13. План разработки MVP

### 13.1. Сжатая версия таймлайна

| Неделя | Что делаем | Артефакт |
|--------|-----------|----------|
| 1 | Фундамент бэкенда | Go + Postgres + Redis, health check |
| 2 | Google OAuth + JWT + cookies | Полный login cycle через curl |
| 3 | Frontend фундамент | Vite + shadcn, login работает в UI |
| 4 | VK + Yandex + CSRF | Все 3 провайдера, слияние аккаунтов |
| 5 | БД генераций + R2 | Upload файлов работает |
| 6 | Nano Banana интеграция | CLI скрипт генерит картинку |
| 7 | Suno через suno-api | CLI скрипт генерит песню |
| 8 | Worker + API генераций | Полный цикл через Postman |
| 9 | Frontend форма создания | 4-шаговая форма работает |
| 10 | Frontend результаты | Полный user flow через UI |
| 11 | Полировка | Rate limiting, ошибки, Sentry, ToS |
| 12 | Деплой и запуск | Live на fungreet.app, 20 beta users |

Подробная разбивка по дням — в документе `FunGreet_Web_MVP_Plan.md`.

### 13.2. Приоритизация задач

**Критичные (без них нет продукта):**
- Авторизация хотя бы через один провайдер
- Генерация картинок + песен
- Списание кредитов при генерации
- Просмотр результата

**Важные (но можно отложить):**
- VK и Yandex OAuth
- История генераций
- Слияние аккаунтов
- Реферальная программа

**Опциональные (для будущих итераций):**
- Email verification
- Publisher watermark на картинках
- Sharing через токены
- Админ-панель

### 13.3. Критерии готовности MVP

Продукт можно запускать в soft launch когда:

- [ ] Регистрация работает через Google (VK и Yandex опционально)
- [ ] Новый пользователь получает 3 кредита
- [ ] Можно создать поздравление через UI
- [ ] Генерация успешна в 85%+ случаев
- [ ] Результат можно скачать
- [ ] История показывает прошлые генерации
- [ ] Terms of Service и Privacy Policy опубликованы
- [ ] Sentry настроен для трекинга ошибок
- [ ] Backup БД работает
- [ ] Health check + UptimeRobot
- [ ] Smoke test пройден на проде

### 13.4. Критерии переходу к следующей итерации

Когда решаем что MVP готов и переходим к Telegram Mini App:

- 100+ реальных пользователей протестировали
- Конверсия регистрация → генерация > 50%
- Средняя генерация успешна в 85%+ случаев
- Нет критических багов в бэклоге
- Suno cookie живёт минимум 3 дня в среднем
- Запас кредитов и 2Captcha на месяц вперёд

---

## Приложение A — Установка dev-окружения

```bash
# 1. Prerequisites
brew install go@1.23 node pnpm postgresql@16 redis ffmpeg mkcert
mkcert -install

# 2. Клонируем репозиторий
git clone https://github.com/you/fungreet
cd fungreet

# 3. Создаём локальные SSL-сертификаты
cd backend
mkcert -key-file ./certs/localhost-key.pem -cert-file ./certs/localhost.pem \
       localhost 127.0.0.1

# 4. Настраиваем .env
cp .env.example .env.dev
# Заполняем реальными значениями (ключи OAuth, Gemini, Suno cookie)

# 5. Запускаем БД и Redis
docker-compose up -d postgres redis

# 6. Миграции
make migrate-up

# 7. Запускаем suno-api (в отдельном окне)
docker-compose up suno-api

# 8. Запускаем бэкенд
make dev-backend  # использует air для hot reload

# 9. Запускаем фронт (в отдельном окне)
cd frontend
pnpm install
pnpm dev

# Открываем https://localhost:5173
```

## Приложение B — Команды Makefile

```makefile
.PHONY: dev-backend dev-frontend migrate-up migrate-down migrate-create test build deploy

dev-backend:
	air -c .air.toml

dev-frontend:
	cd frontend && pnpm dev

migrate-up:
	migrate -path ./internal/repository/migrations \
	        -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path ./internal/repository/migrations \
	        -database "$(DATABASE_URL)" down 1

migrate-create:
	migrate create -ext sql \
	               -dir ./internal/repository/migrations \
	               -seq $(name)

test:
	go test -race ./...

build:
	docker-compose build

deploy:
	git push origin main  # запустит GitHub Actions
```
