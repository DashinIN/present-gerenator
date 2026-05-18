# Документация FunBaza

В этой папке лежит документация по текущему web MVP FunBaza.

FunBaza сейчас представляет собой приложение на React/Vite и Go/Gin для генерации AI-изображений и музыки. Текущий фокус - продакшен-релиз web MVP. Подключение оплаты намеренно вынесено в следующий этап после деплоя.

## С чего начать

| Документ | Назначение |
| --- | --- |
| [04_FunBaza_DevStatus.md](./04_FunBaza_DevStatus.md) | Актуальный статус реализации и готовность к релизу |
| [05_FunBaza_Release_Deploy.md](./05_FunBaza_Release_Deploy.md) | План продакшен-деплоя и чеклист |
| [PROJECT_AUDIT.md](./PROJECT_AUDIT.md) | Исторические заметки аудита |
| [IMPLEMENTATION_PLAN.md](./IMPLEMENTATION_PLAN.md) | Исторический план реализации |
| [FunBaza_Auth_Cookies.md](./FunBaza_Auth_Cookies.md) | Заметки по авторизации и cookie |
| [01_FunBaza_Business_Product.md](./01_FunBaza_Business_Product.md) | Бизнес- и продуктовая стратегия, шире текущего MVP |
| [02_FunBaza_Technical_Part1.md](./02_FunBaza_Technical_Part1.md) | Ранний технический дизайн, частично целевое состояние |
| [03_FunBaza_Technical_Part2.md](./03_FunBaza_Technical_Part2.md) | Ранние технические заметки, частично целевое состояние |
| [FunBaza_Web_MVP_Plan.md](./FunBaza_Web_MVP_Plan.md) | Изначальный roadmap MVP |

## Текущий объем MVP

Реализовано:

- Вход через Google OAuth с httpOnly cookie.
- Баланс кредитов, история транзакций, стартовые начисления, ежедневные начисления, списание и refund.
- Сессии с историей генераций и переименованием.
- Генерация изображений через `kie.ai`.
- Генерация музыки через `kie.ai`.
- Генерация текста песни в трех вариантах с выбором и ручным редактированием.
- Загрузка файлов-референсов для изображений.
- Фоновая обработка через Redis queue и worker.
- Продакшен Docker-образ, который раздает и API, и фронтенд.
- Продакшен Compose stack с PostgreSQL, Redis, MinIO, app и Nginx.

Пока не реализовано:

- Платежный шлюз и покупка кредитов.
- Админка.
- CI/CD автоматизация.
- Полноценный мониторинг и алертинг.

## Полезные команды

Локальная инфраструктура и приложение:

```bash
npm run infra
npm run back
npm run front
```

Проверки фронтенда:

```bash
cd frontend
npm run lint
npm run build
```

Проверки бэкенда:

```bash
cd backend
go test ./...
```

Продакшен-сборка через Docker Compose:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml build app
```

## Продакшен-точки входа

Текущая продакшен-схема ожидает два публичных hostname:

- `BASE_URL`, например `https://api.yourdomain.com`
- `S3_PUBLIC_ENDPOINT`, например `https://storage.yourdomain.com`

Приложение раздается из того же Go-контейнера, что и API. Storage hostname проксирует выдачу объектов из MinIO.

## Примечания по документации

Файлы `01_*`, `02_*`, `03_*` и `FunBaza_Web_MVP_Plan.md` содержат ранние плановые материалы. Для подготовки релиза источником истины считать `04_FunBaza_DevStatus.md` и `05_FunBaza_Release_Deploy.md`.
