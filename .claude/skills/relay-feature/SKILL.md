---
name: relay-feature
description: Relay-specific workflow for building or improving a feature in this repo (Go API + TanStack web + staff CRM). Use AUTOMATICALLY in relay-wp whenever the user asks to build, add, implement, improve, extend, refactor or redesign a feature, endpoint, page, job, integration, plan limit or flow ("add X", "improve the inbox", "make contacts support Y"). Takes precedence over the generic feature-design skill in this repo. Sizes the work, runs system-design / ddia-systems / c4-architecture / bmad-architecture only as much as needed, writes design output where this repo keeps it (PHASE docs, ADRs), builds layer by layer under CLAUDE.md rules, and runs the done checklist. Not for one-line fixes, copy changes, env edits or questions.
---

# Relay feature: design, build, verify

This is the generic `feature-design` workflow bound to relay-wp. CLAUDE.md is
the law; this skill is the order of work. When the two seem to disagree,
CLAUDE.md wins. If a task seems to need breaking a CLAUDE.md rule, stop and ask.

Helper skills (load with the Skill tool only when a step calls for them):

| Skill               | Use it here for                                                               |
| ------------------- | ----------------------------------------------------------------------------- |
| `system-design`     | Requirements, volume estimates, sync vs River job, SSE, rate limits, failure  |
| `ddia-systems`      | Tables, indexes, locks, idempotency keys, transactional enqueue, migrations   |
| `c4-architecture`   | One Mermaid component diagram + a sequence diagram for async/multi-step flows |
| `bmad-architecture` | The invariants other PRs must keep, written as an ADR (not a separate doc)    |

## Step 0: Read before designing

1. Read `docs/ARCHITECTURE.md` once per session, and the ADRs the feature
   touches (`docs/adr/`). The ones most features hit:
   - 0003 / 0004 feature file layout (API / web)
   - 0005 workspace comes from `X-Workspace-ID`, never the URL
   - 0008 plan resource catalog (any limit, quota or paid gate)
   - 0009 realtime SSE (anything that must update live)
   - 0010 omnichannel inbox, 0011 API keys, 0012 automations
2. Read the matching `docs/PHASE-*.md` if the feature belongs to a phase.
3. Find the closest existing feature and copy its shape. Good references:
   - `features/templates`: CRUD + external API (Meta) + a big web feature with
     `components/{list,editor,preview,kit}`
   - `features/automations`: River jobs, run records, cross-feature adapters
   - `features/billing`: plan limits and `RequirePlan` (monthly
     `usage_counters` are planned in ADR 0008; check before assuming they exist)
4. Check whether what's asked already exists (grep the API feature, the web
   feature and `packages/api-client`). Reuse beats new.

## Step 1: Size it

Pick the tier from the most demanding signal:

| Tier  | Relay signals                                                                                                                                                    |
| ----- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **S** | One feature, no migration, no new route group, no job. A filter, a field on an existing DTO, a UI tweak.                                                         |
| **M** | A migration, a new route, API + web together, a CRM screen, a plan limit (ADR 0008), permission/role changes, or a change in an existing flow's behaviour.       |
| **L** | A new feature folder, a new external provider/channel, River jobs or scheduling, a new factory point, a new SSE event family, or something several PRs build on. |

Say it in one line ("M: new column + API + web, gated by a monthly limit") and
move on. Unsure: pick the higher tier.

## Step 2: Design by tier

### S

No helper skills. 3 to 5 bullets in the reply: files by layer, the change,
how you'll verify. Then build.

### M

Write a compact section in the reply (no file), then build without waiting
unless a decision is genuinely the user's (pricing, product behaviour,
something destructive):

1. **Requirements**: what it does, who sees it (role), what must not break,
   behaviour for existing rows.
2. **Data** (`ddia-systems`, only if schema/concurrency is involved):
   columns + `workspace_id`, indexes, the migration and any backfill, locks
   (count-under-row-lock for `count` limits), idempotency.
3. **Cross-feature needs**: if feature A needs B, name the small interface A
   declares in its own `model`/`service` and the adapter in
   `internal/app/<a>_adapters.go`.
4. **Change list by layer**: migration → `repository/*_queries.sql` →
   `model` → `service` → `dto` → `handler/routes.go` → `task gen` →
   web `api/` → `hooks/` → `components/` → route file → seed → tests.

### L

1. **Requirements + estimates** (`system-design`): order-of-magnitude volume
   (msgs/day, jobs/day, rows/workspace, provider rate limits). Design for
   Relay's real scale: one Postgres, River on Postgres, no Redis. Name what
   would need revisiting at 10×.
2. **High-level design** (`system-design`): sync vs River job, retries and
   dedupe, SSE events, provider failure handling, plan gates. Run the 8-row
   diagnostic; fix only rows that matter at this scale.
3. **Data** (`ddia-systems`): tables, ownership, consistency, idempotency
   keys, transactional enqueue (insert row + enqueue job in one tx),
   migration order.
4. **Diagram** (`c4-architecture`): one component diagram; a sequence
   diagram if async or multi-step.
5. **Decisions** (`bmad-architecture`): write the invariants as an ADR,
   `docs/adr/NNNN-<slug>.md` from `0000-template.md` (next free number,
   status `proposed`). A new factory needs this ADR **and** a row in the
   CLAUDE.md factory table; ask the user before adding either.
6. **Plan**: write `docs/PHASE-<n>-<SLUG>.md` with PR ids in order, each PR's
   scope, file boundaries (which PR may edit which feature), migration
   order, rollout/default-off behaviour and rollback. Match the style of
   the existing PHASE docs.

Then **stop** and show: tier, key decisions, open questions, PR list. Ask the
user to confirm before building. Skip the pause only if they said "just build it".

## Step 3: Build (Relay rules that bite most often)

API:
- Migration only via `task migrate:new -- <name>`. Every tenant table has
  `workspace_id`; every tenant repository method takes `workspaceID`.
- Queries list exact columns, including `RETURNING`. No `*`. Secret columns
  stay inside the one query CLAUDE.md names.
- Services return `shared/apperr`; no HTTP status outside `handler/` /
  `platform/httpx`. Services never import `net/http`, huma or chi.
- Workspace routes go under `/v1/workspace/...` with `RequireWorkspace` and
  `header:"X-Workspace-ID" required:"true"`. Staff tools: `/v1/admin` + `apps/crm`.
- No feature imports another feature. Interface in the consumer, adapter in
  `internal/app`, wiring in `internal/app/app.go`.
- Jobs: args in `model/job_args.go`, workers in `jobs/jobs.go`, registered in
  `internal/app/workers.go`.
- Plain `New(deps...)` constructors. No factory unless it's in the approved table.

Web / CRM:
- Never hand-write API types or fetches: change Go, run `task gen`, use
  `@relay/api-client`. Workspace calls pass `headers: inWorkspace(workspaceId)`.
- Import features only through `#/features/<name>`. Route files stay thin.
- No native `<button|input|select|textarea>` outside `components/ui`; use the
  primitives. Missing primitive → add it to `components/ui/` first.
- No default select box: never `NativeSelect`; dropdowns are `OptionSelect`
  (or `Select` + `SelectItem`) from `#/components/ui`.
- Query keys live in the feature's `api/<feature>.keys.ts`; mutations invalidate through them.
- `apps/web` and `apps/crm` never import each other.

Always:
- Sample data in `internal/app/seed_dev.go`, idempotent (second run adds nothing).
- Tests next to the code (`<layer>/*_test.go`) plus an end-to-end
  `internal/app/<feature>_flow_test.go` for M/L API work.
- Keep behaviour backward compatible by default. Don't commit or push unless asked.

## Step 4: Verify (CLAUDE.md section 3, not optional)

```bash
task lint
task typecheck
task test
task gen && git diff --exit-code
grep -rn 'workspaceId}' apps/api/internal --include='*.go'
grep -rn '/w/' apps/web/src --include='*.tsx'
grep -rn '\*' --include='*.sql' apps/api/internal
grep -rnE '<(button|input|select|textarea)([ >]|$)' apps/web/src --include='*.tsx' --exclude-dir=ui
grep -rnE 'NativeSelect|<select' apps/web/src apps/crm/src --include='*.tsx' --exclude-dir=ui
```

Every grep must find nothing new. If the feature has a seed or a migration,
run `task dev:seed` twice on a fresh DB. For UI changes, use the `run` skill
to see it working. Report failures as they are; say if one existed before
your change.

## Step 5: Report

Short and plain:
1. What was built, and verified or not (which checks passed).
2. What changes for the user / staff (behaviour, not file names).
3. Decisions they might want to change.
4. Anything still needed (env var, Meta/Stripe setup, migration on deploy).
5. For L: links to the PHASE doc and ADR.
