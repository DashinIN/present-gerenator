# Project Overview

FunGreet is a monorepo for AI-generated greeting cards/songs.

- Backend: Go API and worker in `backend/`, entrypoint `backend/cmd/server/main.go`.
- Frontend: React/Vite app in `frontend/` with chat-oriented UI.
- Persistence: PostgreSQL migrations in `backend/migrations/`; Redis queue for generation tasks.
- Storage: local upload/results storage by default under `backend/data/uploads`; R2 env placeholders exist but implementation currently only supports local in server startup.
- Docs: product/technical docs in `docs/`; some docs are aspirational or stale relative to current routes and implemented integrations.
