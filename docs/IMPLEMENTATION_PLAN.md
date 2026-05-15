# FunBaza Implementation Plan

Дата: 2026-04-29

Основа: `PROJECT_AUDIT.md`.

Цель плана: превратить аудит в исполнимую последовательность работ, где сначала восстанавливается базовая работоспособность и закрываются критичные риски, затем добавляются тесты, рефакторинг и архитектурные улучшения.

## Принципы Выполнения

- Делать маленькими PR/итерациями: один риск или один слой за раз.
- Каждую P0/P1 правку сопровождать минимальным тестом или проверкой.
- Не смешивать security fixes, dependency migrations и UI-рефакторинг в одном изменении.
- Сначала стабилизировать API-контракт, затем auth/billing, затем архитектуру worker/storage.
- После изменения API обновлять frontend client, Swagger/docs и тесты в той же итерации.

## Порядок Выполнения

| Этап | Приоритет | Цель | Результат |
|---|---:|---|---|
| 0. Baseline | P0 | Зафиксировать текущее состояние и воспроизводимость | Команды запуска/сборки понятны, текущие failures записаны |
| 1. Quick wins | P0 | Вернуть базовые запросы и закрыть явные дыры | API client работает, dev login закрыт, storage безопаснее, lock синхронизирован |
| 2. Минимальные тесты P0 | P0 | Не дать критичным багам вернуться | Unit/contract tests на URL, auth guard, storage paths |
| 3. Auth hardening | P1 | Подготовить cookie auth к production/TMA | Env-aware cookies, CSRF, refresh policy |
| 4. Billing hardening | P1 | Убрать гонку списания кредитов | Атомарное списание, тест параллельных charge |
| 5. Refactoring | P2 | Упростить сопровождение | Route registration, API client split, validation DTO |
| 6. Dependency migrations | P2 | Навести порядок в зависимостях | `package-lock`/`go.mod` консистентны |
| 7. Architecture changes | P3 | Подготовить масштабирование | Reliable queue, storage adapter, сниженный polling |
| 8. Docs and release checklist | P2 | Синхронизировать документацию | Swagger/docs соответствуют факту |

## Quick Wins

### 1. Исправить frontend API contract

Затронутые файлы:

- `frontend/src/lib/api.ts`
- `frontend/src/components/ChatInput.tsx`
- при необходимости `frontend/src/hooks/*`

Шаги:

1. Разделить base paths:
   - versioned API: `/api/v1`
   - auth API: `/api/auth`
2. Сделать `request('/user/me') -> /api/v1/user/me`, без повторного `/api/v1`.
3. Перевести `devLogin`, `logout`, `refresh` на auth base.
4. Перевести lyrics endpoint на `/api/v1/generations/lyrics`.
5. Убедиться, что multipart `generations.create` не добавляет JSON `Content-Type`.

Acceptance criteria:

- `useCurrentUser()` ходит в `/api/v1/user/me`.
- `devLogin()` ходит в `/api/auth/dev/login`.
- lyrics generation ходит в `/api/v1/generations/lyrics`.
- 401 refresh сохраняет retry behavior.

### 2. Закрыть dev login в production

Затронутые файлы:

- `backend/cmd/server/main.go`
- `backend/internal/handlers/auth.go`

Шаги:

1. Регистрировать `GET /api/auth/dev/login` только при `cfg.IsDev()`.
2. Добавить runtime guard в `DevLogin`, чтобы handler сам возвращал `404` или `403` вне development.
3. Обновить Swagger/docs: endpoint dev-only.

Acceptance criteria:

- При `APP_ENV=production` dev login недоступен.
- При `APP_ENV=development` dev login продолжает работать.

### 3. Закрыть path traversal в local storage

Затронутые файлы:

- `backend/internal/services/storage.go`
- `backend/cmd/server/main.go`

Шаги:

1. Добавить helper для безопасного разрешения ключа в путь:
   - запрет absolute path;
   - очистка slash/backslash;
   - запрет `..`;
   - проверка, что final path остается внутри `baseDir`.
2. Использовать helper в `Upload`, `Download`, `Delete`, `/api/files/*key`.
3. Возвращать `400` для unsafe key, `404` для отсутствующего файла.

Acceptance criteria:

- `../`, `%2e%2e`, mixed slash/backslash не читают файлы вне storage.
- Валидные ключи `uploads/<user>/<uuid>.png` и `results/<gen>/image_0.png` работают.

### 4. Синхронизировать frontend dependencies

Затронутые файлы:

- `frontend/package.json`
- `frontend/package-lock.json`

Шаги:

1. Принять решение по `react-router-dom`:
   - если роутинг нужен скоро, вернуть dependency в `package.json`;
   - если нет, удалить из lock через нормальный npm workflow.
2. Исправить форматирование `tailwind-merge` в `package.json`.
3. Проверить `npm ci` в `frontend`.

Acceptance criteria:

- `npm ci` в `frontend` проходит.
- Docker stage `RUN npm ci` не ломается из-за manifest/lock mismatch.

## Refactoring

### Backend route registration

Цель: оставить `main.go` composition root, но убрать из него длинный список routes.

Шаги:

1. Создать `backend/internal/server` или `backend/internal/routes`.
2. Вынести регистрацию:
   - public routes: health, swagger, auth, webhooks, files;
   - secured `/api/v1` routes;
   - static frontend fallback.
3. Передавать зависимости структурой `RouteDeps`, чтобы не разрастались параметры.
4. Добавить route-level тесты через `httptest`.

### Backend validation DTO

Цель: уменьшить ручную валидацию в handlers.

Шаги:

1. Для JSON endpoints использовать Gin binding tags.
2. Для multipart create вынести parsing/validation в отдельную функцию или request parser.
3. Централизовать лимиты:
   - image count;
   - song count;
   - max photos;
   - max file sizes;
   - allowed extensions/MIME.

### Frontend API client split

Цель: убрать смешивание versioned API, auth API и raw fetch.

Шаги:

1. `apiRequest()` для `/api/v1`.
2. `authRequest()` для `/api/auth`.
3. `uploadRequest()` или явный helper для `FormData`.
4. Единая обработка `ApiError`.
5. Unit tests на построение URL и refresh retry.

### UI state cleanup

Шаги:

1. Убрать лишний mount-only `applyTheme` effect.
2. Решить судьбу `App.css`: удалить, если не используется, или подключить осознанно.
3. Снизить inline-style churn постепенно, без большого переписывания UI.

## Архитектурные Изменения

### Reliable queue вместо простого Redis list

Проблема: `BRPop` удаляет task до обработки; при падении worker task может потеряться.

Варианты:

- Redis Streams with consumer groups.
- Reliable queue pattern: pending list + ack + reaper.
- Готовая Go-библиотека очередей, если она не усложняет MVP.

Порядок:

1. Добавить retry/dead-letter модель в терминах `generation_requests`.
2. Сначала покрыть текущий worker тестами.
3. Затем заменить queue implementation за интерфейсом.

### Storage adapter boundary

Проблема: env templates обещают R2, но runtime поддерживает только local.

Порядок:

1. Оформить `StorageService` как стабильный boundary.
2. Добавить `LocalStorage` path safety.
3. Затем реализовать S3/R2 adapter или убрать R2 из env/docs до готовности.
4. Добавить storage integration tests на local adapter.

### Auth production model

Порядок:

1. Env-aware cookie attributes.
2. CSRF middleware для state-changing routes.
3. Refresh rotation/blacklist в Redis.
4. OAuth providers: Google first, затем VK/Yandex по одному.
5. Frontend route/state для OAuth success/error.

### Client update strategy

Проблема: polling `500ms` в фоне плохо масштабируется.

Порядок:

1. Увеличить минимальный polling interval и отключить background polling для неактивного окна.
2. Добавить adaptive polling: быстро первые N секунд, затем медленнее.
3. Позже рассмотреть SSE/WebSocket для статуса генераций.

## Миграции Зависимостей

### Frontend

1. Синхронизировать `package.json` и `package-lock.json`.
2. Решить `react-router-dom`:
   - оставить и внедрить реальные routes;
   - или удалить как неиспользуемую зависимость.
3. Проверить совместимость TypeScript `~6.0.x` и `typescript-eslint`.
4. Добавить test stack:
   - Vitest;
   - React Testing Library;
   - MSW для API mocks, если нужны hook/component tests.

### Backend

1. Выполнить `go mod tidy`, чтобы реально используемые зависимости стали direct, а лишние ушли.
2. Принять один Postgres driver path:
   - сейчас используется `lib/pq` через `database/sql`;
   - `pgx/v5` есть в `go.mod`, но фактически может быть лишним.
3. Добавить test dependencies только по мере появления тестов:
   - `testify` опционально;
   - testcontainers только если integration tests действительно нужны.
4. После dependency changes проверить Docker build.

### Миграции БД

Потенциальные изменения:

1. Для billing hardening может понадобиться таблица/materialized balance:
   - `user_credit_balances(user_id, balance, updated_at)`;
   - транзакционный update с проверкой `balance >= amount`.
2. Для refresh rotation может понадобиться blacklist/session table или Redis-only схема.
3. Для queue reliability может понадобиться расширение `generation_requests`:
   - `locked_at`;
   - `next_retry_at`;
   - `last_error`;
   - `worker_id`.

Любая DB migration должна иметь `.up.sql` и `.down.sql`, плюс тест или ручной сценарий rollback.

## Тесты

### P0 tests

Backend:

- `storage_test.go`: unsafe keys cannot escape base dir.
- `auth_test.go`: dev login unavailable outside development.
- `routes_test.go`: secured endpoints are under `/api/v1`.

Frontend:

- `api.test.ts`: URL construction for auth/versioned endpoints.
- `api.test.ts`: refresh on 401 retries original request once.
- `ChatInput` или lower-level helper test: lyrics endpoint uses `/api/v1`.

### P1 tests

Backend:

- `billing_test.go`: concurrent `Charge` cannot overspend.
- `csrf_test.go`: unsafe methods require CSRF token when cookie auth is active.
- `cookie_test.go`: cookie attributes differ correctly for dev/prod.
- `generation_test.go`: invalid multipart inputs return stable error codes.

Frontend:

- auth flow hook tests with mocked 401/refresh/success.
- logout clears user/balance/session query cache.

### P2/P3 tests

- Worker retry and failure refund tests.
- Queue reliability tests around crash/retry semantics.
- Storage adapter contract tests shared by local and R2/S3 adapter.
- Smoke E2E for login -> create generation -> poll result using mock generators.

## Риски

| Риск | Вероятность | Влияние | Митигация |
|---|---:|---:|---|
| P0 URL fix вскроет дополнительные backend/frontend contract bugs | High | High | Делать с contract tests и ручной smoke-проверкой auth + sessions + generation |
| Закрытие dev login сломает локальную разработку | Medium | Medium | Оставить dev-only route при `APP_ENV=development`, добавить понятную ошибку вне dev |
| Cookie hardening сломает localhost auth | Medium | High | Явно разделить dev/prod cookie policy, не включать `Secure/SameSite=None` без HTTPS в dev |
| CSRF добавит failures во все POST/PATCH | Medium | High | Внедрять после единого API client, сначала покрыть тестами |
| Billing migration может затронуть существующие balances | Medium | High | Написать backfill, проверить суммы ledger, сделать rollback plan |
| Queue replacement может потерять совместимость с worker flow | Medium | High | Сначала ввести интерфейс и тесты, затем менять implementation |
| R2/S3 adapter усложнит storage URLs и permissions | Medium | Medium | Оставить local adapter стабильным, R2 включать feature flag/env mode |
| Dependency updates могут ломать Vite/TS/ESLint | Medium | Medium | Обновлять по одному слою, фиксировать lock, запускать `npm ci`, `lint`, `build` |

## Детальный Execution Checklist

### Этап 0: Baseline

1. Проверить текущее состояние git worktree.
2. Записать текущие результаты:
   - `cd frontend && npm ci`
   - `cd frontend && npm run lint`
   - `npm run build`
   - `cd backend && go test ./...`
3. Если команды уже падают, зафиксировать failures до исправлений.

### Этап 1: P0 quick wins

1. Исправить `frontend/src/lib/api.ts`.
2. Исправить lyrics endpoint.
3. Добавить тесты URL construction.
4. Закрыть dev login route/handler.
5. Добавить storage path safety.
6. Синхронизировать frontend manifest/lock.
7. Проверить:
   - `cd frontend && npm ci`
   - `cd frontend && npm run lint`
   - `npm run build`
   - `cd backend && go test ./...`

### Этап 2: Auth and billing hardening

1. Ввести env-aware cookie config.
2. Добавить CSRF middleware и frontend header support.
3. Исправить concurrent billing charge.
4. Исправить traceability lyrics charge/refund.
5. Добавить P1 tests.

### Этап 3: Docs and contracts

1. Обновить Swagger annotations.
2. Перегенерировать `backend/docs`.
3. Обновить `docs/04_FunBaza_DevStatus.md` под `/api/v1`.
4. Добавить contract tests для ключевых endpoint paths.

### Этап 4: Refactoring

1. Вынести routes из `main.go`.
2. Вынести validation/parsing из generation handler.
3. Разделить frontend API client на auth/versioned/upload helpers.
4. Почистить неиспользуемый CSS/deps.

### Этап 5: Architecture

1. Ввести queue interface и покрыть текущую семантику тестами.
2. Реализовать reliable queue.
3. Уточнить storage roadmap: R2 adapter или удаление обещаний из env/docs.
4. Снизить polling и подготовить SSE/WebSocket option.

## Definition of Done

Для каждой итерации:

- Изменение покрыто релевантным тестом или явно описанной ручной проверкой.
- `frontend/package.json` и lock синхронизированы, если менялись зависимости.
- `go.mod`/`go.sum` tidied, если менялись Go deps.
- API docs обновлены, если менялись routes/contracts.
- Security-sensitive поведение проверено в dev и production-like env.
- Нет unrelated refactor churn в файлах вне области задачи.
