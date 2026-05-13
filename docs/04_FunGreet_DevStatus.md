# FunGreet — Текущее состояние разработки

> Дата обновления: 2026-05-05  
> Ветка: `master`  
> Основной локальный URL: `http://localhost:8080/`

---

## Краткий итог

FunGreet находится в рабочем состоянии как web MVP:

- есть backend на Go/Gin с очередью, биллингом, сессиями и генерациями
- frontend работает как единое SPA, раздается из backend Docker-контейнера
- авторизация через Google OAuth уже внедрена и работает через httpOnly cookies
- кредитная логика активна: стартовый бонус и ежедневное пополнение до лимита `50`
- генерация изображений и песен подключена к `kie.ai`
- UI ввода заметно упрощен: выбор модели изображения вынесен в popover, работа с текстом песни вынесена в модалку

---

## Статус компонентов

| Компонент | Статус | Примечание |
|-----------|--------|------------|
| Backend (Go/Gin) | ✅ Работает | REST API, воркеры, Redis queue |
| Frontend (React/Vite/TS) | ✅ Работает | SPA, чатовый интерфейс, сессии, polling |
| PostgreSQL | ✅ Работает | Миграции применяются автоматически при старте |
| Redis | ✅ Работает | Очередь задач и фоновые воркеры |
| Docker local stack | ✅ Работает | `backend`, `postgres`, `redis` |
| Swagger UI | ✅ Работает | `http://localhost:8080/swagger/index.html` |
| Google OAuth | ✅ Работает | login + callback + cookie auth |
| Dev login | ✅ Работает | Только в `APP_ENV=development` |
| AI генерация изображений | ✅ Работает | `kie.ai`, модели `gpt-image-2`, `flux-2-flex`, `seedream-5-lite` |
| AI генерация песен | ✅ Работает | `kie.ai` |
| Генерация текста песни | ✅ Работает | До 3 вариантов за один запрос с выбором на фронте |
| Кредитная система | ✅ Работает | charge / refund / initial grant / daily grant |
| История сессий | ✅ Работает | session thread + rename |
| Cloudflare R2 | ⚠️ Не включен | сейчас `STORAGE_MODE=local` |
| Платежный шлюз | ❌ Нет | покупка кредитов еще не реализована |

---

## Что сделано в последнем цикле работ

### 1. Авторизация

- добавен реальный вход через Google OAuth
- добавлены env-переменные:
  - `GOOGLE_CLIENT_ID`
  - `GOOGLE_CLIENT_SECRET`
  - `GOOGLE_REDIRECT_URI`
- устранена проблема `invalid_oauth_state`
- устранена проблема смешения `localhost` и `127.0.0.1` после callback
- cookies продолжают использовать текущую JWT-схему:
  - `access_token`
  - `refresh_token`

### 2. Генерации и биллинг

- исправлена ошибка вставки `generation_requests` при пустом `input_audio_keys`
- восстановлен сценарий “только картинка без песни”
- добавлен cleanup при неуспешном создании:
  - refund при ошибке создания генерации
  - удаление пустой новой сессии, если генерация не создалась
  - удаление `generation` и refund, если задача не встала в очередь

### 3. Frontend UX

- упрощено облачко сообщения пользователя в треде:
  - оставлен только текст и мини-превью картинки
  - убраны повтор модели, повтор song prompt / lyrics и кредиты
- переработан `ChatInput`
  - выбор модели изображения вынесен в popover
  - текст песни вынесен в модалку
  - стиль песни вынесен рядом с кнопкой открытия модалки
  - генерация текста песни теперь возвращает 3 варианта
  - пользователь выбирает один вариант и может его доработать

---

## Актуальная архитектура

```text
┌─────────────────────────────────────────────────────┐
│                 Frontend SPA (React)                │
│  LoginPage │ ChatPage │ Sidebar │ ChatThread        │
│  ChatInput (compact composer + lyrics modal)        │
└─────────────────────┬───────────────────────────────┘
                      │ HTTP + cookies
┌─────────────────────▼───────────────────────────────┐
│              Backend (Go + Gin) :8080              │
│                                                     │
│  Handlers → Services → Repositories → PostgreSQL    │
│                  ↓                                  │
│               Redis Queue                           │
│                  ↓                                  │
│                Worker                               │
│                  ↓                                  │
│          kie.ai image/song generation               │
│                  ↓                                  │
│         LocalStorage (./data/uploads)               │
└─────────────────────────────────────────────────────┘
```

---

## Актуальные API-маршруты

Полная интерактивная документация:

- `http://localhost:8080/swagger/index.html`

### Auth

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/api/auth/dev/login` | — | Dev login, только development |
| GET | `/api/auth/google/login` | — | Начало Google OAuth |
| GET | `/api/auth/google/callback` | — | Завершение Google OAuth |
| POST | `/api/auth/refresh` | cookie | Обновление access token |
| POST | `/api/auth/logout` | cookie | Выход |

### User / Billing / Sessions / Generations

Все прикладные эндпоинты живут под `/api/v1/*`.

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/v1/user/me` | Профиль текущего пользователя |
| GET | `/api/v1/billing/balance` | Баланс |
| GET | `/api/v1/billing/tariff` | Активный тариф |
| GET | `/api/v1/billing/estimate` | Предварительная стоимость |
| GET | `/api/v1/billing/transactions` | История транзакций |
| GET | `/api/v1/sessions` | Список сессий |
| GET | `/api/v1/sessions/:id` | Сессия и все генерации |
| PATCH | `/api/v1/sessions/:id` | Переименование сессии |
| POST | `/api/v1/generations` | Создать генерацию |
| POST | `/api/v1/generations/lyrics` | Сгенерировать варианты текста песни |
| GET | `/api/v1/generations` | Список генераций |
| GET | `/api/v1/generations/:id` | Детали генерации |
| GET | `/api/v1/generations/:id/status` | Polling статуса |
| POST | `/api/v1/uploads` | Загрузка файлов |

### Service

| Метод | Путь | Описание |
|-------|------|----------|
| GET | `/api/health` | Health check |
| GET | `/api/files/*key` | Раздача файлов |

---

## Текущая логика кредитов

- новый пользователь получает `initial_grant`
- при входе пользователя endpoint `GET /api/v1/user/me` пытается выдать `daily_grant`
- ежедневное пополнение работает до лимита `50`
- при ошибке создания генерации делается refund
- покупка кредитов еще не реализована

---

## Актуальный UX ввода

### Картинка

- основной prompt в основной форме
- выбор модели изображения через compact popover
- можно приложить до 3 фото

### Песня

- блок песни занимает мало места в основном composer
- кнопка `Текст песни` открывает модалку
- рядом в основном composer задается `Стиль песни`
- в модалке:
  - prompt для генерации текста
  - генерация 3 вариантов
  - выбор одного варианта
  - ручное редактирование выбранного текста

---

## Локальный запуск

### Рекомендуемый способ

```bash
docker compose -f backend/docker-compose.yml up -d --build backend
```

После этого доступны:

- приложение: `http://localhost:8080/`
- swagger: `http://localhost:8080/swagger/index.html`

### Дополнительно

```bash
cd backend && go test ./...
cd frontend && npm run build
```

---

## Ключевые переменные окружения backend

```env
APP_ENV=development
APP_PORT=8080
DATABASE_URL=postgres://fungreet:fungreet@localhost:5433/fungreet?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=...
GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URI=http://localhost:8080/api/auth/google/callback
STORAGE_MODE=local
STORAGE_LOCAL_DIR=./data/uploads
KIE_API_KEY=...
```

---

## Известные ограничения

1. Нет покупки кредитов и платежного шлюза.
2. Хранилище файлов только локальное, `R2` пока не включен.
3. Cookie-настройки все еще dev-ориентированы.
   Сейчас проект стабилен локально, но production-hardening по cookies и CSRF еще нужен.
4. Документация в `docs/02_*` и `docs/03_*` частично описывает более широкий целевой план, чем реально реализовано в текущем MVP.
5. Нет полноценного тестового покрытия frontend.

---

## Результаты работ на текущий момент

- Google OAuth внедрен и работает end-to-end
- приложение раздается единым контейнером через `localhost:8080`
- generation flow стабилизирован для image-only сценариев
- composer упрощен и стал компактнее
- lyrics flow стал интерактивным: 3 варианта + выбор + редактирование
- проект готов к следующему этапу: платежи, production-hardening auth/cookies, подключение cloud storage
