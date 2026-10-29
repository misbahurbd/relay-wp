# Architecture

This document describes how the project is structured and the rules that keep it maintainable. Decisions that change it should be recorded as ADRs in [`docs/adr/`](./adr/).

---

## 0. Decisions

| Area             | Decision                                                                      | Reason                                                           |
| ---------------- | ----------------------------------------------------------------------------- | ---------------------------------------------------------------- |
| Repo             | Monorepo (`apps/api`, `apps/web`, `packages/*`)                               | API and UI changes land in one PR                                |
| Backend language | Go 1.27                                                                       | Low memory, high concurrency, single static binary               |
| HTTP             | `chi` + `huma`                                                                | Standard `net/http`, no lock-in; OpenAPI generated from Go types |
| Database         | Managed PostgreSQL                                                            | No DB to operate; API box stays small                            |
| DB access        | `pgx` + `sqlc`                                                                | Fastest driver, no runtime reflection, plain SQL                 |
| Migrations       | `goose`                                                                       | Plain SQL files                                                  |
| Jobs / queue     | `river` (Postgres)                                                            | No Redis to run; transactional enqueue                           |
| Auth             | Go-issued JWTs (access) + rotating refresh tokens                             | Stateless requests, revocable sessions                           |
| Token transport  | httpOnly cookies (web) **and** `Authorization: Bearer` (mobile / API clients) | One auth system for every client                                 |
| Tenancy          | Workspaces with members and roles                                             | Teams from day one                                               |
| Frontend         | TanStack Start + TanStack Query/Form/Table                                    | SSR, type-safe routing                                           |
| UI               | Tailwind v4 + shadcn/ui                                                       | Owned, customizable components                                   |
| TS lint/format   | Biome                                                                         | One fast tool                                                    |
| Go lint          | `golangci-lint`                                                               |                                                                  |
| Task runner      | Taskfile (`go-task`)                                                          | One entry point for Go + Node                                    |
| Package manager  | pnpm (workspaces)                                                             |                                                                  |
| CI / deploy      | GitHub Actions → Docker images; host chosen later                             | Portable                                                         |

### Resource budget

- **API:** 0.5 vCPU / 1 GB RAM. **Database:** managed, outside this budget.
- Go runtime: Go detects the container CPU limit automatically; set `GOMEMLIMIT≈800MiB` so the GC works before the container hits its limit.
- `pgxpool` max ≈ 10–15 connections per instance.
- One binary, three modes: `api`, `worker`, `all`. Start with `all` on one instance; split `worker` into its own container when load requires it.
- Scale horizontally: the API keeps no state in memory between requests, so add instances behind a load balancer.

> **Managed Postgres gotcha:** River uses `LISTEN/NOTIFY`, which breaks behind a transaction-mode pooler (Neon/Supabase pooled URLs, PgBouncer). Use two URLs: `DATABASE_URL` (pooled, for queries) and `DATABASE_DIRECT_URL` (direct, for River and migrations).

---

## 1. Repository layout

```
<repo>/
├── apps/
│   ├── api/                 # Go backend
│   └── web/                 # TanStack Start frontend
├── packages/
│   └── api-client/          # TS client + types generated from OpenAPI
├── deploy/
│   ├── docker-compose.yml   # local: postgres, mailpit, rustfs (S3)
│   └── docker/
│       ├── api.Dockerfile
│       └── web.Dockerfile
├── docs/
│   ├── ARCHITECTURE.md
│   └── adr/
├── .github/workflows/
│   ├── api.yml              # runs on apps/api/** changes
│   └── web.yml              # runs on apps/web/**, packages/** changes
├── Taskfile.yml
├── pnpm-workspace.yaml
├── package.json             # root scripts + Biome
├── biome.json
├── .editorconfig
├── .env.example
└── README.md
```

---

## 2. Backend (Go)

### Layout

```
apps/api/
├── cmd/
│   └── relay/main.go          # single binary: `relay api | worker | all | migrate | openapi`
├── internal/
│   ├── app/                   # COMPOSITION ROOT: builds deps, wires features
│   │   ├── app.go
│   │   ├── routes.go
│   │   └── workers.go         # registers River workers
│   │
│   ├── features/              # business logic, one folder per feature
│   │   ├── auth/              # signup, login, sessions, passwords, social sign-in
│   │   ├── profiles/
│   │   ├── workspaces/
│   │   ├── onboarding/
│   │   ├── whatsapp/
│   │   ├── team/              # roles, members, teams, invitations, access log
│   │   └── <feature>/         # one Go package per layer
│   │       ├── handler/       # routes.go + HTTP handlers: decode → service → encode
│   │       ├── service/       # business rules (no HTTP, no SQL)
│   │       ├── repository/    # data access (wraps sqlc) + <area>_queries.sql
│   │       ├── dto/           # request/response shapes
│   │       ├── model/         # domain types, constants, errors, job args
│   │       └── jobs/          # River workers (if any)
│   │
│   ├── platform/              # infrastructure only, no business logic
│   │   ├── config/            # typed env config
│   │   ├── database/          # pgx pool, WithTx helper
│   │   ├── httpx/             # server, error mapping, pagination
│   │   ├── middleware/        # request ID, logging, recover, CORS, auth, workspace, rate limit
│   │   ├── jwt/               # sign/verify (Ed25519)
│   │   ├── logger/            # slog setup
│   │   ├── queue/             # River client
│   │   ├── mailer/
│   │   └── storage/           # S3
│   │
│   └── shared/
│       ├── apperr/            # typed errors → HTTP status
│       ├── authctx/           # current user / workspace from context
│       └── id/                # UUIDv7
├── db/
│   ├── migrations/
│   ├── sqlc.yaml
│   └── gen/                   # sqlc output (generated, committed)
├── openapi/openapi.json       # generated, committed
├── .golangci.yml
├── .air.toml
└── go.mod
```

### Feature layout: layer packages

Each feature is split into one Go package per layer ([ADR 0003](adr/0003-feature-file-layout.md)):

```text
features/team/
├── handler/     routes.go  handler.go  roles_handler.go  members_handler.go  …
├── service/     service.go  roles_service.go  members_service.go  invitations_accept.go  email.go  …
├── repository/  repository.go  log_writer.go  roles_queries.sql  members_queries.sql  …
├── dto/         dto.go  roles_dto.go  members_dto.go  …
├── model/       model.go  errors.go  roles_model.go  roles_defaults.go  …
└── jobs/        jobs.go
```

Imports only go down: `handler → service, dto, model`; `service → repository, model`; `dto → model`; `repository → model`; `jobs → service, repository, model`; `model` imports no other layer. Inside a layer, files are named by area (`roles_service.go`), and shared code keeps the plain name. A name is exported only when another layer uses it. `internal/app` imports layers with the feature as prefix (`authservice`, `teamhandler`). sqlc reads every `.sql` file in each feature's `repository/` folder.

### Rules

1. **One-way dependencies:** `handler → service → repository`, each its own package. Services never see `http.Request`; handlers never touch SQL.
2. **Features don't import each other's internals.** A feature that needs another declares a small interface in its own package; `internal/app` injects the implementation.
3. **All wiring lives in `internal/app`.** Constructors (`New(...)`), no globals, no `init()`.
4. **Errors:** services return `apperr` types (`NotFound`, `Conflict`, `Forbidden`, `Unauthorized`, `Validation`); `httpx` maps them to `{ code, message, details }`.
5. **Tenancy:** every tenant-owned table has `workspace_id`. Repository methods take `workspaceID` explicitly. Postgres RLS can be added later.
6. **Transactions:** `database.WithTx(ctx, fn)`. Services own transactions; repositories accept sqlc's `DBTX`.
7. **Side effects** (email, outbound calls) are River jobs enqueued in the same transaction (outbox pattern).
8. **Inbound webhooks:** verify signature → store raw event + enqueue job in one transaction → return `200` quickly. Process asynchronously, idempotently (unique on external event ID).

### Factory pattern: where and where not

**Rule: factories build things once, at startup or once per tenant (then cached), never once per request.** A factory is used only where the implementation is _chosen_ (by config, by tenant, or by event type). Everywhere else, a plain `New(...)` constructor is enough.

#### Use a real factory (pick an implementation by config or key)

| Where                                      | Selected by                           | Implementations                                                                | Built                                             |
| ------------------------------------------ | ------------------------------------- | ------------------------------------------------------------------------------ | ------------------------------------------------- |
| `platform/mailer`                          | `MAILER_DRIVER`                       | smtp (Mailpit locally), resend/ses, log (tests)                                | Once at boot                                      |
| `platform/storage`                         | `STORAGE_DRIVER`                      | s3 (RustFS locally), local disk, memory (tests)                                | Once at boot                                      |
| `platform/queue`                           | `QUEUE_DRIVER` (not yet)              | river only today, so a plain `New`; becomes a factory when nats/redis is added | Once at boot                                      |
| `platform/cache`                           | `CACHE_DRIVER`                        | in-memory LRU now; redis when running several instances                        | Once at boot                                      |
| `platform/logger`                          | `LOG_FORMAT`, `LOG_LEVEL`             | JSON (prod), text (dev)                                                        | Once at boot                                      |
| `platform/jwt`                             | `kid` in token header                 | current key, previous key (rotation)                                           | Once at boot; lookup is a map read                |
| Outbound messaging / third-party providers | provider type stored on the workspace | one client per provider                                                        | **Per workspace, cached** (see below)             |
| Webhook / job handlers                     | event type string                     | one handler per event type                                                     | Registry map filled at boot; lookup is a map read |

Layout for each factory package:

```
platform/mailer/
├── mailer.go     # Mailer interface + New(cfg) factory (switch on driver, error on unknown)
├── smtp.go
├── resend.go
└── log.go
```

#### Per-tenant factory (the one place a factory could become a bottleneck)

When each workspace has its own credentials or settings for an external client:

- Build the client the first time a workspace needs it, then **cache it** (LRU with TTL, size-capped for the 1 GB budget).
- Use `singleflight` so 1 000 concurrent requests for a cold workspace build the client **once**, not 1 000 times.
- Clients share one `http.Client` / transport (one connection pool); only credentials differ per workspace.
- Clear the cache entry when the workspace's credentials change.

#### Do NOT use a factory

| Where                                    | Use instead                                        | Why                                                                                                         |
| ---------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- |
| Feature handlers, services, repositories | Plain `New(deps...)` called once in `internal/app` | Only one implementation exists; a factory adds indirection for nothing                                      |
| Domain models, DTOs, errors              | Struct literals / small helpers                    | Hot path, allocations matter                                                                                |
| Request-scoped values (user, workspace)  | `context` via `shared/authctx`                     | Already resolved by middleware                                                                              |
| Dependency injection                     | Manual wiring in `internal/app`                    | No runtime DI container (`dig`/`fx`): reflection, slower startup, errors at runtime instead of compile time |

#### Performance guardrails

- No reflection in factories; a `switch` on a config string at boot.
- Unknown driver or missing config → **fail at startup**, never at first request.
- Factories return interfaces; callers hold the built instance. Never call a factory from a handler.
- Tests swap implementations by passing a different driver or a fake to `New(...)`, so no mocking framework is needed.

---

## 3. Authentication

### Tokens

| Token   | Format                                                      | Lifetime          | Stored                          |
| ------- | ----------------------------------------------------------- | ----------------- | ------------------------------- |
| Access  | JWT signed with Ed25519 (`sub`, `sid`, `exp`, `iat`, `jti`) | ~15 min           | Nowhere server-side             |
| Refresh | Opaque random 256-bit string                                | ~30 days, sliding | SHA-256 hash in `auth_sessions` |
- **Rotation:** every refresh returns a new refresh token and invalidates the old one. Reusing an old token revokes the whole session family (theft detection).
- **Revocation:** logout / "log out all devices" / admin block deletes `auth_sessions` rows; access tokens expire within 15 min. For instant blocks, keep a small revoked-`sid` check (cached).
- **Roles are not in the JWT.** Workspace membership and role are looked up per request (cached briefly) so role changes apply immediately.
- **Key rotation:** keys have a `kid`; verifier accepts current + previous key.

### Transport (both)

- **Web:** API sets `access_token` (httpOnly, Secure, SameSite=Lax, Path=/) and `refresh_token` (httpOnly, Secure, SameSite=Strict, Path=/v1/auth/refresh) cookies.
- **Mobile / API clients:** `POST /v1/auth/token` returns tokens in the body; clients send `Authorization: Bearer …`.
- **Middleware order:** `Authorization` header first, then cookie.
- **CSRF (cookie requests only):** SameSite plus an `Origin`/`Referer` allow-list check on non-GET requests.

### Workspace scoping

Workspace-scoped routes live under `/v1/workspace/...` and name the workspace in the `X-Workspace-ID` header, so web and mobile scope requests the same way ([ADR 0005](adr/0005-workspace-header.md)). A missing header gets 422. `platform/middleware.RequireWorkspace` checks membership (through `workspaces.Directory`) and puts `workspaceID` and role into context (`shared/authctx`). A non-member gets 404, so workspace IDs can't be probed; owner-only routes pass the allowed roles and others get 403.

---

## 4. Frontend (TanStack Start)

### Layout

```
apps/web/src/
├── routes/                       # THIN: layouts, loaders, guards
│   ├── __root.tsx
│   ├── index.tsx
│   ├── _auth.tsx                 # logged-out layout
│   ├── _auth/login.tsx
│   ├── _auth/signup.tsx
│   ├── _app.tsx                  # authenticated layout (beforeLoad → require user)
│   ├── _app/onboarding.tsx       # create first workspace
│   └── _app/_workspace/
│       ├── route.tsx             # load workspace, check membership
│       ├── index.tsx             # dashboard
│       ├── members.tsx
│       └── settings/
│
├── features/
│   └── <feature>/            # see ADR 0004
│       ├── api/                  # <feature>.api.ts, or .keys.ts + .queries.ts + .mutations.ts
│       ├── components/
│       │   ├── <area>/           # e.g. members/, roles/, invite/ (once the feature has 2+ areas)
│       │   └── kit/              # UI pieces shared by the feature's areas
│       ├── hooks/
│       ├── schemas/              # <feature>.schema.ts (Zod)
│       ├── server/               # server functions (if any)
│       ├── types.ts
│       └── index.ts              # PUBLIC API
│
├── components/
│   ├── ui/                       # shadcn primitives
│   └── layout/
├── lib/
│   ├── api.ts                    # configured client (credentials: include; SSR cookie forwarding)
│   ├── query-client.ts
│   ├── env.ts                    # Zod-validated env
│   └── utils.ts
├── hooks/
├── styles/
└── router.tsx
```

### Rules

1. **Route files stay thin;** they render feature components.
2. **Import features only through `index.ts`.** Enforce with Biome `noRestrictedImports` patterns (add `dependency-cruiser` in CI if stricter rules are needed).
3. **Never hand-write API types;** use `packages/api-client`.
4. **Each feature owns its query keys;** mutations invalidate through them.
5. **Promote to `components/` only when two features need it.**
6. **Name feature files by feature and role** (`team.queries.ts`, `auth.schema.ts`) and group components by area; see [ADR 0004](adr/0004-web-feature-file-layout.md).
7. **Auth on SSR:** server-side requests forward the incoming cookies to the API. On `401`, the client calls `/v1/auth/refresh` once, then retries.

---

## 5. Contract pipeline

```
Go types (huma) ──► openapi/openapi.json ──► @hey-api/openapi-ts ──► packages/api-client
```

- `task gen` runs sqlc, exports OpenAPI, regenerates the TS client.
- CI runs `task gen` and fails if `git diff` is not empty.

---

## 6. Local development

- `deploy/docker-compose.yml`: Postgres 18, Mailpit (SMTP + web UI), RustFS (S3-compatible; MinIO no longer publishes images). Production uses managed Postgres and S3. Host ports are set in `.env`.
- `task dev` starts compose, the API with `air`, and the web dev server.
- `.env.example` at the root lists every variable; apps read their own subset.

### Required tools

Go 1.27, Node 24, pnpm 12, Docker, `go-task`, `air`, `golangci-lint`, `sqlc`, `goose`.

---

## 7. CI / CD

- **`api.yml`:** `golangci-lint`, `go test` (Postgres service container), `sqlc diff`, OpenAPI drift check, Docker build.
- **`web.yml`:** Biome check, `tsc --noEmit`, Vitest, build, Docker build.
- On `main`: push images to GHCR. Deploy target to be decided.
- **Images:** API is a multi-stage build onto distroless/static (~15–25 MB); web is a Node runtime image running the TanStack Start server output.

---

## 8. Conventions

- **IDs:** UUIDv7. **Time:** `timestamptz`, UTC; `created_at`/`updated_at` everywhere.
- **API:** REST under `/v1`, cursor pagination, `snake_case` JSON, `Idempotency-Key` header on create endpoints that clients may retry.
- **Soft deletes** only where needed.
- **Commits:** Conventional Commits. **Decisions:** ADRs in `docs/adr/`.
- **Testing:** service tests with fakes, repository tests against real Postgres (testcontainers), Playwright for critical flows.

---

## 9. Phases

| Phase                          | Scope                                                                                        |
| ------------------------------ | -------------------------------------------------------------------------------------------- |
| **0. Core setup** (done)       | Monorepo, tooling, compose, CI, empty API and web apps that boot, `/healthz`                 |
| **1. Backend platform** (done) | Config, logger, DB pool, migrations, middleware, error mapping, River, OpenAPI → TS pipeline |
| **2. Auth**                    | Signup, login, JWT + refresh rotation, logout, email verification, password reset            |
| **3. Workspaces**              | Workspaces, members, invites, roles, workspace middleware                                    |
| **4. Frontend shell**          | Auth pages, protected layout, workspace switcher, settings, shadcn                           |
| **5. Product features**        | Same feature pattern                                                                         |
| **6. Billing**                 | Plans, checkout, webhooks, limits                                                            |
| **7. Production**              | Deploy target, OpenTelemetry, error tracking, rate limiting, backups                         |
