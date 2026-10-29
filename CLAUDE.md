# Agent rules

These are **hard rules** for every coding agent and contributor. They are not suggestions. If a task seems to need breaking one, stop and ask the user first; never work around it silently.

Full rationale lives in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). Read it before your first change in a session.

---

## 1. Folder structure (MUST)

Never create a new top-level folder, a new folder under `apps/*/` or `internal/`, or a "utils"/"helpers"/"common"/"misc" package. Put code where this table says.

### Where things go

| You are adding                                                              | It goes in                                                                                       |
| --------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| Business logic for a domain (auth, workspaces, members, …)                  | `apps/api/internal/features/<feature>/`                                                          |
| HTTP handler                                                                | `features/<feature>/handler.go` (large feature: `<area>_handler.go`)                             |
| Business rules                                                              | `features/<feature>/service.go` (large feature: `<area>_service.go`)                             |
| SQL access                                                                  | `features/<feature>/repository.go` + `queries.sql` (large feature: `<area>_queries.sql`)         |
| Domain types                                                                | `features/<feature>/model.go` (large feature: `<area>_model.go`)                                 |
| Request/response shapes                                                     | `features/<feature>/dto.go` (large feature: `<area>_dto.go`)                                     |
| Feature errors                                                              | `features/<feature>/errors.go`                                                                   |
| Route registration                                                          | `features/<feature>/routes.go`                                                                   |
| Background job args + workers                                               | `features/<feature>/jobs.go`                                                                     |
| Tests                                                                       | next to the code: `features/<feature>/*_test.go`                                                 |
| Infrastructure (DB, mailer, storage, queue, cache, logger, JWT, middleware) | `apps/api/internal/platform/<name>/`                                                             |
| Small cross-feature primitives (`apperr`, `authctx`, `id`)                  | `apps/api/internal/shared/<name>/`                                                               |
| Wiring features and platform together                                       | `apps/api/internal/app/` only                                                                    |
| CLI commands / entry points                                                 | `apps/api/cmd/relay/`                                                                            |
| Migrations                                                                  | `apps/api/db/migrations/` (create with `task migrate:new -- <name>`)                             |
| Frontend page / layout / loader / guard                                     | `apps/web/src/routes/`                                                                           |
| Frontend domain code                                                        | `apps/web/src/features/<feature>/{api,components,hooks,schemas,server}/`, `types.ts`, `index.ts` |
| shadcn primitives                                                           | `apps/web/src/components/ui/` (add with the shadcn CLI)                                          |
| App shell (sidebar, topbar)                                                 | `apps/web/src/components/layout/`                                                                |
| Generic React hook used by 2+ features                                      | `apps/web/src/hooks/`                                                                            |
| Client setup (API client, query client, env)                                | `apps/web/src/lib/`                                                                              |
| Architecture decision                                                       | `docs/adr/NNNN-title.md`                                                                         |

A new feature starts with only the files it needs. Do not create empty placeholder files.

### Large features: area files ([ADR 0003](docs/adr/0003-feature-file-layout.md))

- A feature is **one Go package**: never create a subfolder inside `features/<feature>/`.
- Once a feature has two or more distinct areas, or a file passes ~400 lines, name its files `<area>_<kind>.go` and `<area>_queries.sql` (`roles_service.go`, `roles_handler.go`, `roles_dto.go`, `roles_model.go`, `roles_queries.sql`). `auth` and `team` follow this.
- Code shared by the whole feature keeps the plain names: `service.go` (the `Service` struct, `New`, interfaces, small helpers), `handler.go`, `dto.go`, `model.go`, `routes.go`, `errors.go`, `repository.go`, `jobs.go`, `email.go`.
- Add new code to its area's file. A new area gets its own `<area>_*` files; don't grow the shared files.

### Backend dependency rules

- Layers call one way only: `handler → service → repository`.
  - Handlers never run SQL or call a repository.
  - Services never import `net/http`, huma or chi.
  - Repositories contain no business rules.
- **A feature never imports another feature's package.** If feature A needs B, A declares a small interface in its own package, and `internal/app` passes B's service in.
- `platform/` and `shared/` never import `features/`.
- Only `internal/app` (and `cmd/`) may know about every feature.
- Every tenant-owned table has `workspace_id`, and every repository method on tenant data takes `workspaceID` as a parameter.
- Return `shared/apperr` errors from services; never write HTTP status codes outside `handler.go` / `platform/httpx`.
- **No `SELECT *`, ever.** Every query lists exactly the columns it needs, including `RETURNING` clauses and subqueries. This keeps secrets out of results, keeps sqlc's generated structs small, and stops a new column from silently changing existing queries.
- **Secrets never leave their query.** `users.password_hash` is selected only by the login and change-password queries. It is never part of a user model, DTO or API response, and never logged. The same applies to every stored token hash (`refresh_tokens.token_hash`, `email_tokens.token_hash`, `oauth_flows.state_hash`, `oauth_flows.completion_token_hash`, `invitations.token_hash`) and to OAuth flow secrets (`oauth_flows.code_verifier`, `nonce`), which only `ConsumeOAuthFlow` reads; `invitations.token_hash` is compared only in `GetInvitationByTokenHash`'s `WHERE` and never selected. Likewise `email_codes.code_hash` (read only by `GetActiveEmailCode`), `oauth_flows.email_code_hash` (read only by `GetOAuthFlowEmailCode`), and the encrypted Meta secrets `whatsapp_accounts.access_token` (only `GetAccountToken`) and `whatsapp_numbers.registration_pin` (only `GetNumberPin`).

### Frontend dependency rules

- Route files stay thin: loader, guard, layout, render feature components. No business logic, no direct `fetch`.
- Import a feature **only** through its `index.ts` (`#/features/<name>`). Biome enforces this; do not disable the rule.
- A feature never imports another feature's internals.
- A component moves to `components/` only when 2+ features use it.
- Never hand-write API types or fetch calls; use `@relay/api-client`.
- Each feature owns its query keys; mutations invalidate through them.

### Generated files (never edit by hand)

`apps/api/db/gen/**`, `apps/api/openapi/openapi.json`, `packages/api-client/src/gen/**`, `apps/web/src/routeTree.gen.ts`. Change the source and run `task gen`.

---

## 2. Factory pattern decision (MUST)

**Default: no factory.** Use a plain `New(deps...)` constructor, called once in `internal/app`.

Use a factory **only if every answer is yes**:

1. Are there **two or more real implementations** today (or a test fake _and_ a production one) that must be **chosen at runtime**?
2. Is the choice made by **config, a tenant (workspace) setting, or a key** (event type, key ID)?
3. Is it built **once at startup**, or **once per workspace and then cached**? (Never once per request.)

If any answer is no, write a plain constructor.

### Approved factory points

| Location                                                                                               | Selected by                                    | Lifetime                                            |
| ------------------------------------------------------------------------------------------------------ | ---------------------------------------------- | --------------------------------------------------- |
| `platform/mailer`                                                                                      | `MAILER_DRIVER`                                | Built once at startup                               |
| `platform/storage`                                                                                     | `STORAGE_DRIVER`                               | Built once at startup                               |
| `platform/queue` (today River only, so a plain `New`; becomes a factory when a second driver is added) | `QUEUE_DRIVER`                                 | Built once at startup                               |
| `platform/cache`                                                                                       | `CACHE_DRIVER`                                 | Built once at startup                               |
| `platform/logger`                                                                                      | `LOG_FORMAT`, `LOG_LEVEL`                      | Built once at startup                               |
| `platform/jwt`                                                                                         | token `kid`                                    | Keys loaded at startup; picking one is a map lookup |
| Webhook / job handler registry                                                                         | event type                                     | Map filled at startup                               |
| Per-workspace external clients (e.g. messaging providers)                                              | provider + credentials stored on the workspace | Per workspace, **cached**                           |
| `features/auth` OAuth provider registry ([ADR 0001](docs/adr/0001-oauth-provider-registry.md))         | `{provider}` path value; only configured ones  | Built once at startup                               |

Adding a factory that is not in this table requires an ADR in `docs/adr/` **and** a new row here. Ask the user first.

### Never a factory

- Feature handlers, services and repositories (one implementation each).
- Models, DTOs and errors: use struct literals.
- Request-scoped values: use `context` via `shared/authctx`.
- Dependency injection: wire by hand in `internal/app`. No `dig`, no `fx`, no reflection-based containers.

### How a factory must be written

- The interface and `New(cfg)` live in the package root file (e.g. `mailer.go`); one file per implementation (`smtp.go`, `resend.go`, `log.go`).
- Use a plain `switch` on the config value. No reflection, no plugin loading.
- An unknown driver or missing config **returns an error at startup**; never fail on the first request.
- Handlers and services hold the built instance; they **never call a factory**.
- Per-workspace factories must use:
  - a size-capped LRU cache with a TTL,
  - `singleflight`, so concurrent requests for an uncached workspace build the client once,
  - one shared `http.Client`/transport for every client,
  - removal of the cached entry when the workspace's credentials change.

---

## 3. Before you say a task is done (MUST)

Run these and make sure they pass. Report any failure honestly; do not hide it.

```bash
task lint        # golangci-lint + Biome
task typecheck
task test
task gen && git diff --exit-code   # generated code is up to date
```

Then check your change against sections 1 and 2:

- [ ] Every new file is in the location the table names.
- [ ] No feature imports another feature.
- [ ] No new factory outside the approved list.
- [ ] No generated file was edited by hand.
- [ ] Tenant data is scoped by `workspace_id`.
- [ ] No `SELECT *` or `RETURNING *` in any query (`grep -rn '\*' --include='*.sql' apps/api/internal` finds none).
- [ ] No password or token hash appears in a model, DTO, response or log line.

---

## 4. Conventions

- Go: `gofumpt` + `goimports` (via `golangci-lint fmt`). TypeScript: Biome.
- IDs: UUIDv7. Time: `timestamptz`, UTC.
- REST under `/v1`, `snake_case` JSON, cursor pagination.
- Commits: Conventional Commits (`feat:`, `fix:`, `chore:`, …).
- Keep dependencies on their latest stable versions. Note in the PR any package pinned below latest, and why.
