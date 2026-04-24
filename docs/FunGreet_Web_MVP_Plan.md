# FunGreet Web MVP — План разработки

## Параметры планирования

- **Темп:** 2–3 часа/день, ~15 часов в неделю
- **Разработка:** один full-stack
- **Scope:** веб-версия с авторизацией и бесплатными кредитами (без оплаты, без TG Mini App)
- **Общий срок:** ~12 недель (3 месяца)

Принцип планирования: каждая неделя должна заканчиваться **рабочим артефактом**, который можно пощупать. Никакого "две недели пишу архитектуру, потом ничего не работает".

---

## Что войдёт в MVP

**Функционал:**
- Регистрация/вход через Google, VK, Yandex
- 3 бесплатных кредита каждому при регистрации
- Форма создания поздравления (фото + промпт + текст песни + аудио)
- Генерация картинок (Nano Banana free tier) + песни (self-hosted suno-api)
- Просмотр результата + скачивание
- История своих генераций

**НЕ войдёт** (будущие итерации):
- Оплата
- Telegram Mini App
- Реферальная программа
- Продвинутые фичи Suno (Mashup, stems)

---

## Неделя 1 — Фундамент бэкенда

**Цель:** HTTP-сервер поднимается, health check работает.

### День 1–2: Структура проекта

- Инициализация Go модуля: `go mod init github.com/you/fungreet`
- Базовая структура:
  ```
  backend/
  ├── cmd/server/main.go
  ├── internal/
  │   ├── config/
  │   ├── handlers/
  │   ├── middleware/
  │   ├── models/
  │   ├── repository/
  │   └── services/
  ├── migrations/
  ├── docker-compose.yml
  ├── Dockerfile
  └── .env.example
  ```
- Gin + конфиг через env (godotenv для dev)
- Graceful shutdown
- Health endpoint `GET /api/health`

### День 3–4: Docker + Postgres + Redis

- `docker-compose.yml`: postgres:16 + redis:7 + backend (hot reload через air)
- Подключение к Postgres: `database/sql` + `pgx` driver
- Подключение к Redis: `github.com/redis/go-redis/v9`
- Проверка соединений при старте — если БД недоступна, сервер не стартует

### День 5: Миграции

- `golang-migrate/migrate` CLI
- Первая миграция: таблицы `users` и `user_identities`
- Make-команды: `make migrate-up`, `make migrate-down`, `make migrate-create name=xxx`

### День 6–7: Логирование + структурированные ошибки

- `log/slog` для структурированного логирования (stdlib, не внешние зависимости)
- Middleware request logger
- Общий формат ошибок API: `{ "error": { "code": "...", "message": "..." } }`
- Recovery middleware (panic → 500, не падение сервера)

**Артефакт недели:** `curl https://localhost:8080/api/health` возвращает 200 OK, в логах видно структурированные JSON-записи.

---

## Неделя 2 — Авторизация: Google OAuth

Начинаем с одного провайдера, чтобы отладить всю цепочку. Остальные добавятся по шаблону.

### День 1: Подготовка

- Регистрация приложения в Google Cloud Console
- Получение `CLIENT_ID` и `CLIENT_SECRET`
- Настройка redirect URI: `https://localhost:8443/api/auth/google/callback`
- Установка `mkcert` для локального HTTPS

### День 2–3: OAuth flow

- `GET /api/auth/google/login` — генерация state, PKCE, redirect
- `GET /api/auth/google/callback` — exchange code, получение профиля
- Хранение state в Redis (TTL 10 мин)
- Интерфейс `oauth.Provider` — сразу абстрагируем, чтобы VK/Yandex добавлялись легко

### День 4: JWT сервис

- Access token (15 мин) + Refresh token (30 дней)
- Подпись HS256, секрет из env
- Функции `Issue(userID, kind, ttl)` и `Verify(token, kind)`
- Unit-тесты на JWT

### День 5: Репозиторий пользователей

- `FindOrCreateByOAuth(provider, profile)` с логикой слияния по email
- Транзакция: создание `users` + `user_identities` атомарно
- Unit-тесты с testcontainers-go

### День 6–7: Cookies

- Установка httpOnly cookies в ответе: access + refresh
- Атрибуты: `HttpOnly; Secure; SameSite=None; Partitioned`
- Разные `Path` для access (/) и refresh (/api/auth/refresh)
- Middleware `AuthRequired` — читает access_token из cookie, проверяет JWT, кладёт user_id в context
- Эндпоинт `GET /api/user/me` — возвращает профиль залогиненного
- Эндпоинт `POST /api/auth/refresh` — обновление пары токенов
- Эндпоинт `POST /api/auth/logout` — стирание cookies + blacklist refresh в Redis

**Артефакт недели:** через Postman (или curl с `-c cookies.txt`) проходит полный цикл: открыть `/auth/google/login` в браузере → редирект на Google → callback → cookies выставлены → `/api/user/me` возвращает профиль → `/auth/logout` стирает.

---

## Неделя 3 — Frontend фундамент

### День 1: Инициализация

```bash
npm create vite@latest frontend -- --template react-ts
cd frontend
npx shadcn@latest init
```

- Зависимости: `react-router-dom`, `@tanstack/react-query`, `lucide-react`
- Установка базовых shadcn компонентов: button, card, input, label, form, sonner

### День 2–3: Роутинг + layout

- Роуты: `/`, `/login`, `/create`, `/history`, `/results/:id`, `/auth/success`
- `AppLayout` с Header (логотип + аватарка + меню)
- `AuthGuard` — HOC или route wrapper для защищённых страниц
- Landing page (простой — заголовок + "Войти")

### День 4: API клиент

- `src/lib/api.ts` — обёртка над fetch с `credentials: 'include'`
- Обработка 401 → автоматический refresh → retry оригинального запроса
- Типизированные хуки: `useCurrentUser()`, `useLogin()`, `useLogout()`
- React Query для кэширования + инвалидация при logout

### День 5: LoginPage

- Кнопки "Войти через Google/VK/Yandex" (пока только Google работает)
- Клик → `window.location.href = '${API}/api/auth/google/login'`
- После callback бэк редиректит на `/auth/success`
- `/auth/success` — мигающая загрузка + редирект на `/`

### День 6–7: Профиль + логаут

- Header показывает аватарку и имя (из `/api/user/me`)
- Dropdown menu: История, Логаут
- `useCurrentUser` — загружается при старте приложения
- Если запрос `/me` возвращает 401 — редирект на `/login`
- Настройка Vite proxy для dev: `/api` → `https://localhost:8443`
- HTTPS локально через mkcert

**Артефакт недели:** открываешь `https://localhost:5173`, кликаешь "Войти через Google", авторизуешься, попадаешь на главную, видишь свою аватарку в хедере, жмёшь "Выйти" — возвращаешься на `/login`.

---

## Неделя 4 — VK + Yandex OAuth + CSRF

### День 1–2: VK ID

- Регистрация в vk.com/dev
- Реализация `VKProvider` по интерфейсу
- Особенность: email приходит в response `/access_token`, не в `users.get`
- Тестирование полного flow

### День 3–4: Yandex ID

- Регистрация в oauth.yandex.ru
- Реализация `YandexProvider`
- Scope: `login:email login:info login:avatar`
- PKCE

### День 5: CSRF защита

- Генерация CSRF токена при логине, отдача как не-HttpOnly cookie
- Middleware `CSRFRequired` для POST/PUT/DELETE
- Header `X-CSRF-Token` во фронтенд API клиенте
- Тестирование: без токена получаем 403

### День 6–7: Слияние аккаунтов

- UI в профиле: "Привязанные способы входа"
- Показывает список `user_identities` с иконками
- Кнопка "Привязать другой способ входа" — открывает auth flow с параметром `?link=true`
- Бэк при `link=true`: не создаёт нового user, а добавляет identity к текущему
- Кнопка "Отвязать" (если identities > 1)

**Артефакт недели:** пользователь может залогиниться любым из 3 провайдеров, в профиле видит какие способы привязаны, может добавить ещё один или отвязать.

---

## Неделя 5 — БД генераций + S3

### День 1–2: Модель данных

Миграция:
```sql
CREATE TABLE generation_requests (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         BIGINT REFERENCES users(id),
    status          VARCHAR(30) DEFAULT 'pending',
    recipient_name  VARCHAR(200),
    occasion        VARCHAR(100),
    image_prompt    TEXT,
    song_lyrics     TEXT,
    song_style      VARCHAR(200),
    input_photos    TEXT[],
    input_audio_key VARCHAR(500),
    result_images   TEXT[],
    result_audio_key VARCHAR(500),
    error_message   TEXT,
    credits_spent   INT DEFAULT 1,
    created_at      TIMESTAMPTZ DEFAULT NOW(),
    completed_at    TIMESTAMPTZ
);
```

- Добавить `free_credits INT DEFAULT 3` в `users` если ещё не добавлено
- Репозиторий генераций: Create, GetByID, ListByUser, UpdateStatus

### День 3: Кредиты

- Сервис `CreditsService`:
  - `Charge(userID, amount)` — списание, атомарно через `UPDATE ... WHERE credits >= amount`
  - `Refund(userID, amount)` — возврат при failed генерации
  - `GetBalance(userID)`
- При регистрации нового user — `free_credits = 3`

### День 4–5: Cloudflare R2

- Создание бакета на Cloudflare R2
- Получение Access Key + Secret Key
- Подключение через `github.com/aws/aws-sdk-go-v2` (S3-compatible)
- Сервис `Storage`:
  - `Upload(ctx, key, data, contentType)` → возвращает ключ
  - `GetSignedURL(ctx, key, ttl)` → pre-signed URL на скачивание
  - `Delete(ctx, key)`
- Структура ключей: `uploads/{user_id}/{uuid}.jpg`, `results/{request_id}/image_0.png`

### День 6–7: Загрузка файлов

- Эндпоинт `POST /api/uploads` (multipart/form-data)
- Принимает 1–3 файла, валидация:
  - Картинки: jpg/png, до 10 MB каждая
  - Аудио: mp3/wav/m4a, до 25 MB, длительность до 5 мин (через ffprobe)
- Возвращает массив ключей
- Эндпоинт `GET /api/files/:key` — редирект на signed URL
- Проверка прав: пользователь может получить только свои файлы

**Артефакт недели:** через Postman загружаешь фото, получаешь key, потом `/api/files/:key` — скачиваешь обратно.

---

## Неделя 6 — Nano Banana интеграция

### День 1: Регистрация + тестовый запрос

- Получение Gemini API ключа в Google AI Studio
- Первый запрос через curl: text-only, проверка что работает
- Параметры free tier: 500 RPD для gemini-2.5-flash-image

### День 2–3: Go клиент

```go
type NanoBananaClient struct {
    apiKey string
    model  string  // "gemini-2.5-flash-image"
    http   *http.Client
}

func (c *NanoBananaClient) GenerateImage(
    ctx context.Context,
    prompt string,
    refImages []ImageRef,
) ([]byte, error)
```

- Парсинг ответа: поиск `inline_data` в `candidates[0].content.parts`
- Декодирование base64 → []byte
- Retry с экспоненциальным backoff (429 → подождать → повторить)
- Timeout 60 секунд

### День 4–5: Шаблоны промптов

- Каталог промптов для разных поводов:
  - Birthday
  - New Year
  - Roast (шутливый)
  - Custom
- Функция `buildPrompt(occasion, userDesc, recipientName)` → финальный промпт
- Тестирование на 10+ фотографиях: насколько сохраняется лицо, качество текста

### День 6–7: Безопасность контента

- Обработка отказов safety filter (response без inline_data)
- Fallback: упростить промпт и повторить (убрать слово "face", изменить формулировку)
- Если 3 раза подряд отказ → ошибка пользователю с понятным сообщением
- Логирование отказов для анализа

**Артефакт недели:** есть CLI-скрипт `go run ./cmd/test_image -photo=me.jpg -prompt="космонавт"` который выдаёт готовую картинку в файл. Качество проверено на 10+ примерах.

---

## Неделя 7 — Suno через gcui-art/suno-api

### День 1: Покупка Suno Pro + настройка suno-api

- Регистрация на suno.com, оформление Pro подписки ($10/мес)
- Клонирование `gcui-art/suno-api`
- Добавление сервиса в `docker-compose.yml`
- Получение cookie из браузера → `SUNO_COOKIE`
- Регистрация на 2Captcha + пополнение + `TWOCAPTCHA_KEY`
- Проверка: `curl http://localhost:3100/api/get_limit` возвращает остаток кредитов

### День 2–3: Go клиент для suno-api

```go
type SunoClient struct {
    baseURL string
    http    *http.Client
}

func (c *SunoClient) CustomGenerate(ctx context.Context, req CustomGenerateRequest) ([]Track, error)
func (c *SunoClient) GetTracks(ctx context.Context, ids []string) ([]Track, error)
func (c *SunoClient) GetLimit(ctx context.Context) (*QuotaInfo, error)
```

- Polling статуса с таймаутом 5 мин
- Статусы: submitted → queued → streaming → complete
- При ошибке "copyright" — возврат специальной ошибки для каскадного fallback

### День 4: FFmpeg предобработка

- Проверка установки FFmpeg (в Dockerfile добавить `RUN apt-get install ffmpeg`)
- Функции:
  - `Validate(path)` через ffprobe
  - `NormalizeToMP3(in, out)`
  - `StretchAudio(in, out, factor)`
  - `AddNoise(in, out)`
- Unit-тесты на реальных файлах

### День 5: Каскадный fallback

- Pipeline: оригинал → stretch → noise+pitch
- Функция возвращает путь к успешно загруженному файлу + уровень обработки
- Сохранение уровня в БД (для аналитики)

### День 6–7: Мониторинг cookie

- Cron-job (каждый час): `GetLimit()`
- Если возвращает 401/403 → отправка уведомления (email, Telegram бот админа — да, даже в веб-MVP удобно иметь бота для себя)
- Логирование `credits_left` — чтобы видеть тренд расхода

**Артефакт недели:** CLI-скрипт `go run ./cmd/test_song -audio=track.mp3 -lyrics="С днём рождения, Маша!"` — через 2-3 минуты выдаёт MP3 файл.

---

## Неделя 8 — Worker + API создания генерации

### День 1–2: Очередь Redis

- Структура задачи: `GenerationTask` в JSON
- Push в Redis list: `LPUSH gen_queue`
- Worker: `BRPOP` с таймаутом
- Параллельно N воркеров (по умолчанию 2)

### День 3–4: Worker pipeline

- `ProcessGeneration(task)`:
  - Обновить status → processing_image
  - errgroup с двумя горутинами
  - Горутина A: генерация 3 картинок последовательно (free tier rate limit 10 RPM)
  - Горутина B: предобработка аудио + Suno генерация + polling
  - Wait → обе закончили
  - Сохранение результатов в БД + R2
  - Обновить status → completed

### День 5: API создания

- `POST /api/generations` (multipart):
  - Валидация прав (авторизован, кредиты > 0)
  - Списание 1 кредита
  - Загрузка файлов в R2
  - Создание записи в БД со статусом pending
  - Push задачи в очередь
  - Возврат `{ id, status }`

### День 6: Эндпоинты чтения

- `GET /api/generations` — пагинация, сортировка по created_at DESC
- `GET /api/generations/:id` — детали + signed URLs для результатов
- `GET /api/generations/:id/status` — только статус (для polling с фронта)

### День 7: Обработка ошибок

- Failed → автоматический refund кредита
- Retry на транзиентных ошибках (timeout, 429)
- Максимум 3 retry на одну задачу, потом — failed навсегда
- Хранение error_message в БД для дебага

**Артефакт недели:** через Postman делаешь POST с фото+аудио, получаешь id, через 2-3 минуты в GET видишь статус completed и ссылки на результаты.

---

## Неделя 9 — Frontend: форма создания

### День 1–2: Stepper компонент

- `CreationStepper` — 4 шага на shadcn компонентах
- React Hook Form без Zod — встроенная валидация через `register("field", { required, minLength })`
- Навигация: кнопки "Назад" / "Далее", валидация перед переходом
- Progress bar сверху

### День 3: Step 1 — Получатель

- Поле "Имя получателя"
- Select "Повод": день рождения, новый год, 8 марта, шутка, другое
- Если "другое" → textarea для описания

### День 4: Step 2 — Фото

- Drag-and-drop компонент
- Ограничения: 1–3 файла, JPG/PNG, до 10 MB
- Preview загруженных с кнопкой "Удалить"
- Textarea "Описание желаемого образа"

### День 5: Step 3 — Музыка

- Загрузка одного аудиофайла (MP3/WAV/M4A)
- Player для прослушивания загруженного
- Textarea "Текст песни"
- Input "Стиль" (опционально): "весёлый поп", "рок-н-ролл"

### День 6: Step 4 — Проверка

- Показ всей введённой информации
- Большая кнопка "Создать поздравление"
- Отображение стоимости: "Будет списан 1 кредит"
- Если кредитов нет — кнопка disabled + текст

### День 7: Submit logic

- Сбор всех данных в FormData
- POST `/api/generations`
- При успехе → редирект на `/results/:id`
- При ошибке → toast через sonner

**Артефакт недели:** форма полностью работает, можно пройти все 4 шага и отправить запрос на бэк.

---

## Неделя 10 — Frontend: статус и результаты

### День 1–2: Страница результата

- `ResultPage` (`/results/:id`)
- Polling статуса каждые 3 секунды через React Query
- Когда `status === "completed"` — останавливаем polling
- Состояния: pending, processing_image, processing_audio, completed, failed

### День 3: Анимация статуса

- Компонент `GenerationProgress`:
  - 3 этапа с иконками (Image, Music, CheckCircle)
  - Текущий этап — анимированный Spinner
  - Пройденные — зелёная галочка
  - Плавные переходы через CSS transitions

### День 4: Галерея картинок

- При completed — показ 3 картинок
- Grid или Carousel
- Клик на картинку → модалка (Dialog shadcn) с полным размером
- Кнопка "Скачать" под каждой

### День 5: Аудиоплеер

- `<audio controls>` с source из signed URL
- Рядом: имя файла + длительность
- Кнопка "Скачать MP3"

### День 6: Share функционал

- Кнопки: "Скопировать ссылку", "Поделиться"
- Для публичного доступа — отдельный токен на генерацию (optional для MVP)
- В MVP: только скачивание, share потом

### День 7: Страница истории

- `/history` — список генераций пользователя
- Карточка каждой: миниатюра первой картинки, имя получателя, дата, статус
- Клик → переход на `/results/:id`
- Пагинация (20 на странице)

**Артефакт недели:** полный user flow с фронта: вход → создание → просмотр прогресса → результат → история.

---

## Неделя 11 — Полировка и устойчивость

### День 1: Rate limiting

- Middleware на базе Redis:
  - Global: 100 req/min на IP
  - Auth: 5 attempts/min на IP
  - Generations: 10 per day на user
- Ответ 429 с Retry-After header

### День 2: Валидация и безопасность

- Валидация всех входных данных на бэке (не только на фронте!)
- Санитизация текстов (без HTML/JS инъекций)
- Ограничение на размер upload (тоже на фронте и на бэке)
- Проверка magic bytes файлов — не только расширение

### День 3: Ошибки в UX

- Общий ErrorBoundary в React
- Понятные сообщения для каждого типа ошибки:
  - "Закончились кредиты"
  - "Не удалось обработать фото — попробуй другое"
  - "Сервис временно недоступен"
- Retry-кнопки где уместно

### День 4: Skeleton states

- `Skeleton` из shadcn для всех загрузок
- Не пустой экран, а плейсхолдеры — лучше воспринимаются пользователем
- Loading states в формах (кнопки с Spinner)

### День 5: Sentry / error tracking

- Подключение Sentry (free tier: 5k events/month)
- На фронте и бэке
- Фильтрация: не отправлять ожидаемые ошибки (валидации, 401)

### День 6: Terms of Service + Privacy Policy

- Страницы `/terms`, `/privacy`
- Чекбокс согласия при первой регистрации
- Шаблоны через termsfeed.com или аналог — потом доработать юридически

### День 7: SEO базовый

- Meta tags на главной
- robots.txt
- sitemap.xml (статический пока)
- Favicon + OpenGraph картинки

**Артефакт недели:** приложение устойчиво к типичным проблемам, выглядит прилично даже когда что-то идёт не так.

---

## Неделя 12 — Деплой и запуск

### День 1–2: Подготовка production

- Покупка VPS (Hetzner CPX41, ~$30/мес)
- Настройка SSH ключей, firewall (ufw)
- Установка Docker + Docker Compose
- Domain на Cloudflare → указать на IP VPS
- SSL через Cloudflare (free)

### День 3: CI/CD

- GitHub Actions:
  - На push в main — build Docker images
  - Push в Container Registry (GitHub Packages)
  - SSH на VPS, `docker-compose pull && up -d`
- Secrets в GitHub: SSH_KEY, VPS_HOST

### День 4: Production env

- Отдельные creds: Google OAuth, Gemini API, Suno cookie (prod аккаунт)
- `docker-compose.prod.yml` с рестриктивными настройками
- Volume для postgres data + backup скрипт (pg_dump → R2 ежедневно)

### День 5: Мониторинг

- Uptime мониторинг: UptimeRobot (free) — проверка `/api/health` каждые 5 мин
- Логи: `docker-compose logs -f` на первое время; потом можно добавить Loki
- Алерты в Telegram при падении сервисов

### День 6: Smoke test на проде

- Полный прогон: регистрация → создание → получение результата
- Проверка на разных браузерах: Chrome, Safari, Firefox
- Проверка на мобильных (responsive)
- Проверка всех 3 провайдеров OAuth

### День 7: Soft launch

- Landing page → "В бета-тесте"
- Приглашение 10–20 друзей/знакомых
- Сбор feedback
- Документирование найденных багов в issues

**Артефакт финала:** работающий сервис на `https://fungreet.app`, первые 20 реальных пользователей, обратная связь собрана.

---

## Риски и буферы

### Что может затянуть сроки

| Проблема | Вероятность | Доп. время | Как избежать |
|----------|------------|-----------|--------------|
| Suno cookie постоянно отваливается | Средняя | +3 дня | Купить 2 аккаунта как backup |
| Safety filter Nano Banana режет промпты | Высокая | +2 дня | Тестировать промпты заранее, собрать каталог |
| OAuth у одного провайдера не работает | Средняя | +2 дня | Начать с Google — у них лучшая документация |
| iOS Safari странно себя ведёт с cookies | Высокая | +3 дня | Тестировать на реальном iPhone с недели 3 |
| 2–3 часа в день недостаточно | Высокая | +4 недели | Считать 12 недель минимумом, реалистично 14–16 |

**Реалистичный срок: 3–4 месяца** до публичной беты.

### Что можно отложить

Если к неделе 8 ясно, что отстаёшь — режь scope в таком порядке:
1. Сначала: история генераций (неделя 10, день 7) — не критично
2. Потом: слияние аккаунтов (неделя 4, дни 6–7) — один провайдер хватит
3. Потом: VK и Yandex — можно оставить только Google для первого релиза
4. В последнюю очередь: сами генерации — без них продукта нет

---

## Зависимости и готовность среды

**Можно начать делать прямо сейчас:**
- Go проект
- Postgres, Redis в Docker
- React + shadcn фронт

**Нужно зарегистрировать заранее (не блокирует, но занимает время):**
- Google Cloud Console (мгновенно)
- VK Developer (1–2 дня на модерацию)
- Yandex OAuth (1–2 дня)
- Suno Pro подписка + получение cookie (мгновенно)
- 2Captcha аккаунт + пополнение $10 (мгновенно)
- Gemini API key (мгновенно)
- Cloudflare R2 бакет (мгновенно)

**Рекомендация:** всё это оформи в первые выходные, параллельно с неделей 1.

---

## Что дальше (после MVP)

Порядок следующих итераций:
1. **Итерация 2 (2 недели):** Telegram Mini App + Telegram OAuth
2. **Итерация 3 (2 недели):** Оплата через Stripe + Telegram Stars
3. **Итерация 4 (1 неделя):** Реферальная программа
4. **Итерация 5 (2 недели):** Улучшение качества генераций, новые шаблоны промптов
5. **Итерация 6:** рост и маркетинг
