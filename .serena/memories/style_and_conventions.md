# Style and Conventions

- Go: package by layer/domain under `backend/internal`; use `gofmt`; handlers/services/repositories are separated; HTTP errors use `{ error: { code, message } }` shape.
- React: TypeScript, PascalCase components, camelCase hooks, shared helpers in `frontend/src/lib`, reusable UI in `frontend/src/components/ui`.
- Frontend state: TanStack Query for server state; local `useState`/context for UI state such as theme and active session.
- Styling: mostly inline styles plus CSS variables and Tailwind v4 import; lucide-react icons are used.
- Config: `.env.example` templates; never commit real secrets.
