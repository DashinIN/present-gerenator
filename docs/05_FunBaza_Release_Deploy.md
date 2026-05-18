# FunBaza - план продакшен-релиза

> Обновлено: 2026-05-18  
> Цель: задеплоить текущий web MVP до подключения оплаты.

## Краткий состав релиза

В текущий релиз входит рабочий MVP:

- Google OAuth authentication.
- Генерация изображений и музыки.
- Генерация и редактирование текста.
- Учет кредитов без платного пополнения.
- История сессий.
- Кастомный тематический аудиоплеер.
- Продакшен Docker stack с app, PostgreSQL, Redis, MinIO и Nginx.

Оплата в релиз не входит. После стабилизации продакшена следующий этап разработки - платежный провайдер и сценарий покупки кредитов.

## Чеклист перед деплоем

1. Выбрать продакшен-домены.
   Пример: `api.yourdomain.com` для приложения/API и `storage.yourdomain.com` для публичных generated assets.
2. Настроить DNS.
   Оба hostname должны указывать на публичный IP VPS.
3. Обновить Nginx configs.
   Заменить `api.yourdomain.com` и `storage.yourdomain.com` в `deploy/nginx/funbaza.conf` и `deploy/nginx/funbaza.ssl.conf`.
4. Настроить Google OAuth.
   Добавить redirect URI `https://api.yourdomain.com/api/auth/google/callback` и origin `https://api.yourdomain.com`.
5. Создать продакшен-файл окружения.
   Взять `.env.prod.example` за основу и создать `.env.prod.local` на сервере.
6. Запустить финальные проверки локально.

```bash
cd frontend
npm run lint
npm run build

cd ../backend
go test ./...
```

7. Проверить секреты.
   Использовать сильные значения для `POSTGRES_PASSWORD`, `JWT_SECRET`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `KIE_API_KEY` и Google OAuth client secret.

## Подготовка сервера

Установить Docker и Docker Compose plugin на VPS.

Склонировать репозиторий или перенести release archive на сервер, затем создать `.env.prod.local` в корне репозитория.

Пример обязательных значений:

```env
APP_ENV=production
BASE_URL=https://api.yourdomain.com
S3_PUBLIC_ENDPOINT=https://storage.yourdomain.com
S3_REGION=us-east-1
S3_BUCKET=funbaza
S3_USE_PATH_STYLE=true

POSTGRES_PASSWORD=replace-with-strong-password
JWT_SECRET=replace-with-long-random-secret
S3_ACCESS_KEY=replace-with-minio-access-key
S3_SECRET_KEY=replace-with-strong-minio-secret

WORKER_COUNT=4
KIE_API_KEY=replace-with-kie-key

GOOGLE_CLIENT_ID=replace-with-google-client-id
GOOGLE_CLIENT_SECRET=replace-with-google-client-secret
GOOGLE_REDIRECT_URI=https://api.yourdomain.com/api/auth/google/callback

LETSENCRYPT_EMAIL=admin@yourdomain.com
```

## Первый HTTP-запуск

Сначала поднять stack без SSL, чтобы Nginx мог отвечать на ACME challenge.

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml up -d --build
docker compose --env-file .env.prod.local -f docker-compose.prod.yml ps
docker compose --env-file .env.prod.local -f docker-compose.prod.yml logs --tail=120 app
```

Проверить health:

```bash
curl http://api.yourdomain.com/api/health
```

## Выпуск SSL-сертификатов

Оставить HTTP stack запущенным и выпустить сертификаты через общий certbot webroot:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml run --rm certbot certonly --webroot --webroot-path /var/www/certbot --email "$LETSENCRYPT_EMAIL" --agree-tos --no-eff-email -d api.yourdomain.com -d storage.yourdomain.com
```

После этого запустить полный SSL stack:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml down
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml up -d --build
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml ps
```

Проверить health:

```bash
curl https://api.yourdomain.com/api/health
```

## Контрольная проверка после деплоя

Проверить на продакшене:

1. Открыть `https://api.yourdomain.com/`.
2. Войти через Google.
3. Убедиться, что `/api/v1/user/me` успешно отвечает после login.
4. Создать image-only generation.
5. Создать music generation.
6. Убедиться, что generated files открываются с `https://storage.yourdomain.com/...`.
7. Проверить, что balance и transactions изменились корректно.
8. Проверить, что logout очищает сессию.
9. Проверить logs на ошибки.

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml logs --tail=200 app
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml logs --tail=100 nginx
```

## План отката

Если новый релиз ломается до того, как миграции данных становятся отдельным риском:

1. Остановить stack.
2. Вернуть предыдущий git commit или release archive.
3. Пересобрать и запустить stack.
4. Проверить `/api/health`.

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml down
git checkout <previous-release-sha>
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml up -d --build
```

Docker volumes с базой данных сохраняются. Не удалять их при откате.

## Задачи после деплоя

- Добавить автоматическое продление SSL-сертификатов.
- Настроить backups для PostgreSQL и MinIO volumes.
- Добавить uptime monitoring для `/api/health`.
- Добавить log retention или внешний logging.
- Добавить базовый admin path для ручной корректировки балансов.
- Начать планирование оплаты после стабильных продакшен-проверок.
