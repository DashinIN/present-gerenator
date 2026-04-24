# FunGreet — Техническая документация

Версия 1.0 · Апрель 2026

Документ для разработчика: архитектура, стек, схемы, код, план имплементации.

---

## Оглавление

1. [Архитектурный обзор](#1-архитектурный-обзор)
2. [Технический стек](#2-технический-стек)
3. [Схема базы данных](#3-схема-базы-данных)
4. [API спецификация](#4-api-спецификация)
5. [Интеграции с внешними сервисами](#5-интеграции-с-внешними-сервисами)
6. [Авторизация и безопасность](#6-авторизация-и-безопасность)
7. [Worker pipeline](#7-worker-pipeline)
8. [Структура кода — backend](#8-структура-кода-backend)
9. [Структура кода — frontend](#9-структура-кода-frontend)
10. [Инфраструктура и деплой](#10-инфраструктура-и-деплой)
11. [Мониторинг и логирование](#11-мониторинг-и-логирование)
12. [Тестирование](#12-тестирование)
13. [План разработки MVP](#13-план-разработки-mvp)

---

## 1. Архитектурный обзор

### 1.1. Высокоуровневая диаграмма

```
┌──────────────────────────────────────────────────────────────┐
│                    FRONTEND (React + TS)                      │
│                     fungreet.app                              │
│  Pages: /, /login, /create, /history, /results/:id           │
│  State: TanStack Query, React Context для auth               │
│  Styling: shadcn/ui + Tailwind CSS                           │
└──────────────────┬───────────────────────────────────────────┘
                   │ HTTPS + httpOnly cookies
                   │ credentials: 'include'
                   ▼
┌──────────────────────────────────────────────────────────────┐
│                  BACKEND (Go + Gin)                           │
│                     api.fungreet.app                          │
│  ┌────────────┐ ┌────────────┐ ┌──────────────┐             │
│  │  Handlers  │ │ Middleware │ │  Services    │             │
│  │  (REST)    │ │  (Auth,    │ │  (Auth,      │             │
│  │            │ │   CORS,    │ │   Credits,   │             │
│  │            │ │   CSRF,    │ │   Generation)│             │
│  │            │ │   RateLimit)│ │              │             │
│  └────────────┘ └────────────┘ └──────────────┘             │
│  ┌─────────────────────────────────────────────┐             │
│  │              Repository Layer                │             │
│  │  Users, Identities, Generations, Payments    │             │
│  └─────────────────────────────────────────────┘             │
└──────┬────────────────────┬──────────────────┬───────────────┘
       │                    │                  │
       ▼                    ▼                  ▼
┌──────────────┐   ┌─────────────┐   ┌──────────────────┐
│  PostgreSQL  │   │    Redis    │   │  Cloudflare R2   │
│  (Users,     │   │  (Queue,    │   │  (Photos, Audio, │
│   Gens,      │   │   Sessions, │   │   Results)       │
│   Payments)  │   │   RateLimit)│   │                  │
└──────────────┘   └─────────────┘   └──────────────────┘
                                             
┌──────────────────────────────────────────────────────────────┐
│                    WORKER (same Go binary)                    │
│  Подписан на Redis Queue, обрабатывает задачи асинхронно     │
└──────┬────────────────────┬──────────────────┬───────────────┘
       │                    │                  │
       ▼                    ▼                  ▼
┌──────────────┐   ┌─────────────────┐   ┌───────────────┐
│  Gemini API  │   │ gcui-art/       │   │    FFmpeg     │
│ Nano Banana  │   │ suno-api        │   │ (subprocess)  │
│              │   │ (localhost:3100)│   │               │
└──────────────┘   └─────────────────┘   └───────────────┘
                          │
                          ▼
                   ┌──────────────┐
                   │  Suno.com    │
                   │ (через cookie)│
                   └──────────────┘
```

### 1.2. Принципы архитектуры

**Монолит с возможностью расщепления.** Всё работает как один Go-бинарник, но разделено на слои (handlers, services, repositories). Когда понадобится — worker легко отделяется в отдельный процесс.

**PostgreSQL как source of truth.** Все критичные данные (пользователи, платежи, генерации) хранятся здесь. Redis — только для эфемерных данных (очереди, сессии, rate limiting).

**R2 для файлов.** Картинки и аудио не хранятся в БД — только ссылки. R2 бесплатен на egress (Cloudflare), что критично когда пользователи скачивают результаты.

**Асинхронность для долгих операций.** Генерация занимает 2-3 минуты. HTTP handler принимает запрос, кладёт в очередь, возвращает `{id, status}`. Worker обрабатывает. Фронтенд опрашивает статус.

**Graceful degradation.** Если Suno API недоступен — генерируем только картинки. Если Nano Banana rate limit — показываем пользователю честное сообщение "попробуйте через минуту".

### 1.3. Не-цели (осознанно не делаем)

- Микросервисы. Оверкилл для 1 разработчика и нагрузки до 1000 DAU.
- Kubernetes. Docker Compose на одном VPS достаточно для старта.
- GraphQL. REST проще и понятнее.
- Отдельный CDN. Cloudflare R2 отдаёт файлы напрямую через публичный URL.
- WebSockets. Для статуса генерации polling каждые 3 секунды работает отлично при таких объёмах.

---

## 2. Технический стек

### 2.1. Backend

| Компонент | Технология | Версия | Примечание |
|-----------|-----------|--------|-----------|
| Язык | Go | 1.23+ | Конкурентность, низкое потребление памяти |
| Web-фреймворк | Gin | v1.10+ | Быстрый, минималистичный |
| ORM/DB | database/sql + pgx | v5+ | Без тяжёлых ORM — pgx даёт максимум перформанса |
| Миграции | golang-migrate | v4+ | CLI + программный API |
| JWT | golang-jwt/jwt | v5+ | Стандарт для Go |
| Валидация | go-playground/validator | v10+ | Встроена в Gin |
| Логирование | log/slog | stdlib | Структурированные JSON-логи |
| Конфиг | godotenv + env vars | — | Простой и понятный |
| HTTP-клиент | net/http | stdlib | Без внешних зависимостей |
| CORS | gin-contrib/cors | — | Standard middleware |
| Тесты | testify + testcontainers-go | — | Интеграционные тесты с реальным Postgres |

### 2.2. Frontend

| Компонент | Технология | Версия |
|-----------|-----------|--------|
| Язык | TypeScript | 5.4+ |
| Framework | React | 19 |
| Билдер | Vite | 5+ |
| UI-kit | shadcn/ui | latest |
| Стили | Tailwind CSS | v4 |
| Роутинг | react-router-dom | v6 |
| State/Data | TanStack Query | v5 |
| Формы | react-hook-form | v7 |
| Иконки | lucide-react | latest |
| Toast | sonner | latest |
| Date | date-fns | v3 |

### 2.3. Инфраструктура

| Компонент | Сервис | Стоимость |
|-----------|--------|-----------|
| VPS | Hetzner CPX41 (8 vCPU, 16 GB, 240 GB) | $30/мес |
| DB | PostgreSQL 16 в Docker | $0 |
| Cache/Queue | Redis 7 в Docker | $0 |
| File storage | Cloudflare R2 | $0–15/мес |
| Domain | Cloudflare registrar | $10/год |
| SSL | Let's Encrypt через Traefik | $0 |
| DNS/CDN | Cloudflare | $0 |
| Monitoring | UptimeRobot + Sentry free tier | $0 |
| Error tracking | Sentry | Free tier (5K events/mo) |

### 2.4. Внешние API

| API | Стоимость | Назначение |
|-----|----------|------------|
| Gemini API (Nano Banana) | Free tier → $0.039/img | Генерация картинок |
| Suno.com Premier | $30/мес | Генерация музыки (через gcui-art/suno-api) |
| 2Captcha | ~$20/мес | Решение hCaptcha от Suno |
| Google OAuth | Бесплатно | Авторизация |
| VK ID | Бесплатно | Авторизация |
| Yandex ID | Бесплатно | Авторизация |

---

## 3. Схема базы данных

### 3.1. Полная DDL

```sql
-- ==========================================
-- Users & Auth
-- ==========================================

CREATE TABLE users (
    id              BIGSERIAL PRIMARY KEY,
    email           VARCHAR(255),
    display_name    VARCHAR(200),
    avatar_url      VARCHAR(500),
    free_credits    INT NOT NULL DEFAULT 3,
    paid_credits    INT NOT NULL DEFAULT 0,
    referral_code   VARCHAR(20) UNIQUE,
    referred_by     BIGINT REFERENCES users(id),
    is_blocked      BOOLEAN NOT NULL DEFAULT FALSE,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;
CREATE INDEX idx_users_referral_code ON users(referral_code) WHERE referral_code IS NOT NULL;

-- Один пользователь может иметь несколько способов входа
CREATE TABLE user_identities (
    id                  BIGSERIAL PRIMARY KEY,
    user_id             BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    provider            VARCHAR(20) NOT NULL,   -- 'telegram', 'google', 'vk', 'yandex'
    provider_id         VARCHAR(200) NOT NULL,  -- ID у провайдера
    email               VARCHAR(255),
    profile_data        JSONB,                  -- сырой профиль от провайдера
    access_token_enc    TEXT,                   -- зашифрованный (pgcrypto)
    refresh_token_enc   TEXT,
    token_expires_at    TIMESTAMPTZ,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    
    UNIQUE(provider, provider_id)
);

CREATE INDEX idx_identities_user ON user_identities(user_id);

-- Blacklist для refresh-токенов (при rotation)
CREATE TABLE refresh_token_blacklist (
    token_hash      VARCHAR(64) PRIMARY KEY,    -- SHA-256 от токена
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    blacklisted_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL
);

CREATE INDEX idx_blacklist_expires ON refresh_token_blacklist(expires_at);

-- ==========================================
-- Generations
-- ==========================================

CREATE TYPE generation_status AS ENUM (
    'pending',
    'processing_image',
    'processing_audio',
    'completed',
    'failed'
);

CREATE TABLE generation_requests (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id                 BIGINT NOT NULL REFERENCES users(id),
    status                  generation_status NOT NULL DEFAULT 'pending',
    
    -- Входные данные (пользовательский ввод)
    recipient_name          VARCHAR(200),
    occasion                VARCHAR(100),               -- 'birthday', 'new_year', 'roast', 'custom'
    image_prompt            TEXT,
    song_lyrics             TEXT,
    song_style              VARCHAR(200),
    song_title              VARCHAR(200),
    
    -- Загруженные файлы (R2 keys)
    input_photos            TEXT[] NOT NULL DEFAULT '{}',
    input_audio_key         VARCHAR(500),
    audio_preprocess_level  INT NOT NULL DEFAULT 0,     -- 0=оригинал, 1=stretch, 2=noise+pitch
    
    -- Результаты
    result_images           TEXT[] NOT NULL DEFAULT '{}',
    result_audio_key        VARCHAR(500),
    
    -- Мета
    suno_track_ids          TEXT[] NOT NULL DEFAULT '{}',
    gemini_request_count    INT NOT NULL DEFAULT 0,
    error_message           TEXT,
    error_code              VARCHAR(50),
    credits_spent           INT NOT NULL DEFAULT 1,
    processing_time_ms      BIGINT,
    
    created_at              TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at              TIMESTAMPTZ,
    completed_at            TIMESTAMPTZ
);

CREATE INDEX idx_gen_user_created ON generation_requests(user_id, created_at DESC);
CREATE INDEX idx_gen_status ON generation_requests(status) WHERE status IN ('pending', 'processing_image', 'processing_audio');

-- ==========================================
-- Payments (для будущей итерации, но схема готова)
-- ==========================================

CREATE TYPE payment_status AS ENUM ('pending', 'completed', 'failed', 'refunded');
CREATE TYPE payment_provider AS ENUM ('telegram_stars', 'stripe', 'yookassa', 'ton');

CREATE TABLE payments (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    amount_cents    INT NOT NULL,
    currency        VARCHAR(3) NOT NULL DEFAULT 'USD',
    credits_added   INT NOT NULL,
    provider        payment_provider NOT NULL,
    provider_tx_id  VARCHAR(200) UNIQUE,
    status          payment_status NOT NULL DEFAULT 'pending',
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);

CREATE INDEX idx_payments_user ON payments(user_id, created_at DESC);
CREATE INDEX idx_payments_provider_tx ON payments(provider, provider_tx_id);

-- ==========================================
-- Credit transactions (аудит всех изменений баланса)
-- ==========================================

CREATE TABLE credit_transactions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id),
    amount          INT NOT NULL,               -- отрицательное для списания
    reason          VARCHAR(50) NOT NULL,       -- 'registration_bonus', 'generation', 'refund', 'purchase', 'referral'
    reference_id    VARCHAR(100),               -- ID связанной генерации/платежа
    balance_after   INT NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_credit_tx_user ON credit_transactions(user_id, created_at DESC);
```

### 3.2. ER-диаграмма (текстовое описание)

```
users (1) ──< (N) user_identities
users (1) ──< (N) generation_requests
users (1) ──< (N) payments
users (1) ──< (N) credit_transactions
users (1) ──< (N) users (self-ref через referred_by)
```

Ключевые связи:

- **users ↔ user_identities**: один пользователь может войти через несколько провайдеров
- **users ↔ generation_requests**: история всех попыток генерации
- **users ↔ credit_transactions**: иммутабельный лог всех изменений баланса (для дебаггинга и аналитики)

### 3.3. Индексы и производительность

Критические запросы:

1. **"Получить баланс пользователя"** — `SELECT free_credits, paid_credits FROM users WHERE id = $1`
   — по PK, быстро
   
2. **"Последние 20 генераций пользователя"** — использует индекс `idx_gen_user_created`

3. **"Найти identity по провайдеру"** — использует `UNIQUE(provider, provider_id)`

4. **"Очередь pending генераций"** — partial index `idx_gen_status` только по активным статусам, не разбухает

### 3.4. Инвариант баланса

Важное правило: баланс пользователя должен **всегда** равняться сумме всех `credit_transactions`.

```sql
-- Проверка целостности
SELECT u.id, u.free_credits + u.paid_credits AS current_balance,
       COALESCE(SUM(ct.amount), 0) AS sum_transactions
FROM users u
LEFT JOIN credit_transactions ct ON ct.user_id = u.id
GROUP BY u.id
HAVING u.free_credits + u.paid_credits != COALESCE(SUM(ct.amount), 0);
-- Должно возвращать 0 строк
```

Это проверка делается ежедневно cron-job'ом. Если расхождение — алерт.

---

## 4. API спецификация

### 4.1. Конвенции

- Base URL: `https://api.fungreet.app`
- Все эндпоинты возвращают JSON
- Ошибки: стандартный формат `{ "error": { "code": "...", "message": "..." } }`
- Авторизация: httpOnly cookies (автоматически отправляются браузером)
- CSRF: header `X-CSRF-Token` на все POST/PUT/DELETE
- Content-Type: `application/json` (кроме upload — `multipart/form-data`)

### 4.2. Полный список эндпоинтов

#### Авторизация

```
GET    /api/auth/:provider/login
       Query: ?link=true (опционально, для привязки к текущему аккаунту)
       Response: 302 redirect на провайдера
       Providers: google, vk, yandex

GET    /api/auth/:provider/callback
       Query: ?code=xxx&state=yyy
       Response: Set-Cookie headers + 302 redirect на /

POST   /api/auth/telegram
       Headers: Authorization: tma {initData}
       Response: Set-Cookie headers + { user: {...} }

GET    /api/auth/telegram-widget
       Query: параметры от Telegram Login Widget
       Response: Set-Cookie headers + 302 redirect на /

POST   /api/auth/refresh
       Cookie: refresh_token
       Response: новые Set-Cookie headers
       
POST   /api/auth/logout
       Response: очистка cookies
```

#### Пользователь

```
GET    /api/user/me
       Response: {
         id, email, display_name, avatar_url,
         free_credits, paid_credits,
         identities: [{ provider, email }]
       }

PATCH  /api/user/me
       Body: { display_name?, avatar_url? }

DELETE /api/user/me
       Полное удаление аккаунта и всех данных

GET    /api/user/identities
       Список привязанных OAuth аккаунтов

DELETE /api/user/identities/:id
       Отвязка OAuth аккаунта (если это не последний)
```

#### Файлы

```
POST   /api/uploads
       Content-Type: multipart/form-data
       Body: files[] (до 3 картинок + 1 аудио)
       Response: { uploads: [{ key, mime_type, size }] }

GET    /api/files/:key
       Response: 302 redirect на pre-signed URL от R2
       Проверка: файл принадлежит текущему пользователю
```

#### Генерации

```
POST   /api/generations
       Body: {
         recipient_name: string,
         occasion: string,
         image_prompt: string,
         song_lyrics: string,
         song_style?: string,
         photo_keys: string[],  // из /api/uploads
         audio_key: string
       }
       Response: { id: UUID, status: "pending" }
       Списывает 1 кредит.

GET    /api/generations
       Query: ?limit=20&cursor=xxx
       Response: {
         items: [...],
         next_cursor: string | null
       }

GET    /api/generations/:id
       Response: полный объект GenerationRequest с signed URLs

GET    /api/generations/:id/status
       Облегчённый эндпоинт для polling
       Response: { status, progress_percent }

DELETE /api/generations/:id
       Удаление генерации + связанных файлов в R2
```

#### Кредиты

```
GET    /api/credits/balance
       Response: { free: N, paid: M, total: N+M }

GET    /api/credits/transactions
       Query: ?limit=50
       Response: { items: [...] }
```

#### Служебные

```
GET    /api/health
       Response: { status: "ok", version: "1.2.3" }

GET    /api/health/deep
       Проверяет Postgres, Redis, suno-api
       Response: { status: "ok", checks: {...} }
```

### 4.3. Примеры запросов

**Создание генерации:**

```http
POST /api/generations
Cookie: access_token=eyJ...
X-CSRF-Token: abc123
Content-Type: application/json

{
  "recipient_name": "Никита",
  "occasion": "birthday",
  "image_prompt": "Сделай его космонавтом в скафандре со шляпой-пиццей",
  "song_lyrics": "С днём рождения, Никита!\nТы у нас самый лучший...",
  "song_style": "upbeat pop",
  "photo_keys": ["uploads/123/abc.jpg", "uploads/123/def.jpg"],
  "audio_key": "uploads/123/reference.mp3"
}
```

**Ответ:**

```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending",
  "credits_remaining": 2,
  "estimated_time_seconds": 180
}
```

### 4.4. Коды ошибок

| Code | HTTP | Описание |
|------|------|----------|
| `invalid_credentials` | 401 | Невалидные токены |
| `token_expired` | 401 | Access token истёк (фронт сделает refresh) |
| `csrf_mismatch` | 403 | CSRF токен не совпадает |
| `insufficient_credits` | 402 | Нет кредитов для операции |
| `rate_limit_exceeded` | 429 | Превышен rate limit |
| `file_too_large` | 413 | Файл больше лимита |
| `invalid_file_format` | 400 | Неподдерживаемый формат |
| `generation_failed` | 500 | Внутренняя ошибка AI |
| `external_service_unavailable` | 503 | Suno или Gemini недоступны |

---

## 5. Интеграции с внешними сервисами

### 5.1. Gemini API (Nano Banana)

**Endpoint:** `https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash-image:generateContent`

**Аутентификация:** API key в header `x-goog-api-key`

**Rate limits (free tier):**
- 500 requests per day
- 10 requests per minute
- 250,000 tokens per minute

**Стратегия:**
1. Начинаем на free tier для MVP (до ~150 генераций/день)
2. При росте — переход на paid: $0.039/img для `gemini-2.5-flash-image`
3. Fallback на Imagen 4 Fast ($0.02/img) если Nano Banana недоступен

**Безопасность контента:**
- Safety filter может отклонить промпт
- Retry с упрощённой формулировкой (3 попытки)
- Логирование отказов для анализа и улучшения промптов

**Go клиент:** см. раздел 8.5

### 5.2. Suno через gcui-art/suno-api

**Локальный endpoint:** `http://suno-api:3000` (внутри Docker сети)

**Почему self-hosted:**
- Официального Suno API не существует
- Сторонние провайдеры берут $0.10+/песня
- Self-hosted даёт $0.02/песня при подписке Premier ($30/мес на 10,000 кредитов)

**Настройка:**
```env
SUNO_COOKIE=<из браузера>
TWOCAPTCHA_KEY=<API ключ>
BROWSER=chromium
BROWSER_HEADLESS=true
```

**Ключевые эндпоинты suno-api:**
- `POST /api/custom_generate` — генерация с lyrics + стилем
- `POST /api/extend_audio` — продление трека
- `GET /api/get?ids=` — polling статуса
- `GET /api/get_limit` — остаток кредитов

**Подводные камни:**
- Cookie протухает раз в 1-7 дней — нужен мониторинг + алерт
- 2Captcha решает hCaptcha за 20-40 секунд
- Playwright + Chromium ест 300-500 MB RAM в Docker
- При росте нужны несколько Suno аккаунтов и pool

**Обход copyright filter:**
1. Попытка с оригиналом
2. FFmpeg atempo=0.99 (stretch 1.01x)
3. FFmpeg pitch shift + 0.3s white noise

### 5.3. Cloudflare R2

**S3-compatible API.** Используем AWS SDK v2.

```env
R2_ACCOUNT_ID=...
R2_ACCESS_KEY_ID=...
R2_SECRET_ACCESS_KEY=...
R2_BUCKET=fungreet-storage
R2_PUBLIC_URL=https://pub-xxx.r2.dev  # или custom domain
```

**Стратегия ключей:**
- `uploads/{user_id}/{uuid}.{ext}` — загруженные пользователем
- `results/{generation_id}/image_{n}.png` — сгенерированные картинки
- `results/{generation_id}/song.mp3` — сгенерированная песня

**Lifecycle rules:**
- `uploads/` — TTL 24 часа (удаление после генерации)
- `results/` — TTL 30 дней

**Signed URLs:**
- Для скачивания результатов — TTL 1 час
- Pre-signed на download, не на upload (upload через backend для валидации)

### 5.4. OAuth провайдеры

Все три работают по Authorization Code Flow. Детали в разделе 6.

---

## 6. Авторизация и безопасность

### 6.1. Стратегия

- **httpOnly cookies** для access и refresh токенов
- **CSRF Double Submit Cookie** — дополнительный не-HttpOnly cookie + header
- **SameSite=None; Secure; Partitioned** — для совместимости с Telegram Mini App
- **JWT HS256** для подписи токенов
- **Access TTL 15 мин**, **Refresh TTL 30 дней**
- **Refresh rotation** при каждом обновлении

### 6.2. Жизненный цикл сессии

```
1. Login → выдаются access_token (cookie, 15 min) + refresh_token (cookie, 30d)
                                + csrf_token (non-httpOnly cookie, 15 min)

2. API request → cookies отправляются автоматически
                 → middleware проверяет access JWT
                 → если истёк → 401

3. 401 на фронте → автоматический POST /api/auth/refresh
                   → выдаются НОВЫЕ access + refresh + csrf
                   → старый refresh → blacklist
                   → retry оригинального запроса

4. Logout → refresh добавляется в blacklist
            → cookies стираются (Max-Age=-1)
```

### 6.3. Cookie атрибуты

```
Set-Cookie: access_token=eyJ...;
            HttpOnly; Secure; SameSite=None; Partitioned;
            Path=/; Domain=.fungreet.app; Max-Age=900

Set-Cookie: refresh_token=eyJ...;
            HttpOnly; Secure; SameSite=None; Partitioned;
            Path=/api/auth/refresh; Domain=.fungreet.app; Max-Age=2592000

Set-Cookie: csrf_token=abc123...;
            Secure; SameSite=None; Partitioned;
            Path=/; Domain=.fungreet.app; Max-Age=900
```

### 6.4. CORS конфигурация

```go
cors.Config{
    AllowOrigins: []string{
        "https://fungreet.app",
        "https://www.fungreet.app",
        "https://web.telegram.org",
    },
    AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
    AllowHeaders: []string{"Content-Type", "X-CSRF-Token", "Authorization"},
    AllowCredentials: true,  // обязательно для cookies
    MaxAge: 12 * time.Hour,
}
```

### 6.5. Чеклист безопасности

- [ ] HTTPS everywhere (включая dev через mkcert)
- [ ] State параметр в OAuth (CSRF для OAuth flow)
- [ ] PKCE для Google и Yandex
- [ ] JWT секреты в env, не в коде
- [ ] Шифрование OAuth tokens в БД через pgcrypto
- [ ] Rate limiting на auth эндпоинтах
- [ ] Email verification — опционально, для дополнительной защиты
- [ ] Логирование failed auth attempts (для мониторинга брутфорса)
- [ ] Валидация ВСЕХ входных данных на бэке
- [ ] Проверка magic bytes файлов (не только расширение)
- [ ] Проверка owner при каждом доступе к ресурсу

---

## 7. Worker pipeline

### 7.1. Архитектура worker'а

Worker — это горутина в том же бинарнике, что и HTTP-сервер (на старте). При росте легко отделяется в отдельный процесс.

```go
type Worker struct {
    redis       *redis.Client
    db          *sql.DB
    storage     *storage.R2
    gemini      *nanobnn.Client
    suno        *suno.Client
    audioProc   *audio.Preprocessor
    concurrency int  // количество одновременных задач
}

func (w *Worker) Start(ctx context.Context) error {
    semaphore := make(chan struct{}, w.concurrency)
    
    for {
        select {
        case <-ctx.Done():
            return nil
        default:
        }
        
        // BRPOP с таймаутом 5 секунд
        result, err := w.redis.BRPop(ctx, 5*time.Second, "gen_queue").Result()
        if err != nil {
            continue
        }
        
        var task GenerationTask
        json.Unmarshal([]byte(result[1]), &task)
        
        semaphore <- struct{}{}
        go func() {
            defer func() { <-semaphore }()
            w.processGeneration(ctx, task)
        }()
    }
}
```

### 7.2. Основной pipeline

```go
func (w *Worker) processGeneration(ctx context.Context, task GenerationTask) {
    startTime := time.Now()
    
    // 1. Обновить статус
    w.updateStatus(task.ID, "processing_image")
    
    // 2. Параллельно запустить картинки и музыку
    var images []string
    var audioKey string
    
    g, ctx := errgroup.WithContext(ctx)
    
    g.Go(func() error {
        var err error
        images, err = w.generateImages(ctx, task)
        return err
    })
    
    g.Go(func() error {
        var err error
        audioKey, err = w.generateSong(ctx, task)
        return err
    })
    
    if err := g.Wait(); err != nil {
        w.markFailed(task.ID, err)
        w.refundCredit(task.UserID, task.ID)
        return
    }
    
    // 3. Сохранить результат
    elapsed := time.Since(startTime).Milliseconds()
    w.markCompleted(task.ID, images, audioKey, elapsed)
}
```

### 7.3. Генерация картинок

```go
func (w *Worker) generateImages(ctx context.Context, task GenerationTask) ([]string, error) {
    var results []string
    
    for i, photoKey := range task.PhotoKeys {
        // Загрузка фото из R2
        photoData, err := w.storage.Download(ctx, photoKey)
        if err != nil {
            return nil, err
        }
        
        // Построение промпта
        prompt := buildImagePrompt(task.Occasion, task.ImagePrompt, task.RecipientName)
        
        // Генерация с retry при safety filter
        var imgBytes []byte
        for attempt := 0; attempt < 3; attempt++ {
            imgBytes, err = w.gemini.GenerateImage(ctx, prompt, photoData, "image/jpeg")
            if err == nil {
                break
            }
            if isSafetyFilterError(err) {
                prompt = simplifyPrompt(prompt)
                continue
            }
            return nil, err
        }
        
        if imgBytes == nil {
            return nil, fmt.Errorf("all attempts rejected by safety filter")
        }
        
        // Сохранение в R2
        key := fmt.Sprintf("results/%s/image_%d.png", task.ID, i)
        if err := w.storage.Upload(ctx, key, imgBytes, "image/png"); err != nil {
            return nil, err
        }
        
        results = append(results, key)
    }
    
    return results, nil
}
```

### 7.4. Генерация песни

```go
func (w *Worker) generateSong(ctx context.Context, task GenerationTask) (string, error) {
    w.updateStatus(task.ID, "processing_audio")
    
    // 1. Загрузить аудио из R2
    audioData, err := w.storage.Download(ctx, task.AudioKey)
    if err != nil {
        return "", err
    }
    
    // 2. Сохранить во временный файл
    tmpPath := fmt.Sprintf("/tmp/%s.mp3", task.ID)
    os.WriteFile(tmpPath, audioData, 0644)
    defer os.Remove(tmpPath)
    
    // 3. Валидация
    if err := w.audioProc.Validate(tmpPath); err != nil {
        return "", err
    }
    
    // 4. Каскадный fallback для обхода copyright
    processedPath, level, err := w.audioProc.ProcessWithFallbacks(tmpPath)
    if err != nil {
        return "", err
    }
    defer os.Remove(processedPath)
    
    w.updatePreprocessLevel(task.ID, level)
    
    // 5. Отправка в Suno через suno-api
    tracks, err := w.suno.CustomGenerate(ctx, suno.CustomGenerateRequest{
        Prompt: task.SongLyrics,
        Tags:   task.SongStyle,
        Title:  fmt.Sprintf("Поздравление для %s", task.RecipientName),
    })
    if err != nil {
        return "", err
    }
    
    // 6. Polling результата (таймаут 5 мин)
    ids := strings.Join(extractIDs(tracks), ",")
    pollCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
    defer cancel()
    
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-pollCtx.Done():
            return "", fmt.Errorf("suno generation timeout")
        case <-ticker.C:
            result, err := w.suno.GetTracks(ctx, ids)
            if err != nil {
                continue
            }
            if result[0].Status == "streaming" || result[0].Status == "complete" {
                // Скачать результат
                songData := downloadFromURL(result[0].AudioURL)
                
                key := fmt.Sprintf("results/%s/song.mp3", task.ID)
                if err := w.storage.Upload(ctx, key, songData, "audio/mpeg"); err != nil {
                    return "", err
                }
                
                return key, nil
            }
        }
    }
}
```

### 7.5. FFmpeg preprocessor

```go
type Preprocessor struct {
    workDir string
}

// Level 0: только нормализация
func (p *Preprocessor) Normalize(input, output string) error {
    return exec.Command("ffmpeg", "-y",
        "-i", input,
        "-codec:a", "libmp3lame",
        "-b:a", "128k",
        "-ar", "44100",
        "-ac", "2",
        output,
    ).Run()
}

// Level 1: stretch 1.01x (замедление на 1%)
func (p *Preprocessor) Stretch(input, output string) error {
    return exec.Command("ffmpeg", "-y",
        "-i", input,
        "-filter:a", "atempo=0.99",
        "-vn",
        output,
    ).Run()
}

// Level 2: noise + pitch shift
func (p *Preprocessor) NoiseAndPitch(input, output string) error {
    return exec.Command("ffmpeg", "-y",
        "-f", "lavfi", "-t", "0.3",
        "-i", "anoisesrc=c=white:r=44100:a=0.005",
        "-i", input,
        "-filter_complex",
        "[1:a]asetrate=44100*1.01,aresample=44100[pitched];[0:a][pitched]concat=n=2:v=0:a=1",
        "-vn",
        output,
    ).Run()
}

// Каскадная обработка — возвращает путь и уровень
func (p *Preprocessor) ProcessWithFallbacks(input string) (string, int, error) {
    // Всегда начинаем с нормализации
    normalized := filepath.Join(p.workDir, "normalized.mp3")
    if err := p.Normalize(input, normalized); err != nil {
        return "", -1, err
    }
    
    // Возвращаем нормализованный как начальный
    return normalized, 0, nil
    
    // Worker потом решит уровень в зависимости от ответа Suno:
    // - Если Suno принял → level 0, готово
    // - Если Suno отклонил → level 1 (stretch) → retry
    // - Если снова отклонил → level 2 (noise+pitch) → retry
    // - Если всё ещё отклонил → ошибка
}
```

### 7.6. Обработка ошибок

| Ошибка | Стратегия |
|--------|-----------|
| Сетевой timeout | Retry до 3 раз с exp backoff |
| 429 rate limit | Wait + retry |
| Safety filter (Gemini) | Упростить промпт, retry до 3 раз |
| Copyright detected (Suno) | Каскад FFmpeg обработок |
| Cookie invalid (Suno) | Алерт админу, fail generation, refund |
| 5xx от провайдера | Retry до 3 раз |
| Неожиданная ошибка | Fail, refund, log с stack trace |

Refund логика:

```go
func (w *Worker) refundCredit(userID int64, generationID string) error {
    tx, err := w.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Вернуть кредит (сначала в paid, если были; иначе в free)
    _, err = tx.Exec(`
        UPDATE users 
        SET paid_credits = paid_credits + 1 
        WHERE id = $1
    `, userID)
    if err != nil {
        return err
    }
    
    // Записать транзакцию
    _, err = tx.Exec(`
        INSERT INTO credit_transactions (user_id, amount, reason, reference_id, balance_after)
        SELECT $1, 1, 'refund', $2, free_credits + paid_credits FROM users WHERE id = $1
    `, userID, generationID)
    if err != nil {
        return err
    }
    
    return tx.Commit()
}
```

---

(Продолжение в следующей части — структура кода, инфраструктура, тесты, план)
