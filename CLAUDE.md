# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

**FunGreet** — AI-сервис генерации персонализированных поздравлений (изображения + песни). Монолит на Go с React TS фронтендом. Каналы: Web (MVP), Telegram Mini App (следующий этап).

## Architecture

```
Browser (React TS)
       ↓ /api/*
Backend (Go, Gin)  ←→  PostgreSQL (source of truth)
       ↓                Redis (gen_queue + webhook store)
   Worker goroutine      local storage / Cloudflare R2 (файлы)
       ↓
   KieImageGenerator    — реальная генерация картинок через kie.ai (z-image)
   SunoAPIGenerator     — реальная генерация песен через sunoapi.org
   (fallback: MockImageGenerator / MockSongGenerator, если ключи не заданы)
```

**Одна Go-бинарка** содержит HTTP-сервер и воркер (запускается горутиной). Воркер читает из Redis-очереди (`gen_queue`) через `BRPop` и обрабатывает генерации.

В production фронтенд собирается в `./web/dist` и раздаётся сервером через `r.Static` + SPA fallback. В dev — Vite dev server проксирует `/api` на `:8080`.

### Key packages

| Path | Role |
|------|------|
| `backend/cmd/server/main.go` | Точка входа, инициализация, роутинг |
| `backend/internal/models/` | Все доменные типы (User, GenerationRequest, …) |
| `backend/internal/repository/` | SQL-запросы (database/sql + lib/pq) |
| `backend/internal/services/` | JWT, billing, storage, генераторы |
| `backend/internal/services/kie_image.go` | KieImageGenerator (kie.ai, z-image модель) |
| `backend/internal/services/suno.go` | SunoAPIGenerator (sunoapi.org) |
| `backend/internal/worker/` | Очередь + воркер + WebhookStore (Redis) |
| `backend/internal/handlers/` | HTTP-хендлеры (Gin) |
| `backend/internal/handlers/webhook.go` | Webhook-хендлеры kie.ai + sunoapi.org |
| `backend/internal/middleware/` | Auth (cookie), logger, recovery |
| `frontend/src/lib/api.ts` | API-клиент с авторазворачиванием токенов |
| `frontend/src/pages/ChatPage.tsx` | Основной экран (сессии + чат-поток генераций) |
| `frontend/src/components/ChatInput.tsx` | Форма отправки (фото, промт, песня) |
| `frontend/src/components/ChatThread.tsx` | Рендер истории генераций |

### Auth

- Аутентификация через httpOnly cookie `access_token` (JWT).
- `POST /api/auth/refresh` — обновление токена; вызывается автоматически из `api.ts` при 401.
- Dev-вход: `GET /api/auth/dev/login` (создаёт/возвращает тестового пользователя).
- OAuth (Google/VK/Yandex) — в роадмапе, не реализовано.

### Credits / Billing

Кредитная система: новый пользователь получает `initial_grant`. За генерацию списывается по тарифу (`tariff_id`). При ошибке — `generation_refund`. Транзакции в таблице `credit_transactions`.

### Storage

`STORAGE_MODE=local` — файлы в `./data/uploads`. `STORAGE_MODE=r2` — Cloudflare R2 (настройки через `R2_*` переменные). Файлы отдаются через `/api/files/*key` с `Content-Disposition: attachment`.

`StorageService` интерфейс: `Upload`, `GetURL`, `Download`, `Delete`. `GetURL` собирает абсолютный URL вида `{BASE_URL}/api/files/{key}` — используется при резолве `result_images`, `result_audios`, `input_photos` перед отдачей клиенту.

## Generation Flow

### UI (ChatInput)

Два независимо включаемых блока:
- **Картинка** (по умолчанию включена): `image_count = 1`, поле `image_prompt`; можно прикрепить фото (`input_photos`) — передаются как ref-images.
- **Песня** (по умолчанию включена): `song_count = 1`; два режима:
  - «Написать текст» — поле `song_lyrics` (пользователь пишет сам)
  - «Сгенерировать текст» — поле `song_prompt` (ИИ генерирует текст перед аудио)
  
Нельзя выключить оба блока одновременно.

### Request lifecycle

```
POST /api/generations
  → списание кредитов (billing.Charge)
  → INSERT generation_requests (status=pending)
  → RPUSH gen_queue
  → 202 + {id, status}

Worker (BRPop gen_queue)
  → если BASE_URL задан → processAsync (webhook-режим)
  → иначе              → process (polling-режим)

[polling] process():
  → errgroup: imageGen.Generate() + songGen.Generate() параллельно
  → загрузка файлов в storage
  → UpdateResults() → status=completed

[async] processAsync():
  → imageGen.Submit(cbURL=/api/webhooks/kie) → RegisterTask в Redis
  → songGen.Submit(cbURL=/api/webhooks/suno) → RegisterTask в Redis
  → InitPending(genId, [image, song])
  → UpdateStatus → processing_images

POST /api/webhooks/kie  (от kie.ai)
  → LookupTask(taskId) → genId
  → скачать картинки → AppendImages → CompletePending(image)
  → если все pending выполнены → UpdateResults → completed

POST /api/webhooks/suno  (от sunoapi.org)
  → LookupTask(taskId) → genId
  → скачать аудио → AppendAudios → CompletePending(song)
  → если все pending выполнены → UpdateResults → completed

GET /api/generations/{id}/status
  → отдаёт текущий статус + result_images + result_audios + input_photos
    (все ключи резолвятся в абсолютные URL через storage.GetURL)

Фронт поллит статус каждые 3 сек пока status != completed|failed
```

### Image generation (kie.ai)

`KieImageGenerator` (`services/kie_image.go`):
- Модель: `z-image` ($0.004/image, text-to-image only, aspect_ratio обязателен)
- `Submit(ctx, prompt, refImages, callbackURL)` → taskId (async-режим)
- `Generate(ctx, prompt, refImages, count)` → `[][]byte` (sync-режим через внутренний poll)
- Ref-images (input_photos пользователя) загружаются в kie.ai File Upload API перед submit, но z-image их не использует — заготовлено для будущей image-to-image модели
- Poll: каждые 5 сек, таймаут 5 мин; состояния: `generating` → `success` / `fail`
- `success`: парсим `resultJson.resultUrls`, скачиваем и сохраняем в storage

### Song generation (sunoapi.org)

`SunoAPIGenerator` реализует три интерфейса:
- `SongGenerator` — `Generate(ctx, lyrics, style, count)` → `[][]byte`
- `LyricsGenerator` — `GenerateLyrics(ctx, prompt)` → `(text, title, error)`
- `StreamingSongGenerator` — `GenerateStreaming(...)` с partial-callback (отдаёт первый клип до завершения второго)
- `AsyncSongGenerator` — `Submit(ctx, lyrics, style, callbackURL)` → taskId

Воркер type-assert-ит на нужный интерфейс по контексту (polling vs async, lyrics vs lyrics+audio).

API sunoapi.org:
- `POST /api/v1/generate` — submit задачи
- `GET /api/v1/generate/record-info?taskId=...` — polling (PENDING → TEXT_SUCCESS → FIRST_SUCCESS → SUCCESS)
- `POST /api/v1/generate/lyrics` + `GET /api/v1/generate/lyrics/record-info?taskId=...` — генерация текста
- Ответ: `data.response.sunoData[].audioUrl`
- Polling интервал: 5 сек, таймаут: 10 мин

## Development Commands

### Backend

```bash
cd backend

# Запустить инфраструктуру (Postgres :5433, Redis :6379)
docker compose up -d

# Собрать и запустить сервер (читает .env автоматически через godotenv)
go build -o bin/server.exe ./cmd/server && ./bin/server.exe

# Применить миграции напрямую через psql (golang-migrate driver конфликтует с lib/pq):
cat migrations/000004_add_song_prompt.up.sql | psql "postgres://fungreet:fungreet@localhost:5433/fungreet?sslmode=disable"
# После применения вручную обновить schema_migrations:
echo "INSERT INTO schema_migrations (version, dirty) VALUES (4, false) ON CONFLICT DO NOTHING;" | psql "..."

# Остановить инфраструктуру
docker compose down
```

> **Важно**: `make` на Windows может не работать. Использовать команды выше напрямую.  
> Перед запуском нужен `backend/.env` (см. `.env.example`).

### Frontend

```bash
cd frontend
npm install
npm run dev      # Vite dev server, проксирует /api → localhost:8080
npm run build    # Сборка в dist/
npm run lint
```

### Swagger UI

Доступен на `http://localhost:8080/swagger/index.html`. Docs генерируются из аннотаций swaggo.

```bash
cd backend
swag init -g cmd/server/main.go
```

После регенерации нужно пересобрать бинарник.

## Database Schema

Ключевые таблицы: `users`, `user_identities` (один юзер → N провайдеров), `credit_transactions`, `tariffs`, `generation_sessions`, `generation_requests`.

Поля `generation_requests`:
- `image_prompt` — промт для изображений
- `image_count` — 0 или 1
- `song_count` — 0 или 1
- `input_photos` — `TEXT[]` ключи storage прикреплённых фото пользователя
- `song_prompt` — промт для генерации текста (режим «Сгенерировать»)
- `song_lyrics` — текст песни (заполняется пользователем или воркером после GenerateLyrics)
- `song_style` — стиль (теги: "pop, dance", "jazz" и т.д.)
- `result_images` — `TEXT[]` ключи storage сгенерированных картинок
- `result_audios` — `TEXT[]` ключи storage сгенерированного аудио
- `status` — `pending` | `processing_images` | `processing_audio` | `completed` | `failed`

Миграции в `backend/migrations/` (файлы `NNN_name.up/down.sql`). Текущая версия схемы: **4**.

## Error Response Format

```json
{ "error": { "code": "snake_case_code", "message": "Human readable" } }
```

## Environment Variables

| Переменная | Обязательна | Описание |
|---|---|---|
| `DATABASE_URL` | да | Postgres DSN |
| `JWT_SECRET` | да | Секрет для JWT (мин. 32 символа) |
| `STORAGE_MODE` | нет | `local` (по умолчанию) или `r2` |
| `STORAGE_LOCAL_DIR` | нет | Путь для локальных файлов (default: `./data/uploads`) |
| `KIE_API_KEY` | нет | API ключ kie.ai; если пусто — используется MockImageGenerator |
| `SUNO_API_KEY` | нет | API ключ sunoapi.org; если пусто — используется MockSongGenerator |
| `REDIS_URL` | нет | Redis DSN (default: `redis://localhost:6379`) |
| `APP_PORT` | нет | HTTP порт (default: `8080`) |
| `APP_ENV` | нет | `development` / `production` |
| `BASE_URL` | нет | Публичный URL сервера; если задан — воркер использует webhook-режим вместо polling |

При `STORAGE_MODE=r2` дополнительно нужны `R2_ACCOUNT_ID`, `R2_ACCESS_KEY`, `R2_SECRET_KEY`, `R2_BUCKET`.
