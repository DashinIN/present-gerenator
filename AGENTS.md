# Repository Guidelines

## Project Structure & Module Organization

This repository contains a Go backend and a React/Vite frontend for FunBaza.

- `backend/` contains the API server, repositories, services, middleware, migrations, and generated Swagger docs.
- `backend/cmd/server/` is the main server entry point; `backend/cmd/sunotest/` is a small integration test utility.
- `backend/internal/` holds application code grouped by responsibility: `handlers`, `services`, `repository`, `worker`, `models`, `config`, and `middleware`.
- `frontend/` contains the Vite React app. Source files live in `frontend/src/`, with reusable UI in `components/`, hooks in `hooks/`, API/types/theme helpers in `lib/`, and pages in `pages/`.
- `docs/` contains product and technical documentation. `examples/` is reserved for sample inputs or outputs.
- Root Docker files and `backend/docker-compose.yml` define local infrastructure.

## Build, Test, and Development Commands

- `npm run dev` starts local infrastructure, backend, and frontend together.
- `npm run infra` starts backend dependencies with Docker Compose.
- `npm run infra:stop` stops local infrastructure.
- `npm run back` runs the Go API from `backend/cmd/server`.
- `npm run front` runs the Vite frontend.
- `npm run build` builds the frontend with TypeScript checks.
- `cd frontend && npm run lint` runs ESLint.
- `cd backend && go test ./...` runs Go tests when present.
- `cd backend && make migrate-up` applies database migrations using `DATABASE_URL` from `backend/.env`.

## Coding Style & Naming Conventions

Use `gofmt` for Go files and keep packages focused by layer. Name Go files by domain, such as `billing.go` or `generation.go`. In React, use TypeScript, PascalCase component files (`ChatThread.tsx`), camelCase hooks (`useAuth.ts`), and shared primitives under `frontend/src/components/ui/`.

## Testing Guidelines

No dedicated test suite is currently committed. Add backend tests as `*_test.go` beside the code under test and run `go test ./...`. For frontend changes, at minimum run `npm run lint` and `npm run build`; add component or hook tests if a frontend test framework is introduced.

## Commit & Pull Request Guidelines

Recent commits use short Conventional Commit-style subjects such as `feat: ...` and `chore: ...`. Keep subjects imperative and scoped to one change. Pull requests should include a concise description, commands run, linked issues when applicable, screenshots for UI changes, and notes for migrations or new environment variables.

## Security & Configuration Tips

Do not commit real secrets. Use `.env.example` as the template, keep local values in `.env` files, and document new configuration keys when adding services or migrations.
