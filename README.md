# Relay

Monorepo for the Relay SaaS: a Go API (`apps/api`) and a TanStack Start web app (`apps/web`).

- Architecture, structure and conventions: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md)
- Hard rules for contributors and coding agents: [CLAUDE.md](CLAUDE.md) (also linked as `AGENTS.md`)

---

## Contents

1. [Prerequisites](#1-prerequisites)
2. [Install the tools (macOS)](#2-install-the-tools-macos)
3. [Install the tools (Linux)](#3-install-the-tools-linux)
4. [Verify the toolchain](#4-verify-the-toolchain)
5. [First-time project setup](#5-first-time-project-setup)
6. [Configure `.env`](#6-configure-env)
7. [Run the project](#7-run-the-project)
8. [Daily workflow](#8-daily-workflow)
9. [Database and migrations](#9-database-and-migrations)
10. [Code generation](#10-code-generation)
11. [Lint, format, type-check and test](#11-lint-format-type-check-and-test)
12. [Docker images](#12-docker-images)
13. [CI](#13-ci)
14. [Repository layout](#14-repository-layout)
15. [Troubleshooting](#15-troubleshooting)

---

## 1. Prerequisites

| Tool                  | Version                                          | Used for                             |
| --------------------- | ------------------------------------------------ | ------------------------------------ |
| Go                    | 1.27.x                                           | API                                  |
| Node.js               | 24 (see `.nvmrc`)                                | Web app, tooling                     |
| pnpm                  | 12 (pinned in `package.json` → `packageManager`) | JS package manager / workspaces      |
| Docker                | Docker Desktop or OrbStack                       | Local Postgres, Mailpit, S3          |
| Task (`go-task`)      | 3.x                                              | Single entry point for every command |
| golangci-lint         | 2.x                                              | Go lint + format                     |
| air (`air-verse/air`) | 1.x                                              | API hot reload                       |
| sqlc                  | 1.x                                              | Type-safe Go from SQL                |
| goose                 | 3.x                                              | SQL migrations                       |

---

## 2. Install the tools (macOS)

```bash
# Go, Node and Docker, if you don't have them yet
brew install go node@24
brew install --cask orbstack          # or: brew install --cask docker

# Project tools
brew install go-task golangci-lint sqlc goose

# air: install with Go. Homebrew's "air" formula is an unrelated R formatter.
go install github.com/air-verse/air@latest

# pnpm: use the version pinned in package.json
corepack enable
```

`go install` puts binaries in `$(go env GOPATH)/bin` (usually `~/go/bin`). Add it to your `PATH` if it isn't there yet:

```bash
echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.zshrc && source ~/.zshrc
```

If you use `nvm`, run `nvm use` in the repo root to switch to the Node version in `.nvmrc`.

---

## 3. Install the tools (Linux)

- **Go:** download from <https://go.dev/dl/>.
- **Node:** install with `nvm install` (reads `.nvmrc`), then `corepack enable`.
- **Docker:** install Docker Engine + the Compose plugin.
- **Task:** `sh -c "$(curl -fsSL https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin`

Then install the Go-based tools:

```bash
go install github.com/air-verse/air@latest
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
go install github.com/pressly/goose/v3/cmd/goose@latest
curl -sSfL https://golangci-lint.run/install.sh | sh -s -- -b "$(go env GOPATH)/bin"
```

---

## 4. Verify the toolchain

```bash
go version && node -v && pnpm -v && docker version --format '{{.Server.Version}}'
```

```bash
task --version && golangci-lint version && air -v && sqlc version && goose --version
```

Every command should print a version. If `air -v` prints nothing or shows an R formatter, see [Troubleshooting](#15-troubleshooting).

---

## 5. First-time project setup

```bash
git clone https://github.com/misbahurbd/relay-wp.git
cd relay-wp
task setup
```

`task setup`:

1. Copies `.env.example` to `.env` (only if `.env` doesn't exist yet).
2. Runs `pnpm install` for the whole workspace.
3. Runs `go mod download` for the API.

---

## 6. Configure `.env`

All configuration lives in a single `.env` at the repo root. Task loads it automatically for every command. `.env` is gitignored; `.env.example` is the committed template, so **add every new variable to `.env.example` too**.

| Group               | Variables                                                                             | Notes                                                                                                                |
| ------------------- | ------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| Local service ports | `POSTGRES_PORT`, `MAILPIT_SMTP_PORT`, `MAILPIT_UI_PORT`, `S3_PORT`, `S3_CONSOLE_PORT` | Host ports for docker-compose. Change them if another project already uses the defaults.                             |
| API                 | `APP_ENV`, `HTTP_ADDR`, `LOG_LEVEL`, `LOG_FORMAT`, `CORS_ALLOWED_ORIGINS`             |                                                                                                                      |
| Database            | `DATABASE_URL`, `DATABASE_DIRECT_URL`, `DATABASE_MAX_CONNS`                           | Two URLs: see below.                                                                                                 |
| Mail                | `MAILER_DRIVER`, `SMTP_HOST`, `SMTP_PORT`                                             | Locally, mail goes to Mailpit.                                                                                       |
| Storage             | `STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`        | Locally, S3 is RustFS.                                                                                               |
| Runtime             | `GOMEMLIMIT`                                                                          | Keep below the container memory limit (1 GB in production).                                                          |
| Web                 | `WEB_PORT`, `VITE_API_URL`                                                            | Dev server port (fails if taken; keep `CORS_ALLOWED_ORIGINS` in sync) and the URL the browser uses to reach the API. |

**Two database URLs:**

- `DATABASE_URL` is the connection for normal queries. In production it can go through a connection pooler (Neon, Supabase, PgBouncer).
- `DATABASE_DIRECT_URL` must connect **without** a pooler. River (background jobs) needs `LISTEN/NOTIFY`, which poolers break, and migrations use it too.

Locally both point at the same Postgres.

**If you change a port, update the URLs that use it.** For example, if you set `POSTGRES_PORT=55440`, change `localhost:5432` to `localhost:55440` in both database URLs.

---

## 7. Run the project

```bash
task dev
```

This:

1. Starts Postgres, Mailpit and S3 in Docker and waits until they are healthy (`task up`).
2. Starts the API with hot reload (`air`): it rebuilds on every `.go` or `.sql` change.
3. Starts the web dev server (Vite) with hot module reload.

Open (default ports):

| Service    | URL                             | Notes                                             |
| ---------- | ------------------------------- | ------------------------------------------------- |
| Web        | <http://localhost:3000>         |                                                   |
| API        | <http://localhost:8080/healthz> | Should return `{"status":"ok"}`                   |
| API docs   | <http://localhost:8080/docs>    | Generated from the OpenAPI spec                   |
| Mailpit    | <http://localhost:8025>         | Every email the app sends locally                 |
| S3 console | <http://localhost:9001>         | Log in with `S3_ACCESS_KEY` / `S3_SECRET_KEY`     |
| Postgres   | `localhost:5432`                | User / password / db: `relay` / `relay` / `relay` |

Press `Ctrl+C` to stop the API and web. The Docker services keep running; stop them with `task down`.

To run the pieces separately:

```bash
task up        # services only
task dev:api   # API only
task dev:web   # web only
```

---

## 8. Daily workflow

```bash
git pull
task setup       # only if dependencies changed
task dev
```

Before you push:

```bash
task fmt
task lint
task typecheck
task test
task gen && git diff --exit-code   # generated code must be committed and up to date
```

Run `task` on its own to list every available task.

| Command                                               | What it does                                         |
| ----------------------------------------------------- | ---------------------------------------------------- |
| `task setup`                                          | Install dependencies, create `.env`                  |
| `task up` / `task down`                               | Start / stop local services                          |
| `task reset`                                          | Stop services **and delete their data** (asks first) |
| `task dev`                                            | Services + API + web                                 |
| `task build`                                          | Build the API binary and the web app                 |
| `task gen`                                            | sqlc, OpenAPI spec, TS client                        |
| `task fmt`                                            | Format Go and TypeScript                             |
| `task lint`                                           | Lint Go and TypeScript                               |
| `task typecheck`                                      | Type-check TypeScript                                |
| `task test`                                           | Go tests (with the race detector)                    |
| `task migrate:new -- <name>`                          | Create a new SQL migration                           |
| `task migrate:up` / `migrate:down` / `migrate:status` | Apply / roll back / inspect migrations               |

---

## 9. Database and migrations

Migrations are plain SQL files in `apps/api/db/migrations/`, managed by goose. They run against `DATABASE_DIRECT_URL`.

```bash
task migrate:new -- create_users   # creates db/migrations/00001_create_users.sql (numbered in order)
task migrate:up                    # apply all pending migrations
task migrate:status                # list applied / pending
task migrate:down                  # roll back the latest migration
```

Each migration file has an `Up` and a `Down` section:

```sql
-- +goose Up
-- SQL to apply

-- +goose Down
-- SQL to undo
```

Connect to the local database:

```bash
docker exec -it relay-postgres-1 psql -U relay -d relay
```

To wipe local data and start fresh: `task reset`, then `task up` and `task migrate:up`.

---

## 10. Code generation

The API is the source of truth for the API contract:

```
Go types (huma) ──► apps/api/openapi/openapi.json ──► packages/api-client (TS client + TanStack Query hooks)
SQL (queries.sql) ──► apps/api/db/gen (sqlc)
```

```bash
task gen            # everything
task gen:sqlc       # Go from SQL (skips itself until the first queries.sql exists)
task gen:openapi    # export the OpenAPI spec from the API
task gen:client     # regenerate the TS client from the spec
```

Run `task gen` after changing API routes, request/response types, or SQL queries, and commit the result. CI fails if generated files are out of date.

**Never edit generated files by hand:**
- `apps/api/db/gen/**`
- `apps/api/openapi/openapi.json`
- `packages/api-client/src/gen/**`
- `apps/web/src/routeTree.gen.ts`

---

## 11. Lint, format, type-check and test

| Language        | Lint                 | Format                                    | Config                   |
| --------------- | -------------------- | ----------------------------------------- | ------------------------ |
| Go              | `golangci-lint run`  | `golangci-lint fmt` (gofumpt + goimports) | `apps/api/.golangci.yml` |
| TS / JSON / CSS | `pnpm check` (Biome) | `pnpm check:fix`                          | `biome.json`             |

Editor setup:
- **VS Code:** install the Biome and Go extensions. `.vscode/settings.json` already makes Biome the default formatter.
- **Go:** point your editor's format-on-save at `golangci-lint fmt`, or at `gofumpt` + `goimports`.

Tests: `task test` runs `go test -race ./...` in `apps/api`.

---

## 12. Docker images

Build from the repo root (both Dockerfiles expect the root as the build context):

```bash
docker build -f deploy/docker/api.Dockerfile -t relay-api .
docker build -f deploy/docker/web.Dockerfile -t relay-web .
```

Run them:

```bash
docker run --rm -p 8080:8080 --env-file .env relay-api
docker run --rm -p 3000:3000 relay-web
```

Inside a container, `localhost` is the container itself. When the API needs the database, point `DATABASE_URL` / `DATABASE_DIRECT_URL` at `host.docker.internal:<POSTGRES_PORT>` instead of `localhost`.

- **API:** static binary on distroless (non-root), about 18 MB, `GOMEMLIMIT=800MiB` by default.
- **Web:** Node 24 Alpine running the TanStack Start (Nitro) server output.

---

## 13. CI

GitHub Actions, one workflow per app. Each runs only when its files change.

| Workflow                    | Runs                                                                                        |
| --------------------------- | ------------------------------------------------------------------------------------------- |
| `.github/workflows/api.yml` | golangci-lint, `go vet`, `go test -race` (with Postgres), OpenAPI drift check, Docker build |
| `.github/workflows/web.yml` | Biome, API client drift check, type-check, build, Docker build                              |

On pushes to `main`, images are pushed to GHCR as `ghcr.io/<owner>/<repo>/{api,web}:<sha>`.

---

## 14. Repository layout

```
apps/
  api/                 Go API (cmd/relay, internal/{app,features,platform,shared}, db/)
  web/                 TanStack Start app (src/{routes,features,components,lib,hooks,styles})
packages/
  api-client/          Generated TS client
deploy/
  docker-compose.yml   Local services
  docker/              Dockerfiles
docs/
  ARCHITECTURE.md      Architecture and conventions
  adr/                 Architecture decision records
CLAUDE.md              Hard rules for contributors and agents (AGENTS.md links here)
Taskfile.yml           All commands
```

Where each kind of code goes is defined in [CLAUDE.md](CLAUDE.md#1-folder-structure-must). Follow it exactly.

---

## 15. Troubleshooting

**`Bind for :::5432 failed: port is already allocated`** (or 1025, 8025, 9000, 9001)
Another container or local service is using that port. Either stop it, or change the port in `.env` (for example `POSTGRES_PORT=55440`) and update `DATABASE_URL` / `DATABASE_DIRECT_URL` to match. Find what's using a port with `lsof -nP -iTCP:5432 -sTCP:LISTEN`.

**`air: command not found`, or `air` is an R formatter**
Homebrew's `air` is a different tool. Run `brew uninstall air`, then `go install github.com/air-verse/air@latest`, and make sure `$(go env GOPATH)/bin` is on your `PATH`.

**`pnpm: command not found` or the wrong pnpm version**
Run `corepack enable` in the repo root; it uses the version pinned in `package.json`.

**Background jobs never run in production**
`DATABASE_DIRECT_URL` probably points at a pooled connection. River needs a direct connection (no PgBouncer / pooler).

**CI fails with a `git diff` after codegen**
Run `task gen` locally and commit the changed files.

**`openapi-ts` fails with `Cannot read properties of undefined (reading 'AnyKeyword')`**
`packages/api-client` must stay on TypeScript 6: the generator needs the JavaScript compiler API, which TypeScript 7 removed. Don't upgrade it to 7 until `@hey-api/openapi-ts` supports it.

**Type errors after pulling**
Run `pnpm install`, then `task gen`.
