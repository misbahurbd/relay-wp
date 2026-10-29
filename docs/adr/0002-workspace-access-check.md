# 0002. Workspace access check: one query per request

- **Status:** accepted
- **Date:** 2026-10-29

## Context

Team access ([PHASE-3-TEAM-ACCESS.md](../PHASE-3-TEAM-ACCESS.md)) gives every workspace member a role, per-person grant and deny overrides (some with an end date), teams, WhatsApp numbers and a chat scope. Every workspace request needs that member's effective access, and a change must apply on the next request.

The API runs on 0.5 vCPU and 1 GB. We compared three designs with a benchmark before building:

- **A, normalized:** one SQL query per request combines role permissions, overrides, teams and numbers into a permission bitmask.
- **B, versioned cache:** a primary-key lookup of the role and member version numbers, then an in-process cache of the role bitmask and the member's overrides, teams and numbers.
- **C, denormalized:** the effective access is stored on `workspace_members` and recomputed by every write that affects it.

Benchmark setup: Postgres 18.6 (uncapped), Go API capped at `--cpus=0.5 --memory=1g`; 2,000 workspaces (10 of 2,300–5,000 members), 101k members, 2.03M conversations, 2.5M access-log rows, 50k overrides. Request mix: 75% inbox list scoped to the member's chats, 15% access check, 8% grant/revoke, 2% role edit. Two rounds in opposite order; averages below.

|                                         | A          | B                       | C                           |
| --------------------------------------- | ---------- | ----------------------- | --------------------------- |
| Requests/s, 50 users back to back       | **4,929**  | 4,661                   | 2,841                       |
| p95, 200 users back to back             | **63 ms**  | 89 ms                   | 102 ms                      |
| p95, 1,000 active users (≈1 req/s each) | **5.0 ms** | 4.5 ms in a clean round | 6.1 ms                      |
| Role edit p50, 3,200-member workspace   | 1.5 ms     | 1.6 ms                  | 46 ms (rewrites 3,199 rows) |
| Memory                                  | **79 MB**  | 115 MB                  | 80 MB                       |

## Decision

Use **A**. The middleware resolves access with one query per request and keeps nothing in memory between requests.

- Permissions have fixed bit positions (`permissions.bit`, 0–63, never reused); the query builds the bitmask with `bit_or`, and Go checks a permission with one bit test.
- Expired overrides are filtered in the query (`expires_at > now()`), so no job or cache is needed for correctness.
- `roles.version` and `workspace_members.access_version` are kept for optimistic concurrency and to tell the web app to refresh, not as cache keys.

## Consequences

- A cache (B) would not remove the round trip: a hit still needs the version lookup, and invalidation by every permission change held the hit rate to 87–95%. It cost 45% more memory for about 0.1 ms less database CPU per request. Revisit only if the database moves several milliseconds away and the access query becomes expensive.
- Storing access on member rows (C) makes role edits scale with workspace size, holds row locks, creates dead rows, and depends on every write path remembering to recompute.
- At 1,000 active users plan A used 0.09 cores (18% of the 0.5 vCPU). The bench API skipped JWT checks, validation and logging, so the real API has roughly 2–3× less headroom than measured.
- The largest database cost in the benchmark was the inbox list for team-scoped members (`team_id = ANY(...)`, 9.5 ms in a large workspace). Querying each team separately and merging took 0.3 ms. The inbox (Phase 5) should use that shape.
