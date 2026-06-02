# FunBaza production deploy

Current production status: deployed and verified on 2026-06-02.

Production server:

- Public IP: `83.166.237.247`
- Project path: `/opt/funbaza`
- Compose stack: `docker-compose.prod.ssl.yml`
- Env file: `/opt/funbaza/.env.prod.local`

Target domains:

- `https://funbaza.ru` and `https://www.funbaza.ru` - app and API.
- `https://storage.funbaza.ru` - public generated files through MinIO.
- `https://grafana.funbaza.ru` - monitoring and analytics dashboards.
- `https://pgadmin.funbaza.ru` - PostgreSQL admin UI.

## Server baseline

Recommended single-server start:

- Russia-based VPS/VDS or dedicated server, Moscow or Saint Petersburg.
- 4 vCPU, 8 GB RAM, 80-120 GB NVMe minimum for MVP.
- Ubuntu 24.04 LTS.
- Docker Engine and Docker Compose plugin.
- Open ports: `80/tcp`, `443/tcp`, `22/tcp`.
- Keep PostgreSQL, Redis, MinIO, Prometheus, Grafana and pgAdmin unexposed to the public internet except through nginx.

For Russian availability and payments, start with Selectel, Timeweb Cloud, Yandex Cloud, Beget Cloud or a comparable provider with Russian data centers and DDoS protection. Selectel explicitly offers Moscow cloud servers and DDoS-protected VPS/VDS. Timeweb Cloud is also a common Russian VPS/cloud option.

## DNS

Create `A` records pointing to the server public IPv4:

```text
funbaza.ru
www.funbaza.ru
storage.funbaza.ru
grafana.funbaza.ru
pgadmin.funbaza.ru
```

Wait until all records resolve to the VPS before issuing certificates.

For the current deployment all five domains resolve to `83.166.237.247`.

## Production env

Create `.env.prod.local` on the server from `.env.prod.example`.

Required values:

```env
APP_ENV=production
BASE_URL=https://funbaza.ru
S3_PUBLIC_ENDPOINT=https://storage.funbaza.ru
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
GOOGLE_REDIRECT_URI=https://funbaza.ru/api/auth/google/callback

PGADMIN_DEFAULT_EMAIL=admin@funbaza.ru
PGADMIN_DEFAULT_PASSWORD=replace-with-strong-pgadmin-password
GRAFANA_ADMIN_USER=admin
GRAFANA_ADMIN_PASSWORD=replace-with-strong-grafana-password
PROMETHEUS_RETENTION=30d

LETSENCRYPT_EMAIL=admin@funbaza.ru
```

Google OAuth must allow:

- Authorized JavaScript origin: `https://funbaza.ru`
- Redirect URI: `https://funbaza.ru/api/auth/google/callback`

## First HTTP start

Start HTTP first so Certbot can pass ACME webroot challenges:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml up -d --build
docker compose --env-file .env.prod.local -f docker-compose.prod.yml ps
docker compose --env-file .env.prod.local -f docker-compose.prod.yml logs --tail=120 app
```

Check:

```bash
curl http://funbaza.ru/api/health
```

## Issue TLS certificate

Issue one SAN certificate under `funbaza.ru`:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml run --rm certbot certonly \
  --webroot \
  --webroot-path /var/www/certbot \
  --email "$LETSENCRYPT_EMAIL" \
  --agree-tos \
  --no-eff-email \
  -d funbaza.ru \
  -d www.funbaza.ru \
  -d storage.funbaza.ru \
  -d grafana.funbaza.ru \
  -d pgadmin.funbaza.ru
```

Then switch to HTTPS:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.yml down
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml up -d --build
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml ps
```

Check:

```bash
curl https://funbaza.ru/api/health
```

Expected result:

```json
{"status":"ok"}
```

## Current production checks

Run from any machine with network access:

```bash
curl -s -o /dev/null -w 'funbaza %{http_code}\n' https://funbaza.ru/
curl -s -o /dev/null -w 'health %{http_code}\n' https://funbaza.ru/api/health
curl -s -o /dev/null -w 'grafana %{http_code}\n' https://grafana.funbaza.ru/login
curl -s -o /dev/null -w 'pgadmin %{http_code}\n' https://pgadmin.funbaza.ru/login
curl -s -o /dev/null -w 'storage-root %{http_code}\n' https://storage.funbaza.ru/
```

Verified on 2026-06-02:

```text
funbaza 200
health 200
grafana 200
pgadmin 200
storage-root 403
```

`storage-root 403` is expected: MinIO should not list bucket contents. Generated assets are served through signed URLs created by the app.

Check the stack on the server:

```bash
cd /opt/funbaza
sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml ps
sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml logs --tail=120 app
```

## Monitoring and analytics

Grafana opens at `https://grafana.funbaza.ru`.

Credentials come from `/opt/funbaza/.env.prod.local`:

```env
GRAFANA_ADMIN_USER=...
GRAFANA_ADMIN_PASSWORD=...
```

Provisioned dashboards:

- RPS from nginx exporter.
- Host CPU and memory from node exporter.
- Container metrics from cAdvisor.
- PostgreSQL connections from postgres exporter.
- New users by hour.
- Generations by status.
- Credits spent by hour.
- Top users by credits and generation count.
- Top generation prompt/topic signals.
- Image model distribution.
- Basic language signal from prompt text.

Important limitation: the app currently stores `credits_spent`, not provider token counts. The dashboard treats credits as internal token/usage spend. If exact KIE/OpenAI token usage is required, add dedicated columns such as `provider`, `provider_task_id`, `input_tokens`, `output_tokens`, `provider_cost_usd`, `topic`, and `language` to `generation_requests` or a separate immutable `generation_usage_events` table.

## pgAdmin

pgAdmin opens at `https://pgadmin.funbaza.ru`.

Credentials come from `/opt/funbaza/.env.prod.local`:

```env
PGADMIN_DEFAULT_EMAIL=...
PGADMIN_DEFAULT_PASSWORD=...
```

Add server:

- Host: `postgres`
- Port: `5432`
- Database: `funbaza`
- Username: `funbaza`
- Password: value of `POSTGRES_PASSWORD`

## Admin security

Minimum:

- Strong unique passwords for Grafana and pgAdmin.
- Do not expose PostgreSQL, Redis, MinIO console, Prometheus, node exporter or cAdvisor ports publicly.
- Restrict `grafana.funbaza.ru` and `pgadmin.funbaza.ru` by source IP at VPS firewall or nginx if there is a stable admin IP.
- Enable host firewall after Docker is installed.
- Configure backups before real traffic.

## Renew TLS

Manual renewal command:

```bash
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml run --rm certbot renew
docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml exec nginx nginx -s reload
```

Put it into cron/systemd timer on the server.

## Backups

At minimum, back up:

- `postgres_data`
- `minio_data`
- `.env.prod.local`

Use encrypted off-server backups. For a single-server MVP, start with daily PostgreSQL dumps and MinIO volume snapshots; for real production, move PostgreSQL and object storage to managed services or set up tested restore automation.
