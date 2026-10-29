# 0003. Features split into layer packages

- **Status:** accepted
- **Date:** 2026-10-29

## Context

Every feature was one Go package with one file per kind: `service.go`, `handler.go`, `dto.go`, `model.go`, `queries.sql`. That worked for small features, but `auth` and `team` outgrew it. `auth/service.go` was 917 lines covering signup, login, sessions and passwords; `team/dto.go` was 627 lines and `team/queries.sql` held 62 queries. Finding one flow's code meant scrolling through unrelated flows, and a flat folder of 40 files was hard to scan.

We considered:

- **Area files in one package** (`signup_service.go`, `roles_dto.go`). Idiomatic Go and no code changes, but every file of every layer still sits in one folder.
- **A package per area** (`team/roles/`, `team/invitations/`). Areas lean on each other heavily: signup, login, social sign-in and invites all start a session; accepting an invite adds a member, sets teams and creates overrides. That means many cross-package interfaces and a high risk of import cycles.
- **A package per layer** (`team/handler/`, `team/service/`, …). The layers already only call one way, so they split into packages without cycles.

## Decision

Each feature has one folder per layer, and each folder is its own Go package:

| Folder        | Holds                                                         | May import                       |
| ------------- | ------------------------------------------------------------- | -------------------------------- |
| `handler/`    | `routes.go`, HTTP handlers                                    | `service`, `dto`, `model`        |
| `service/`    | business rules, emails, external clients (OAuth, Graph)       | `repository`, `model`            |
| `repository/` | `repository.go` helpers and the feature's `*.sql` query files | `model`                          |
| `dto/`        | request/response shapes and converters                        | `model`                          |
| `model/`      | domain types, constants, errors, River job args               | no other layer                   |
| `jobs/`       | River workers                                                 | `service`, `repository`, `model` |

- Inside a layer, files are named by area once a feature has two or more areas (`roles_service.go`, `roles_queries.sql`). Shared code keeps the plain name (`service.go`, `dto.go`).
- A name is exported only when another layer uses it. Exported names get doc comments, except in `dto/`, where the type names and huma `doc` tags describe them (revive's `exported` rule is turned off for `dto/`).
- `internal/app` imports layer packages with the feature as prefix: `authservice`, `teamhandler`, `whatsappjobs`.
- sqlc reads every `.sql` file in each feature's `repository/` folder.
- No other folders inside a feature, and no folders inside a layer.

## Consequences

- Each layer is a short folder, and the import rules are checked by the compiler: a handler can't reach the repository without an import that review will spot.
- Names that cross layers are exported (about 270 renames when this was applied), so packages expose more than a single-package feature did. Treat exported names in `model/`, `repository/` and `dto/` as internal to the feature; other features still never import them.
- Every feature has packages called `service`, `handler` and so on, so imports outside a feature need aliases.
- Applying it fixed one bug: team's request-header struct was unexported, so huma skipped it and the access log never recorded the User-Agent.
- Not idiomatic Go for small packages. We accept that for one predictable layout across every feature.
