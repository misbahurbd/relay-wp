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
9. [Authentication](#9-authentication)
10. [Database and migrations](#10-database-and-migrations)
11. [Code generation](#11-code-generation)
12. [Lint, format, type-check and test](#12-lint-format-type-check-and-test)
13. [Docker images](#13-docker-images)
14. [CI](#14-ci)
15. [Repository layout](#15-repository-layout)
16. [Troubleshooting](#16-troubleshooting)

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

Every command should print a version. If `air -v` prints nothing or shows an R formatter, see [Troubleshooting](#16-troubleshooting).

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
4. Adds a JWT signing key to `.env` if it has none (`task keygen:env`).

---

## 6. Configure `.env`

All configuration lives in a single `.env` at the repo root. Task loads it automatically for every command. `.env` is gitignored; `.env.example` is the committed template, so **add every new variable to `.env.example` too**.

| Group               | Variables                                                                                                                                                                               | Notes                                                                                                                |
| ------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| Local service ports | `POSTGRES_PORT`, `MAILPIT_SMTP_PORT`, `MAILPIT_UI_PORT`, `S3_PORT`, `S3_CONSOLE_PORT`                                                                                                   | Host ports for docker-compose. Change them if another project already uses the defaults.                             |
| API                 | `APP_ENV`, `HTTP_ADDR`, `LOG_LEVEL`, `LOG_FORMAT`, `CORS_ALLOWED_ORIGINS`                                                                                                               |                                                                                                                      |
| Database            | `DATABASE_URL`, `DATABASE_DIRECT_URL`, `DATABASE_MAX_CONNS`                                                                                                                             | Two URLs: see below.                                                                                                 |
| Mail                | `MAILER_DRIVER`, `SMTP_HOST`, `SMTP_PORT`                                                                                                                                               | Locally, mail goes to Mailpit.                                                                                       |
| Storage             | `STORAGE_DRIVER`, `S3_ENDPOINT`, `S3_ACCESS_KEY`, `S3_SECRET_KEY`, `S3_BUCKET`                                                                                                          | Locally, S3 is RustFS.                                                                                               |
| Auth                | `APP_BASE_URL`, `JWT_SIGNING_KEY`, `JWT_SIGNING_KEY_ID`, `JWT_PREVIOUS_PUBLIC_KEYS`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`, `COOKIE_DOMAIN`, `COOKIE_SECURE`, `PASSWORD_BREACH_CHECK` | `task setup` generates a local signing key. Set `COOKIE_SECURE=false` only for plain-HTTP local development.         |
| Runtime             | `GOMEMLIMIT`                                                                                                                                                                            | Keep below the container memory limit (1 GB in production).                                                          |
| Web                 | `WEB_PORT`, `VITE_API_URL`                                                                                                                                                              | Dev server port (fails if taken; keep `CORS_ALLOWED_ORIGINS` in sync) and the URL the browser uses to reach the API. |

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
| Readiness  | <http://localhost:8080/readyz>  | `ok` only when the database answers, else `503`   |
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

### API run modes

The API is one binary, `relay`, with several commands:

| Command                          | What it runs                                                                                            |
| -------------------------------- | ------------------------------------------------------------------------------------------------------- |
| `relay all`                      | HTTP server and background worker in one process (default; used by `task dev:api` and the Docker image) |
| `relay api`                      | HTTP server only. It can queue jobs but doesn't run them.                                               |
| `relay worker`                   | Background worker only                                                                                  |
| `relay migrate up\|down\|status` | Database migrations                                                                                     |
| `relay openapi`                  | Prints the OpenAPI spec                                                                                 |

Start with `all` on one small instance. When load grows, run `api` and `worker` as separate containers from the same image.

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

## 9. Authentication

Password sign-in, Google and Facebook are built (Apple comes later). The full design is in [docs/PHASE-2-AUTH.md](docs/PHASE-2-AUTH.md).

| Flow                 | Endpoints                                                                                                                                  |
| -------------------- | ------------------------------------------------------------------------------------------------------------------------------------------ |
| Sign up and verify   | `POST /v1/auth/signup`, `POST /v1/auth/verify-email`, `POST /v1/auth/verify-email/resend`                                                  |
| Sign in and out      | `POST /v1/auth/login`, `POST /v1/auth/token/refresh`, `POST /v1/auth/logout`, `POST /v1/auth/logout-all`                                   |
| Passwords            | `POST /v1/auth/password/forgot`, `POST /v1/auth/password/reset`, `POST /v1/auth/password/change`                                           |
| Devices and activity | `GET /v1/auth/sessions`, `DELETE /v1/auth/sessions/{id}`, `GET /v1/auth/events`                                                            |
| Profile              | `GET /v1/me`, `PATCH /v1/me/profile`, `PATCH /v1/me/preferences`                                                                           |
| Social sign-in       | `GET /v1/auth/providers`, `GET /v1/auth/oauth/{provider}/start`, `…/callback`, `POST /v1/auth/oauth/complete`, `POST /v1/auth/oauth/token` |
| Connected accounts   | `GET /v1/auth/identities`, `GET /v1/auth/oauth/{provider}/link`, `DELETE /v1/auth/identities/{provider}`, `POST /v1/auth/password/set`     |
| Meta compliance      | `POST /v1/auth/oauth/facebook/data-deletion`                                                                                               |
- **Web** (`client` omitted or `web`): tokens arrive as httpOnly cookies: `relay_access` (15 minutes) and `relay_refresh` (30 days, sent only to the refresh endpoint). Requests that change data using cookies must come from an origin in `CORS_ALLOWED_ORIGINS`.
- **Mobile** (`"client": "mobile"` on login): tokens arrive in the response body. Send `Authorization: Bearer <access_token>`, and `{"refresh_token": …}` to refresh. Send `X-Device-Name` so the sessions list shows the device.
- **Try it locally**: sign up from the API docs (`/docs`), then find the verification link in Mailpit. The link points at the web app's `/verify-email` page; until that page exists, post its `token` to `POST /v1/auth/verify-email`.

### Social sign-in

- **Web:** link the button to `GET /v1/auth/oauth/google/start?redirect_to=<page>&timezone=<IANA>`. After the provider, the browser lands on `redirect_to` signed in (cookies set). On failure it lands there with `?error=<code>`: `oauth_cancelled`, `email_in_use_sign_in_first`, `account_disabled`, `invalid_oauth_state`, `oauth_failed` or `server_error`.
- **Facebook without an email** (phone-number accounts): the browser lands on `redirect_to?complete_token=…`. Ask for an email and `POST /v1/auth/oauth/complete` with `{token, email}`.
- **Mobile:** open `…/start?client=mobile&redirect_to=<deep link in AUTH_ALLOWED_REDIRECTS>&code_challenge=<PKCE S256>` in the system browser. The app receives `<deep link>?code=…` and redeems it at `POST /v1/auth/oauth/token` with `{code, code_verifier}`.
- **Connecting from settings:** `GET /v1/auth/oauth/{provider}/link?redirect_to=<settings page>` needs a sign-in within `REAUTH_MAX_AGE`. The browser returns with `?linked=google`, or `?error=reauthentication_required` / `identity_in_use`.
- **Rules:** an email the provider doesn't vouch for is never attached to an existing account, and Facebook never vouches. The last sign-in method can't be disconnected.

**Setting up the providers** (development):

1. **Google:** in [Google Cloud Console](https://console.cloud.google.com), open Google Auth Platform:
   - Set the audience to **External** in **Testing**, and add your Gmail under **Test users**.
   - Add the scopes `openid`, `email` and `profile`.
   - Create a **Web application** client with the redirect URI `http://localhost:8080/v1/auth/oauth/google/callback`.
   - Put its ID and secret in `GOOGLE_CLIENT_ID` and `GOOGLE_CLIENT_SECRET`.
2. **Facebook:** in [Meta for Developers](https://developers.facebook.com), create an app with the **Facebook Login** use case:
   - Add the `email` and `public_profile` permissions.
   - Add the valid OAuth redirect URI `http://localhost:8080/v1/auth/oauth/facebook/callback`.
   - Copy the app ID and secret from **App settings → Basic** into `FACEBOOK_APP_ID` and `FACEBOOK_APP_SECRET`.
   - While the app is in Development mode, only people with a role on the app can sign in.
3. Restart the API. The log line `social sign-in providers enabled=[…]` lists what's on.
4. **Production:**
   - Register `https://<api domain>/v1/auth/oauth/{provider}/callback` with each provider.
   - Set `API_BASE_URL`.
   - Before switching Meta to Live, give it the data-deletion callback `https://<api domain>/v1/auth/oauth/facebook/data-deletion`.

**Rotating the signing key:** generate one with `task keygen`, move the old key's `kid:public-key` into `JWT_PREVIOUS_PUBLIC_KEYS`, and deploy. Remove the old entry after 15 minutes, once every token signed with it has expired.

---

## 10. Database and migrations

Migrations are plain SQL files in `apps/api/db/migrations/`, managed by goose and embedded in the binary. They run through `relay migrate` against `DATABASE_DIRECT_URL`, and `up` also applies River's job tables.

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

## 11. Code generation

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

## 12. Lint, format, type-check and test

| Language        | Lint                 | Format                                    | Config                   |
| --------------- | -------------------- | ----------------------------------------- | ------------------------ |
| Go              | `golangci-lint run`  | `golangci-lint fmt` (gofumpt + goimports) | `apps/api/.golangci.yml` |
| TS / JSON / CSS | `pnpm check` (Biome) | `pnpm check:fix`                          | `biome.json`             |

Editor setup:
- **VS Code:** install the Biome and Go extensions. `.vscode/settings.json` already makes Biome the default formatter.
- **Go:** point your editor's format-on-save at `golangci-lint fmt`, or at `gofumpt` + `goimports`.

Tests: `task test` runs `go test -race ./...` in `apps/api`.

- **Database tests** use `TEST_DATABASE_URL` when it's set (CI does this). Otherwise they start a throwaway Postgres container with testcontainers, so Docker must be running. Use `go test -short ./...` to skip them.
- **Live mail and storage tests** are skipped unless you point them at the local services:

```bash
cd apps/api && SMTP_TEST_PORT=1025 go test -run TestSMTPSend ./internal/platform/mailer/
```

```bash
cd apps/api && S3_TEST_ENDPOINT=http://localhost:9000 S3_ACCESS_KEY=relay S3_SECRET_KEY=relay-secret go test -run TestS3 ./internal/platform/storage/
```

---

## 13. Docker images

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

## 14. CI

GitHub Actions, one workflow per app. Each runs only when its files change.

| Workflow                    | Runs                                                                                        |
| --------------------------- | ------------------------------------------------------------------------------------------- |
| `.github/workflows/api.yml` | golangci-lint, `go vet`, `go test -race` (with Postgres), OpenAPI drift check, Docker build |
| `.github/workflows/web.yml` | Biome, API client drift check, type-check, build, Docker build                              |

On pushes to `main`, images are pushed to GHCR as `ghcr.io/<owner>/<repo>/{api,web}:<sha>`.

---

## 15. Repository layout

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

## 16. Troubleshooting

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
