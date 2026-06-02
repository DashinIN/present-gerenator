# FunBaza production status

> Updated: 2026-06-02

## Summary

FunBaza is deployed as a public HTTPS MVP on `funbaza.ru`.

The production stack is running on one Ubuntu server with Docker Compose. Nginx terminates TLS and routes traffic to the Go app, public MinIO storage, Grafana, and pgAdmin. PostgreSQL, Redis, MinIO, Prometheus, exporters, Grafana, pgAdmin, and the app container are up.

## URLs

| Service | URL | Status |
| --- | --- | --- |
| App and API | `https://funbaza.ru` | Live |
| Health check | `https://funbaza.ru/api/health` | `200` |
| Public storage | `https://storage.funbaza.ru` | Signed URLs work; root returns `403` |
| Grafana | `https://grafana.funbaza.ru` | Login page live |
| pgAdmin | `https://pgadmin.funbaza.ru` | Login page live |

## Server

```text
IP: 83.166.237.247
SSH user: ubuntu
Project path: /opt/funbaza
Env file: /opt/funbaza/.env.prod.local
HTTPS compose file: docker-compose.prod.ssl.yml
```

Use `sudo docker compose` on the server because the `ubuntu` user may not have direct Docker socket access.

## Verified Behavior

Verified on 2026-06-02:

- `https://funbaza.ru/` returns `200`.
- `https://funbaza.ru/api/health` returns `200`.
- `https://grafana.funbaza.ru/login` returns `200`.
- `https://pgadmin.funbaza.ru/login` returns `200`.
- Google OAuth login completes and `/api/v1/user/me` returns `200` for an authenticated user.
- Billing balance, tariff, and session endpoints return `200` for an authenticated user.
- At least one generation completed successfully in production logs.
- Generated image and audio assets are served through `storage.funbaza.ru` signed URLs.

## Operational Commands

Check containers:

```bash
cd /opt/funbaza
sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml ps
```

Read app logs:

```bash
cd /opt/funbaza
sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml logs --tail=120 app
```

Read nginx logs:

```bash
cd /opt/funbaza
sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml logs --tail=120 nginx
```

Restart production stack:

```bash
cd /opt/funbaza
sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml up -d --build
```

## Admin Access

Grafana credentials are stored in `/opt/funbaza/.env.prod.local`:

```env
GRAFANA_ADMIN_USER=...
GRAFANA_ADMIN_PASSWORD=...
```

pgAdmin credentials are stored in `/opt/funbaza/.env.prod.local`:

```env
PGADMIN_DEFAULT_EMAIL=...
PGADMIN_DEFAULT_PASSWORD=...
```

Add the production database in pgAdmin with:

```text
Host: postgres
Port: 5432
Database: funbaza
Username: funbaza
Password: POSTGRES_PASSWORD from /opt/funbaza/.env.prod.local
```

## Remaining Product Limits

- Payment and paid credit purchase are not implemented.
- There is no custom admin UI for users, balances, or failed generations.
- Deployment is manual through Docker Compose; CI/CD is not configured.
- MinIO is bundled on the VPS. External object storage remains an optional hardening step.
- Backups and access restrictions for Grafana/pgAdmin should be configured before broader public traffic.
