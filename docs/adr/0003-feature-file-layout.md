# 0003. Large features split into area files, not subfolders

- **Status:** accepted
- **Date:** 2026-10-29

## Context

Every feature had one file per kind: `service.go`, `handler.go`, `dto.go`, `model.go`, `queries.sql`. That worked for small features, but `auth` and `team` outgrew it. `auth/service.go` was 917 lines covering signup, login, sessions and passwords; `team/dto.go` was 627 lines and `team/queries.sql` held 62 queries. Finding the code for one flow (say, signup validation) meant scrolling through unrelated flows, and editor tabs and file search showed many identical names (`service.go`, `dto.go`).

We considered:

- **Subfolders per area** (`auth/oauth/`, `team/roles/`). In Go each folder is its own package, so an area could not use the feature's unexported repository, errors and helpers without exporting them, and areas that call each other would hit import cycles.
- **Feature-prefixed names** (`auth.dto.go`). The folder already names the feature, and a big feature would still have one huge `auth.service.go`.
- **Area-prefixed names in one package** (`signup_service.go`, `signup_dto.go`).

## Decision

A feature stays one Go package with no subfolders. When it has two or more distinct areas, or a file passes ~400 lines, its files are named `<area>_<kind>.go` (`_service`, `_handler`, `_dto`, `_model`, `_test`) and its queries `<area>_queries.sql`. A more specific word is fine when an area needs two files of a kind (`invitations_accept.go`, `oauth_google.go`, `log_writer.go`).

Code shared by the whole feature keeps the plain names: `service.go` (the `Service` struct, `New`, interfaces, small helpers), `handler.go`, `dto.go`, `model.go`, `routes.go`, `errors.go`, `repository.go`, `jobs.go`, `email.go`.

`sqlc.yaml` lists each feature folder, so sqlc reads every `.sql` file in it and new query files need no config change.

Applied now:

- `auth`: `signup`, `login`, `session`, `password`, `oauth`, `invite`, plus `account_queries.sql` and `cleanup_queries.sql` for queries several areas share.
- `team`: `access`, `roles`, `members`, `overrides`, `teams`, `invitations`, `log`.

The other features stay as they are.

## Consequences

- One area's files sort together, and searching for the area name finds its handler, service, DTOs, models and queries.
- No exported names changed, so `internal/app` and other packages are untouched. The move kept every line as is.
- sqlc writes one generated file per query file (`db/gen/roles_queries.sql.go`, …); the generated `Querier` interface and models are the same.
- Choosing an area is a judgement call. Add new code to the area that owns the flow; put something in the shared files only when several areas use it.
