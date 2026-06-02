# FunBaza - текущее состояние разработки

> Обновлено: 2026-06-02  
> Цель релиза: публичный web MVP до подключения оплаты  
> Продакшен URL: `https://funbaza.ru/`

## Краткий итог

FunBaza выведен в продакшен как web MVP для генерации AI-изображений и музыки. Основной пользовательский сценарий работает end-to-end: вход через Google, баланс кредитов, история сессий, генерация изображений, генерация музыки, генерация текста, загрузка файлов, фоновая обработка и выдача готовых ассетов.

Оплата намеренно не входит в этот релиз. Пользователи получают кредиты через текущую логику начислений; платное пополнение - следующий крупный этап после продакшен-деплоя.

## Статус компонентов

| Компонент | Статус | Примечание |
| --- | --- | --- |
| Backend | Готов | Go/Gin API, repositories, services, workers, migrations |
| Frontend | Готов | React/Vite SPA, в продакшене раздается Go-контейнером |
| PostgreSQL | Готов | Миграции применяются автоматически при старте сервера |
| Redis | Готов | Очередь задач и хранение webhook state |
| Object storage | Готов к prod | Продакшен compose использует S3-compatible MinIO |
| Google OAuth | Готов | Login, callback, refresh и logout через httpOnly cookies |
| Cookie auth | Базово готов к релизу | В продакшене cookie ставятся как `Secure`, `HttpOnly`, `SameSite=Lax` |
| Генерация изображений | Готова | Интеграция с `kie.ai`, mock fallback при пустом ключе |
| Генерация музыки | Готова | Интеграция с `kie.ai`, mock fallback при пустом ключе |
| Генерация текста | Готова | Три варианта, выбор и ручное редактирование в UI |
| Кредиты и биллинг | Готово для MVP | Initial/daily grants, charge, refund, transaction history |
| Оплата | Не реализована | Покупка кредитов - следующий этап |
| Swagger | Готов | Доступен по `/swagger/index.html` |
| Production Docker | Работает | HTTPS stack поднят через `docker-compose.prod.ssl.yml` |
| Мониторинг | Работает | Grafana, Prometheus, exporters, cAdvisor |
| БД admin | Работает | pgAdmin доступен через отдельный поддомен |

## Пользовательские изменения с прошлого статуса

- Сценарий создания начинается с выбора режима: изображение или музыка.
- Composer поддерживает image presets, выбор модели, загрузку фото, стиль музыки, генерацию текста и редактирование текста.
- В thread добавлены более выразительные loading states для изображений и аудио.
- Сгенерированное аудио использует кастомный тематический плеер с play/pause, seek, duration, download и анимированной волной.
- Тексты музыкального UI сделаны нейтральными, чтобы продукт не ограничивался только поздравительными сценариями.
- Продакшен-cookie теперь ставятся с `Secure` вне development-режима.

## Актуальная архитектура

```text
Browser
  |
  | HTTPS
  v
Nginx
  |
  +--> Go app :8080
  |      |
  |      +--> PostgreSQL
  |      +--> Redis queue
  |      +--> MinIO / S3-compatible storage
  |      +--> kie.ai image and music APIs
  |
  +--> MinIO public storage endpoint
  +--> Grafana / pgAdmin
```

В продакшене фронтенд собирается root `Dockerfile` и копируется в `backend/web/dist`. Go server раздает SPA и API из одного app-контейнера.

## Важные переменные окружения

```env
APP_ENV=production
BASE_URL=https://funbaza.ru
POSTGRES_PASSWORD=...
JWT_SECRET=...
KIE_API_KEY=...

GOOGLE_CLIENT_ID=...
GOOGLE_CLIENT_SECRET=...
GOOGLE_REDIRECT_URI=https://funbaza.ru/api/auth/google/callback

S3_ENDPOINT=http://minio:9000
S3_PUBLIC_ENDPOINT=https://storage.funbaza.ru
S3_REGION=us-east-1
S3_ACCESS_KEY=...
S3_SECRET_KEY=...
S3_BUCKET=funbaza
S3_USE_PATH_STYLE=true
WORKER_COUNT=4
```

## Статус проверок

Последние проверки:

- `https://funbaza.ru/` - `200`, проверено 2026-06-02.
- `https://funbaza.ru/api/health` - `200`, проверено 2026-06-02.
- `https://grafana.funbaza.ru/login` - `200`, проверено 2026-06-02.
- `https://pgadmin.funbaza.ru/login` - `200`, проверено 2026-06-02.
- Google OAuth login - проходит, проверено 2026-06-02.
- `/api/v1/user/me`, balance, tariff и sessions - `200` для авторизованного пользователя, проверено 2026-06-02.
- Генерация - в prod logs есть успешный `generation completed`, проверено 2026-06-02.
- Signed URLs для изображений и аудио через `storage.funbaza.ru` - работают, проверено 2026-06-02.

Локальные проверки перед следующим релизным коммитом:

- `cd frontend && npm run lint`
- `cd frontend && npm run build`
- `cd backend && go test ./...`
- `docker compose --env-file .env.prod.example -f docker-compose.prod.yml config --quiet`
- `docker compose --env-file .env.prod.example -f docker-compose.prod.ssl.yml config --quiet`

## Известные ограничения релиза

1. Нет платежного шлюза и покупки кредитов.
2. Нет админки для ручной корректировки балансов, пользователей и неуспешных генераций.
3. Продакшен-хранилище по умолчанию использует bundled MinIO; переход на Cloudflare R2 или другой внешний S3-провайдер остается опциональным.
4. CI/CD pipeline не закоммичен; деплой выполняется вручную через Docker Compose.
5. Frontend проверяется build/lint, но отдельного component test suite пока нет.
6. Grafana и pgAdmin доступны публично через поддомены; перед широким трафиком желательно ограничить доступ по IP или дополнительной авторизацией.

## Решение по релизу

Проект подходит для контролируемого публичного MVP-релиза. Продакшен-домены, DNS, SSL, OAuth, Docker stack, мониторинг, pgAdmin и основной пользовательский сценарий проверены на `https://funbaza.ru`.
