# Task Completion Checklist

Before finishing code changes:

- Backend changes: run `cd backend && go test ./...` where feasible; ensure Go files are gofmt-formatted.
- Frontend changes: run `cd frontend && npm run lint` and `npm run build` from repo root or `cd frontend && npm run build`.
- If API contracts change, update frontend client, Swagger/docs, and relevant docs under `docs/`.
- If migrations/config change, document new env keys and migration notes.
- Do not overwrite unrelated user changes in the worktree.
