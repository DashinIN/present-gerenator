# FunGreet — Текущее состояние разработки

> Дата: 2026-04-24  
> Ветка: `master`

---

## Статус проекта

| Компонент | Статус | Примечание |
|-----------|--------|------------|
| Backend (Go/Gin) | ✅ Работает | Полный REST API, воркеры |
| Frontend (React/TS) | ✅ Работает | Чат-интерфейс, загрузка файлов |
| PostgreSQL | ✅ Работает | Миграции применяются автоматически |
| Redis | ✅ Работает | Очередь генераций |
| Swagger UI | ✅ Работает | `/swagger/index.html` |
| AI генерация изображений | ⚠️ Mock | `MockImageGenerator` — заглушка |
| AI генерация песен | ⚠️ Mock | `MockSongGenerator` — заглушка |
| OAuth (Google/VK) | ⚠️ Заготовка | Маршруты есть, хэндлеры не реализованы |
| Cloudflare R2 | ⚠️ Не активен | `STORAGE_MODE=local` |
| Оплата | ❌ Нет | Только кредитная логика без платёжного шлюза |

---

## Архитектура

```
┌─────────────────────────────────────────────────────┐
│                     Frontend (React)                 │
│  LoginPage │ ChatPage │ Sidebar │ ChatThread         │
│  Vite + Tailwind + React Query                       │
│  Proxy: /api/* → :8080                              │
└─────────────────────┬───────────────────────────────┘
                      │ HTTP (cookies)
┌─────────────────────▼───────────────────────────────┐
│               Backend (Go + Gin)  :8080              │
│                                                      │
│  Handlers → Services → Repositories → PostgreSQL     │
│                  ↓                                   │
│            Worker Queue (Redis)                      │
│                  ↓                                   │
│         MockImageGen / MockSongGen                   │
│                  ↓                                   │
│         LocalStorage (./data/uploads)               │
└─────────────────────────────────────────────────────┘
```

---

## Эндпоинты API

Полная интерактивная документация: **`http://localhost:8080/swagger/index.html`**

### Auth

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/api/auth/dev/login` | — | Dev-логин (только development) |
| POST | `/api/auth/refresh` | cookie | Обновить access token |
| POST | `/api/auth/logout` | cookie | Выйти, очистить cookies |

### User

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/api/user/me` | ✅ | Профиль текущего пользователя |

### Billing

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/api/billing/balance` | ✅ | Баланс кредитов |
| GET | `/api/billing/tariff` | ✅ | Активный тариф |
| GET | `/api/billing/estimate` | ✅ | Предварительная стоимость |
| GET | `/api/billing/transactions` | ✅ | История транзакций |

### Sessions

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| GET | `/api/sessions` | ✅ | Список сессий (тредов) |
| GET | `/api/sessions/:id` | ✅ | Сессия + все генерации |
| PATCH | `/api/sessions/:id` | ✅ | Переименовать сессию |

### Generations

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| POST | `/api/generations` | ✅ | Создать генерацию (multipart) |
| GET | `/api/generations` | ✅ | Список генераций |
| GET | `/api/generations/:id` | ✅ | Детали генерации |
| GET | `/api/generations/:id/status` | ✅ | Статус для polling |

### Uploads & Files

| Метод | Путь | Auth | Описание |
|-------|------|------|----------|
| POST | `/api/uploads` | ✅ | Загрузить файл |
| GET | `/api/files/*` | — | Скачать файл |

---

## Модели данных

### User
```json
{
  "id": 42,
  "email": "user@example.com",
  "display_name": "Иван",
  "avatar_url": "",
  "created_at": "2026-04-24T10:00:00Z"
}
```

### GenerationRequest (статусы: `pending → processing_images → processing_audio → completed | failed`)
```json
{
  "id": "uuid",
  "session_id": "uuid",
  "status": "completed",
  "recipient_name": "Мама",
  "occasion": "День рождения",
  "image_count": 3,
  "song_count": 1,
  "result_images": ["http://localhost:8080/api/files/..."],
  "result_audios": ["http://localhost:8080/api/files/..."],
  "credits_spent": 70,
  "created_at": "2026-04-24T10:00:00Z"
}
```

### CreditTransaction (types: `initial_grant`, `generation_charge`, `generation_refund`, `purchase`)
```json
{
  "id": 1,
  "user_id": 42,
  "amount": -70,
  "type": "generation_charge",
  "description": "3 images, 1 songs",
  "created_at": "2026-04-24T10:00:00Z"
}
```

---

## Аутентификация

- **Access Token** — httpOnly cookie `access_token`, TTL 15 мин
- **Refresh Token** — httpOnly cookie `refresh_token`, TTL 30 дней, path `/api/auth/refresh`
- Автоматическое обновление на клиенте через interceptor в `lib/api.ts`
- Cookie: `HttpOnly=true`, `Secure=false` (dev), `SameSite=Strict`

---

## Запуск локально

```bash
# 1. Инфраструктура (Postgres + Redis в Docker)
npm run infra

# 2. Backend + Frontend одновременно
npm run dev

# Отдельно:
npm run back   # Go backend :8080
npm run front  # Vite frontend :5173
```

Swagger доступен на: `http://localhost:8080/swagger/index.html`

---

## Переменные окружения (backend/.env)

```env
APP_ENV=development
APP_PORT=8080
DATABASE_URL=postgres://fungreet:fungreet@localhost:5433/fungreet?sslmode=disable
REDIS_URL=redis://localhost:6379
JWT_SECRET=dev-secret-key-min-32-characters-long!!
STORAGE_MODE=local
STORAGE_LOCAL_DIR=./data/uploads
```

⚠️ **`backend/.env` не коммитится в git** (добавлен в `.gitignore`).  
Шаблон: `backend/.env.example`.

---

## Структура репозитория

```
present generator/
├── backend/
│   ├── cmd/server/main.go        # Точка входа
│   ├── docs/                     # Сгенерированный Swagger (swag init)
│   ├── internal/
│   │   ├── config/               # Загрузка конфига из .env
│   │   ├── handlers/             # HTTP хэндлеры
│   │   ├── middleware/           # Auth, логгер, recovery
│   │   ├── models/               # Структуры данных
│   │   ├── repository/           # Слой работы с БД
│   │   ├── services/             # Бизнес-логика + mock генераторы
│   │   └── worker/               # Async воркер (Redis queue)
│   ├── migrations/               # SQL миграции
│   └── .env.example              # Шаблон переменных окружения
├── frontend/
│   ├── src/
│   │   ├── components/           # UI компоненты
│   │   ├── hooks/                # React Query хуки
│   │   ├── lib/                  # API клиент, типы, утилиты
│   │   └── pages/                # LoginPage, ChatPage
│   └── vite.config.ts
├── docs/                         # Проектная документация
├── docker-compose.yml            # Production Docker
├── Dockerfile                    # Multi-stage build
└── .env.example                  # Шаблон для Docker Compose
```

---

## Что нужно сделать для production

1. **Заменить mock-генераторы** — подключить реальные AI API (Stability AI, Suno и т.п.)
2. **Реализовать OAuth** — Google/VK/Yandex хэндлеры
3. **Подключить Cloudflare R2** — `STORAGE_MODE=r2` + credentials
4. **Платёжный шлюз** — ЮKassa / Stripe для пополнения кредитов
5. **HTTPS + secure cookies** — при деплое установить `Secure=true` в setCookie
6. **Мониторинг** — логи в slog/json уже есть, добавить метрики (Prometheus)
