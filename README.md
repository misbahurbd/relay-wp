# Relay

Monorepo for the Relay SaaS: Go API (`apps/api`) and TanStack Start web app (`apps/web`).
See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for structure and conventions.

## Requirements

Go 1.27, Node 24, pnpm 12, Docker, and:

```bash
brew install go-task air golangci-lint sqlc goose
```

## Getting started

```bash
task setup   # install deps, create .env
task dev     # Postgres/Mailpit/S3 + API (hot reload) + web
```

| Service    | URL                                                 |
| ---------- | --------------------------------------------------- |
| Web        | http://localhost:3000                               |
| API        | http://localhost:8080 (`/healthz`, docs at `/docs`) |
| Mailpit    | http://localhost:8025                               |
| S3 console | http://localhost:9001                               |

## Common tasks

| Command                    | What it does                  |
| -------------------------- | ----------------------------- |
| `task gen`                 | sqlc, OpenAPI spec, TS client |
| `task lint` / `task fmt`   | Lint / format Go and TS       |
| `task test`                | Go tests                      |
| `task migrate:new -- name` | New SQL migration             |
| `task migrate:up`          | Apply migrations              |

Run `task` to list everything.
