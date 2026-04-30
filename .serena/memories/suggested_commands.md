# Suggested Commands

Windows/PowerShell from repo root:

- `npm run infra` starts local Postgres and Redis via `backend/docker-compose.yml`.
- `npm run infra:stop` stops local infrastructure.
- `npm run back` runs the Go API from `backend/cmd/server`.
- `npm run front` runs the Vite dev server.
- `npm run dev` starts infra plus backend/frontend concurrently.
- `npm run build` runs `cd frontend && npm run build`.
- `cd frontend && npm run lint` runs ESLint.
- `cd backend && go test ./...` runs Go tests when present.
- `cd backend && make migrate-up` applies SQL migrations using `DATABASE_URL` from `backend/.env`.
- Prefer `rg` / `rg --files` for code search.
