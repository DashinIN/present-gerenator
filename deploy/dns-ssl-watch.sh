#!/usr/bin/env bash
set -u

cd /opt/funbaza || exit 1

domains=(
  funbaza.ru
  www.funbaza.ru
  storage.funbaza.ru
  grafana.funbaza.ru
  pgadmin.funbaza.ru
)

expected_ip="83.166.237.247"

log() {
  printf '[%s] %s\n' "$(date -u '+%Y-%m-%dT%H:%M:%SZ')" "$*"
}

domain_resolves() {
  local domain="$1"
  nslookup "$domain" 1.1.1.1 2>/dev/null | grep -q "Address: ${expected_ip}"
}

all_domains_resolve() {
  local domain
  for domain in "${domains[@]}"; do
    if ! domain_resolves "$domain"; then
      log "waiting: ${domain} does not resolve to ${expected_ip} yet"
      return 1
    fi
  done
  return 0
}

issue_certificate() {
  local email
  email="$(grep '^LETSENCRYPT_EMAIL=' .env.prod.local | cut -d= -f2-)"

  sudo docker compose --env-file .env.prod.local -f docker-compose.prod.yml run --rm certbot certonly \
    --webroot \
    --webroot-path /var/www/certbot \
    --email "$email" \
    --agree-tos \
    --no-eff-email \
    -d funbaza.ru \
    -d www.funbaza.ru \
    -d storage.funbaza.ru \
    -d grafana.funbaza.ru \
    -d pgadmin.funbaza.ru
}

switch_to_https() {
  sudo docker compose --env-file .env.prod.local -f docker-compose.prod.yml down
  sudo docker compose --env-file .env.prod.local -f docker-compose.prod.ssl.yml up -d --build
}

log "funbaza DNS/SSL watcher started"

while true; do
  if all_domains_resolve; then
    log "DNS is ready, issuing certificate"
    if issue_certificate; then
      log "certificate issued, switching to HTTPS"
      switch_to_https
      if curl -fsS https://funbaza.ru/api/health >/dev/null; then
        log "HTTPS health check passed"
      else
        log "HTTPS switch completed, but health check failed"
      fi
      exit 0
    fi
    log "certificate issue failed, retrying in 10 minutes"
  fi

  sleep 600
done
