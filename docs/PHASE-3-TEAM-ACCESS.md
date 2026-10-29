# Phase 3 plan: team access

Status: **Draft for review.** Nothing built yet. Covers how people join a workspace, what they can do once inside, how access is taken away, how new chats are routed to them, and what is recorded each time, as designed in the ChatPulse prototype ("Team", "Team Access Flow", "Invite teammates", "Accept invite", "Team Member Access", "Roles and permissions", "Assignment Rules", "Access Log"). Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md).

This is the second half of Phase 3 ("Workspaces, members, invites, roles"); [PHASE-3-ONBOARDING.md](PHASE-3-ONBOARDING.md) covered the first. The frontend pages are Phase 4; they are described here so the API matches them. The routing engine that actually hands chats to agents needs conversations, which arrive with the Inbox (Phase 5); this phase stores the rules and ships the rule tester.

Decisions marked **(proposed)** depend on an open question in section 10.

---

## 1. Decisions

| Topic                   | Decision                                                                                                                                                                                           |
| ----------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Authorization model     | **RBAC with per-member overrides and resource scoping.** Not a general ABAC policy engine: every rule in the design is "has permission X" or "is this chat inside their teams, numbers and scope". |
| Effective access        | `(role permissions ∪ granted) ∖ denied`, limited to the member's teams, WhatsApp numbers and chat scope. A suspended member has none.                                                              |
| Permission catalog      | **25 permissions in 8 groups**, from the Roles editor. The Invite wizard's 9 and the Team tab's 8 rows are summaries of these, not separate lists.                                                 |
| Roles                   | Four system roles (Owner, Admin, Agent, Viewer) copied into **every workspace**, so editing Agent in one workspace never changes another. Plus custom roles. Owner is locked.                      |
| Where reach lives       | **Chat scope on the role**, overridable per member. **Teams and numbers on the member.** **(proposed)**                                                                                            |
| Overrides               | Per member, `grant` or `deny`, optional end date and reason. The Invite wizard's "customize for this invite" becomes overrides on accept.                                                          |
| Owners                  | **Exactly one Owner.** Ownership moves only by transfer (Owner picks an Admin, confirms with password; old Owner becomes Admin). **(proposed)**                                                    |
| Seats                   | Owner, Admin, Agent and custom roles use a seat; Viewer is free. **Pending invites hold a seat.** Checked under a row lock on the workspace. **(proposed)**                                        |
| No privilege escalation | You can only give (by role edit, override or invite) permissions you hold yourself. Nobody changes their own role, overrides or status.                                                            |
| Removal                 | Deletes the membership row. History stays in the access log and in chat history. "Re-invite" creates a new invitation.                                                                             |
| When changes apply      | **On the next request.** Every request checks the member's current versions (section 5), so there is no stale window and no need to sign out.                                                      |
| Undo (10 minutes)       | The client sends the inverse change. No undo table.                                                                                                                                                |
| Invite links            | Single use, bound to the invited email, 24 h / 7 days (default) / 30 days. The token is stored hashed.                                                                                             |
| Invitee sign-up         | Signing up from a valid invite link **skips email verification** (the link proves the address) and skips onboarding.                                                                               |
| Access log              | Written in the **same transaction** as every change. Append-only (trigger blocks UPDATE/DELETE), monthly partitions, kept 12 months, CSV export.                                                   |
| Assignment rules        | Stored and editable now, with a tester. Executed by the Inbox in Phase 5.                                                                                                                          |
| Access check            | **One SQL query per request, no cache** (section 5, [ADR 0002](adr/0002-workspace-access-check.md)). Measured fastest of three designs on a 0.5 vCPU / 1 GB API.                                   |

---

## 2. The flow

The prototype's "Team access: full flow" in four lanes.

### A. Add people (Owner or Admin, needs `team.invite`)

| #   | Screen          | Route (Phase 4)                         | User does                                                                                                      | Server effect                                                                                    |
| --- | --------------- | --------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------ |
| 1   | People          | `_app/w/$workspaceId/team/invite` (1/4) | Emails (typed, pasted or CSV). Live seat meter.                                                                | `GET …/seats`                                                                                    |
| 2   | Role            | `…/team/invite` (2/4)                   | Picks one role for everyone in this invite; optionally switches permissions on or off for these people only.   | –                                                                                                |
| 3   | Access          | `…/team/invite` (3/4)                   | Teams, WhatsApp numbers (at least one), which chats they can open.                                             | –                                                                                                |
| 4   | Review and send | `…/team/invite` (4/4)                   | Personal message, link expiry, "require two-factor" (question 6). Send.                                        | `POST …/invitations` → one invitation per email, emails sent, log entries.                       |
| 5   | Accept          | `/invite` (public)                      | Opens the email link, sees workspace, inviter, role and what they'll be able to do. Google or password. Joins. | `POST /invitations/accept` → membership, teams, numbers, overrides; inviter notified; log entry. |
| 6   | Active          | `_app/w/$workspaceId/inbox`             | Quick start: set status Online, try a quick reply, turn on notifications.                                      | Assignment rules can now pick them.                                                              |

Invite page states: **valid**, **expired** ("Request a new invite" notifies the inviter), **revoked** ("no longer active"), **wrong account** (signed in as another email: log out or switch).

### B. Give access (takes effect on the next request)

| Action                     | Where                          | Needs        |
| -------------------------- | ------------------------------ | ------------ |
| Change role (up)           | Team › member › Role           | `team.roles` |
| Add a permission to a role | Team › Roles & permissions     | `team.roles` |
| Grant one extra permission | Team › member › Extra perms    | `team.roles` |
| Add to a team or number    | Team › member, or Team › Teams | `team.roles` |

### C. Take access away (always confirmed first)

| Action                          | Needs         | Impact shown before confirming                                                              |
| ------------------------------- | ------------- | ------------------------------------------------------------------------------------------- |
| Revoke an extra permission      | `team.roles`  | –                                                                                           |
| Remove a permission from a role | `team.roles`  | Everyone with the role, their gains and losses; optional email to each.                     |
| Downgrade role                  | `team.roles`  | Gains, losses, seat change.                                                                 |
| Suspend member                  | `team.remove` | Blocks access at once; seat stays reserved; open chats go to their team queue (Phase 5).    |
| Remove member                   | `team.remove` | Type their name. Choose who gets their open chats and chatbot drafts (Phase 5). Seat freed. |
| Revoke pending invite           | `team.invite` | Link stops working at once; seat freed.                                                     |
| Temporary permission expires    | (system)      | Member and granter emailed 24 h before.                                                     |

### D. After every change in B or C

1. Takes effect on the member's next request (section 5).
2. Access log entry: who, what, for whom, before and after, IP and device.
3. Email to the affected person ("Nadia changed your role to Admin"; removals say who to contact).
4. A toast with Undo for 10 minutes (client sends the inverse; removals are undone by re-inviting).

### Safety rules (enforced in the service, not the UI)

| Rule                          | How                                                                                                    |
| ----------------------------- | ------------------------------------------------------------------------------------------------------ |
| Owner can do everything       | Owner's effective permissions are **all bits**, computed in code, so a new permission is never missed. |
| Admins can't touch the Owner  | Any change whose target is the Owner is refused unless the actor is the Owner.                         |
| Nobody edits their own access | Actor = target → `403 cannot_change_self` for role, overrides, teams, numbers, status.                 |
| No escalation                 | Every permission being given must be in the actor's own effective set → `403 cannot_grant`.            |
| Owner role is never assigned  | Only `POST …/ownership/transfer` sets it.                                                              |
| Always one Owner              | Owner can't be suspended, removed or downgraded; transfer is atomic.                                   |
| Removals need the name typed  | `confirm_name` must match the member's display name (case-insensitive).                                |
| Viewers can't receive chats   | Assignment rules can't target a Viewer; validated on save.                                             |

---

## 3. Permissions and roles

### Catalog

Defined in code (`shared/authctx/permissions.go`) and mirrored into the `permissions` table by migration, so foreign keys hold. **A bit is never reused:** a retired permission keeps its bit forever.

| Bit | Key                     | Group        | Label                             | Sensitive |
| --- | ----------------------- | ------------ | --------------------------------- | --------- |
| 0   | `inbox.reply`           | Inbox        | Reply to customers                |           |
| 1   | `inbox.send_template`   | Inbox        | Send templates outside 24h window |           |
| 2   | `inbox.close_reassign`  | Inbox        | Close and reassign chats          |           |
| 3   | `inbox.notes`           | Inbox        | Add private notes                 |           |
| 4   | `contacts.view`         | Contacts     | View contacts                     |           |
| 5   | `contacts.edit`         | Contacts     | Edit contacts                     |           |
| 6   | `contacts.import`       | Contacts     | Import contacts                   |           |
| 7   | `contacts.export`       | Contacts     | Export contacts                   | yes       |
| 8   | `contacts.delete`       | Contacts     | Delete contacts                   | yes       |
| 9   | `broadcasts.create`     | Broadcasts   | Create broadcasts                 |           |
| 10  | `broadcasts.send`       | Broadcasts   | Send broadcasts                   |           |
| 11  | `broadcasts.reports`    | Broadcasts   | View reports                      |           |
| 12  | `templates.create`      | Templates    | Create templates                  |           |
| 13  | `templates.submit`      | Templates    | Submit to Meta                    |           |
| 14  | `templates.delete`      | Templates    | Delete templates                  |           |
| 15  | `automations.edit`      | Chatbots     | Edit flows and automations        |           |
| 16  | `automations.publish`   | Chatbots     | Publish changes                   |           |
| 17  | `integrations.manage`   | Integrations | Manage integrations               |           |
| 18  | `integrations.api_keys` | Integrations | View and create API keys          | yes       |
| 19  | `team.invite`           | Team         | Invite members                    |           |
| 20  | `team.roles`            | Team         | Change roles and access           | yes       |
| 21  | `team.remove`           | Team         | Remove and suspend members        |           |
| 22  | `billing.view`          | Billing      | View billing                      |           |
| 23  | `billing.pay`           | Billing      | Pay and change plan               |           |
| 24  | `billing.topup`         | Billing      | Top up wallet                     |           |

Reading chats isn't a permission: anyone can read the chats inside their scope. Replying needs `inbox.reply`.

### System role defaults

| Role   | Seat | Chat scope | Permissions                                                       |
| ------ | ---- | ---------- | ----------------------------------------------------------------- |
| Owner  | yes  | all        | All (computed, locked)                                            |
| Admin  | yes  | all        | All except `billing.pay` **(proposed)**                           |
| Agent  | yes  | team       | `inbox.*`, `contacts.view`, `contacts.edit`, `broadcasts.reports` |
| Viewer | no   | team       | `contacts.view`, `broadcasts.reports` **(proposed)**              |

Seeded for every new workspace in the create-workspace transaction, and backfilled for existing ones by migration. "Reset to default" on a system role restores this table.

---

## 4. Database

20 tables in the diagram; 17 are new. All tenant tables carry `workspace_id`. IDs are UUIDv7; times are `timestamptz` UTC; every table has `created_at`/`updated_at` with the existing `set_updated_at` trigger unless noted.

### Owned by `features/team`

**`permissions`** (global reference data, no `workspace_id`)

| Column      | Type       | Notes                      |
| ----------- | ---------- | -------------------------- |
| `key`       | `text` PK  | `contacts.export`          |
| `bit`       | `smallint` | UNIQUE, 0–63, never reused |
| `group_key` | `text`     |                            |
| `label`     | `text`     |                            |
| `sensitive` | `boolean`  |                            |
| `position`  | `int`      | Order in the editor        |

**`roles`**

| Column         | Type      | Notes                                                                       |
| -------------- | --------- | --------------------------------------------------------------------------- |
| `id`           | `uuid` PK |                                                                             |
| `workspace_id` | `uuid`    | FK `workspaces` ON DELETE CASCADE                                           |
| `system_key`   | `text`    | `owner`/`admin`/`agent`/`viewer`, NULL for custom                           |
| `name`         | `text`    | UNIQUE per workspace on `lower(name)`                                       |
| `description`  | `text`    |                                                                             |
| `color`        | `text`    | `green`/`blue`/`amber`/`red`/`gray`                                         |
| `chat_scope`   | `text`    | `assigned`/`team`/`all`                                                     |
| `uses_seat`    | `boolean` | false only for Viewer                                                       |
| `locked`       | `boolean` | true only for Owner                                                         |
| `version`      | `int`     | Bumped in the same transaction as any change to the role or its permissions |
| `created_by`   | `uuid`    | FK `users` ON DELETE SET NULL                                               |

UNIQUE `(workspace_id, system_key) WHERE system_key IS NOT NULL`.

**`role_permissions`**: `workspace_id`, `role_id` (FK CASCADE), `permission_key` (FK `permissions`). PK `(role_id, permission_key)`.

**`workspace_members`** (exists; changed)

| Column                                           | Change                                                                                             |
| ------------------------------------------------ | -------------------------------------------------------------------------------------------------- |
| `role`                                           | **Dropped**, replaced by `role_id uuid NOT NULL` FK `roles` (backfilled to the Owner role)         |
| `status`                                         | New. `active`/`suspended`                                                                          |
| `chat_scope`                                     | New. NULL = use the role's                                                                         |
| `availability`                                   | New. `online`/`away`/`offline`, default `offline`                                                  |
| `access_version`                                 | New. `int`, bumped on any change to this member's role, overrides, teams, numbers, scope or status |
| `invited_by`                                     | New. FK `users` ON DELETE SET NULL                                                                 |
| `joined_at`                                      | New                                                                                                |
| `suspended_at`, `suspended_by`, `suspend_reason` | New                                                                                                |

Index `(workspace_id, role_id)`. A partial unique index enforces one Owner: `(workspace_id) WHERE role_id = <owner role>` can't be expressed directly, so the service checks it under the workspace row lock and a deferred constraint trigger re-checks at commit.

**`member_permission_overrides`**

| Column                     | Type          | Notes                                         |
| -------------------------- | ------------- | --------------------------------------------- |
| `id`                       | `uuid` PK     |                                               |
| `workspace_id`, `user_id`  | `uuid`        | FK `workspace_members` ON DELETE CASCADE      |
| `permission_key`           | `text`        | FK `permissions`                              |
| `effect`                   | `text`        | `grant`/`deny`                                |
| `reason`                   | `text`        | Shown in the log                              |
| `granted_by`               | `uuid`        |                                               |
| `expires_at`               | `timestamptz` | NULL = no expiry                              |
| `expiry_notified_at`       | `timestamptz` | Set by the 24 h warning job                   |
| `revoked_at`, `revoked_by` | –             | Revoked rows stay for the member page history |

Indexes: `(workspace_id, user_id) WHERE revoked_at IS NULL`; `(expires_at) WHERE revoked_at IS NULL AND expires_at IS NOT NULL`. UNIQUE `(workspace_id, user_id, permission_key) WHERE revoked_at IS NULL`.

**`member_numbers`**: `workspace_id`, `user_id`, `number_id` (FK `whatsapp_numbers` CASCADE). PK `(workspace_id, user_id, number_id)`.

**`teams`**: `id`, `workspace_id`, `name` (UNIQUE per workspace), `description`, `color`, `number_id` (FK, SET NULL), `routing_method` (`round_robin`/`least_busy`/`manual`), `round_robin_cursor` (user id, nullable).

**`team_members`**: `workspace_id`, `team_id`, `user_id`. PK `(team_id, user_id)`; index `(workspace_id, user_id)`.

**`invitations`**

| Column                            | Type          | Notes                                                               |
| --------------------------------- | ------------- | ------------------------------------------------------------------- |
| `id`                              | `uuid` PK     |                                                                     |
| `workspace_id`                    | `uuid`        |                                                                     |
| `email`                           | `citext`      |                                                                     |
| `role_id`                         | `uuid`        | FK `roles` (RESTRICT; deleting a role moves its invites)            |
| `chat_scope`                      | `text`        | NULL = role's                                                       |
| `token_hash`                      | `bytea`       | UNIQUE. **Secret** (section 9)                                      |
| `personal_message`                | `text`        | ≤ 500 chars                                                         |
| `require_2fa`                     | `boolean`     | Question 6                                                          |
| `status`                          | `text`        | `pending`/`accepted`/`revoked`. Expired = pending past `expires_at` |
| `invited_by`                      | `uuid`        |                                                                     |
| `send_count`, `last_sent_at`      | –             | Resend limit                                                        |
| `expires_at`                      | `timestamptz` |                                                                     |
| `accepted_at`, `accepted_user_id` | –             |                                                                     |
| `revoked_at`, `revoked_by`        | –             |                                                                     |
| `renewal_requested_at`            | `timestamptz` | "Request a new invite"                                              |

UNIQUE `(workspace_id, email) WHERE status = 'pending'`; index `(workspace_id, status)`.

**`invitation_teams`**, **`invitation_numbers`**: join tables, PK `(invitation_id, team_id|number_id)`.

**`invitation_permission_overrides`**: `invitation_id`, `permission_key`, `effect`. PK `(invitation_id, permission_key)`.

**`access_log_events`** (append-only, partitioned)

| Column                        | Type          | Notes                                                                                                                                                                                                                                                                           |
| ----------------------------- | ------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `id`                          | `uuid`        | UUIDv7. PK is `(id, created_at)` (partitioning requires the key in the PK)                                                                                                                                                                                                      |
| `created_at`                  | `timestamptz` | Partition key, `PARTITION BY RANGE`, one partition per month                                                                                                                                                                                                                    |
| `workspace_id`                | `uuid`        | FK `workspaces` CASCADE                                                                                                                                                                                                                                                         |
| `actor_user_id`               | `uuid`        | NULL = System. **No FK**: the log outlives users                                                                                                                                                                                                                                |
| `subject_user_id`             | `uuid`        | NULL for a pending invitee. No FK                                                                                                                                                                                                                                               |
| `subject_email`               | `citext`      | Kept so entries read correctly after removal                                                                                                                                                                                                                                    |
| `action`                      | `text`        | `role_changed`, `permission_granted`, `permission_revoked`, `permission_expired`, `invited`, `invite_revoked`, `invite_accepted`, `removed`, `suspended`, `restored`, `team_changed`, `number_changed`, `role_created`, `role_updated`, `role_deleted`, `ownership_transferred` |
| `target`                      | `text`        | Permission key, role id, team id…                                                                                                                                                                                                                                               |
| `before_value`, `after_value` | `jsonb`       | Small; never contains secrets                                                                                                                                                                                                                                                   |
| `note`                        | `text`        | Reason, reassignment summary                                                                                                                                                                                                                                                    |
| `ip`                          | `inet`        |                                                                                                                                                                                                                                                                                 |
| `user_agent`                  | `text`        |                                                                                                                                                                                                                                                                                 |

No `updated_at`. A trigger raises on UPDATE and DELETE (dropping a partition is the only way rows leave). Indexes on the parent: `(workspace_id, created_at DESC)`, `(workspace_id, subject_user_id, created_at DESC)`. A `DEFAULT` partition catches anything outside the created months.

### Owned by `features/assignment`

**`assignment_settings`**: `workspace_id` PK, `timezone`, `default_max_open_chats` (8), `only_online` (true), `reassign_on_no_reply` (true), `no_reply_minutes` (15). Seeded with the workspace.

**`working_hours`**: `workspace_id`, `weekday` (0–6), `opens_at`, `closes_at` (`time`). PK `(workspace_id, weekday)`. A missing weekday = closed. Default Mon–Sat 09:00–19:00.

**`assignment_rules`**

| Column                                                                                                          | Notes                                                                              |
| --------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------- |
| `id`, `workspace_id`, `name`                                                                                    |                                                                                    |
| `position`                                                                                                      | Priority, unique per workspace (deferrable, so reordering is one statement)        |
| `enabled`                                                                                                       |                                                                                    |
| `is_fallback`                                                                                                   | UNIQUE `(workspace_id) WHERE is_fallback`; always last, can't be deleted or paused |
| `match_mode`                                                                                                    | `all`/`any`                                                                        |
| `target_type`                                                                                                   | `team`/`agent`                                                                     |
| `target_team_id`                                                                                                | FK `teams` (RESTRICT)                                                              |
| `target_user_id`                                                                                                | FK `workspace_members` (CASCADE → rule disabled by the service first)              |
| `method`                                                                                                        | `round_robin`/`least_busy`/`manual` (team targets)                                 |
| `only_online`, `max_open_chats`                                                                                 |                                                                                    |
| `no_reply_enabled`, `no_reply_minutes`, `no_reply_action` (`next_agent`/`escalate`/`queue`), `escalate_user_id` |                                                                                    |
| `after_hours_action`                                                                                            | `away_message`/`assign`/`chatbot`                                                  |
| `created_by`                                                                                                    |                                                                                    |

CHECK: exactly one of `target_team_id`/`target_user_id` matches `target_type`. Index `(workspace_id, position) WHERE enabled`.

**`assignment_rule_conditions`**: `id`, `workspace_id`, `rule_id` (CASCADE), `field` (`label`/`keyword`/`number`/`tag`/`language`/`source`/`hours`), `match_values text[]` (1–20 values), `position`.

### Changes to `workspaces`

`seat_limit int NOT NULL DEFAULT 5`. Set by the plan in Phase 6.

### Migrations

| File                    | Contents                                                                                                                                                                                                        |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `00010_team_access.sql` | `permissions` (+ rows), `roles`, `role_permissions`, `workspace_members` changes, overrides, numbers, teams, invitations. Backfill: system roles for every workspace, `role_id` = Owner role, then drop `role`. |
| `00011_access_log.sql`  | Partitioned `access_log_events`, this month and the next two, `DEFAULT` partition, append-only trigger.                                                                                                         |
| `00012_assignment.sql`  | Assignment tables; settings, hours and a fallback rule (→ first team or queue) for every workspace.                                                                                                             |

`seat_limit` goes in `00010`.

---

## 5. Authorization at runtime

### Per request

`platform/middleware/workspace.go` keeps its job (404 for non-members) but asks an `AccessResolver` for an `authctx.Access` instead of a role string:

**One query, no cache** ([ADR 0002](adr/0002-workspace-access-check.md)). For `(workspace_id, user_id)` it returns, in one round trip:

- the member's `status`, whether the role is the Owner's, and the effective chat scope (`coalesce(member.chat_scope, role.chat_scope)`);
- the role's permissions as a bitmask (`bit_or(1 << permissions.bit)` over `role_permissions`);
- grant and deny bitmasks from overrides that are not revoked and not past `expires_at`;
- the member's team IDs and WhatsApp number IDs;
- `roles.version` and `access_version`.

No row → 404. `status = suspended` → `403 member_suspended`. Then `Access{Bits: (role | grant) &^ deny, Scope, Teams, Numbers, Owner}`; Owner → all bits.

Every part is an index lookup, so the query costs a few microseconds in Postgres, and an expired grant is never served because the query filters on `expires_at > now()`. The benchmark in ADR 0002 compared this with a versioned in-process cache and with storing access on the member row; this was fastest and simplest. `roles.version` and `access_version` stay for optimistic concurrency and for telling the web app to refresh, not as cache keys.

Editing a role touches only that role's rows, however many members it has.

### Route checks

Handlers declare what they need; the middleware stays generic:

```go
huma.Register(api, op, h.sendBroadcast) // op.Middlewares: RequireWorkspace, RequirePermission(authctx.PermBroadcastsSend)
```

`RequirePermission` returns `403 permission_denied` with the missing key, so the UI can say what's needed. Services that need more (no escalation, owner checks) read `authctx.AccessFrom(ctx)`.

### Data scoping

Lists are filtered in SQL, never row by row. Repository methods for scoped data take a `Scope` value:

```sql
WHERE workspace_id = $1
  AND number_id = ANY($2)
  AND CASE $3 WHEN 'all' THEN true
              WHEN 'team' THEN team_id = ANY($4)
              ELSE assignee_id = $5 END
```

The conversations table (Phase 5) gets indexes `(workspace_id, team_id, last_message_at DESC)` and `(workspace_id, assignee_id, last_message_at DESC)` to serve this with cursor pagination. Postgres row-level security is not used: it fights pooled connections and the `workspaceID` parameter rule already scopes every query.

### Telling the web app

Every workspace response carries `X-Access-Version: {role_version}.{access_version}`. When it changes, the web app refetches `GET …/me/access` and re-renders menus and buttons. A `403 permission_denied` also triggers a refetch. Push (SSE) can replace this later without API changes.

---

## 6. API endpoints

All under `/v1/workspaces/{id}` unless noted. "Member" = any active member. Otherwise the column names the permission.

### Access

| Method and path          | Needs  | What it does                                                                                                 |
| ------------------------ | ------ | ------------------------------------------------------------------------------------------------------------ |
| `GET /me/access`         | Member | Role (id, name, system key), permission keys, chat scope, team and number IDs, `access_version`, `is_owner`. |
| `PATCH /me/availability` | Member | `online`/`away`/`offline`.                                                                                   |
| `POST /me/leave`         | Member | Leave the workspace. Refused for the Owner (`409 owner_must_transfer`).                                      |
| `GET /v1/permissions`    | Auth   | The catalog: key, group, label, description, sensitive.                                                      |
| `GET /seats`             | Member | `limit`, `used` (members), `reserved` (pending invites), `available`.                                        |

### Roles

| Method and path                | Needs        | What it does                                                                                                                                                           |
| ------------------------------ | ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /roles`                   | Member       | All roles with permission keys, scope, seat flag, member count, `version`.                                                                                             |
| `POST /roles`                  | `team.roles` | `name`, `description`, `color`, `copy_from_role_id` or `permissions`, `chat_scope`. No escalation.                                                                     |
| `POST /roles/{roleId}/preview` | `team.roles` | Same body as PATCH. Returns `gains`, `losses`, affected members, `sensitive_gain`. Changes nothing.                                                                    |
| `PATCH /roles/{roleId}`        | `team.roles` | `expected_version` (else `409 role_changed`), any of name/description/color/permissions/chat_scope, `notify_members`. Owner role → `403 role_locked`. Bumps `version`. |
| `POST /roles/{roleId}/reset`   | `team.roles` | System roles only; restores section 3 defaults.                                                                                                                        |
| `DELETE /roles/{roleId}`       | `team.roles` | Custom roles only. `move_to_role_id` required; moves members and pending invites in one transaction.                                                                   |

### Members

| Method and path                            | Needs         | What it does                                                                                                                                             |
| ------------------------------------------ | ------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /members`                             | Member        | Cursor list. Filters: `role_id`, `team_id`, `status`, `q`. Includes pending invites when `include_pending=true` (needs `team.invite`).                   |
| `GET /members/{userId}`                    | Member        | Profile, role, status, teams, numbers, seat, effective permissions with source (`role` or `override`, expiry). Overrides and activity need `team.roles`. |
| `POST /members/{userId}/role/preview`      | `team.roles`  | Gains, losses, seat change.                                                                                                                              |
| `PUT /members/{userId}/role`               | `team.roles`  | `role_id`, `expected_access_version`. Seat checked when moving from a free role.                                                                         |
| `PATCH /members/{userId}/access`           | `team.roles`  | Any of `chat_scope` (or `null`), `team_ids`, `number_ids`.                                                                                               |
| `POST /members/{userId}/overrides`         | `team.roles`  | `permission_key`, `effect`, `expires_at?`, `reason?`. Schedules the expiry jobs.                                                                         |
| `DELETE /members/{userId}/overrides/{oid}` | `team.roles`  | Revokes (sets `revoked_at`).                                                                                                                             |
| `POST /members/{userId}/suspend`           | `team.remove` | `reason?`.                                                                                                                                               |
| `POST /members/{userId}/restore`           | `team.remove` |                                                                                                                                                          |
| `DELETE /members/{userId}`                 | `team.remove` | `confirm_name`, `reassign` (`queue`/`user`/`round_robin` + target; stored for Phase 5), `transfer_drafts_to?`. Frees the seat.                           |
| `POST /ownership/transfer`                 | Owner         | `to_user_id` (an Admin), `password`. Old Owner becomes Admin.                                                                                            |

### Teams

| Method and path               | Needs        | What it does                                                      |
| ----------------------------- | ------------ | ----------------------------------------------------------------- |
| `GET /teams`                  | Member       | Teams with members, number, routing method, open chats (Phase 5). |
| `POST /teams`                 | `team.roles` | `name`, `description`, `color`, `number_id`, `routing_method`.    |
| `PATCH /teams/{teamId}`       | `team.roles` |                                                                   |
| `PUT /teams/{teamId}/members` | `team.roles` | `user_ids`. Bumps each changed member's `access_version`.         |
| `DELETE /teams/{teamId}`      | `team.roles` | Refused while an assignment rule targets it (`409 team_in_use`).  |

### Invitations

| Method and path                        | Needs         | What it does                                                                                                                                                                                                                      |
| -------------------------------------- | ------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /invitations`                     | `team.invite` | Pending, expired and recently revoked.                                                                                                                                                                                            |
| `POST /invitations`                    | `team.invite` | `emails` (1–50), `role_id`, `team_ids`, `number_ids` (≥1), `chat_scope?`, `overrides?`, `message?`, `expires_in` (`24h`/`7d`/`30d`). Seat check, no escalation, no Owner role. Existing members → `409 already_member` per email. |
| `POST /invitations/{invId}/resend`     | `team.invite` | New token, new expiry. Max 5 sends per invitation.                                                                                                                                                                                |
| `DELETE /invitations/{invId}`          | `team.invite` | Revoke.                                                                                                                                                                                                                           |
| `POST /v1/invitations/preview`         | –             | `token`. Workspace, inviter, role, "you'll be able to" list, state (`valid`/`expired`/`revoked`/`accepted`). Same 404 for unknown tokens. Rate limited per IP.                                                                    |
| `POST /v1/invitations/accept`          | Auth          | `token`. Signed-in email must match (`403 invite_email_mismatch`). Creates membership, teams, numbers, overrides; marks accepted; notifies inviter.                                                                               |
| `POST /v1/invitations/request-renewal` | –             | `token` of an expired invite. Emails the inviter once.                                                                                                                                                                            |
| `POST /v1/auth/signup` (changed)       | –             | Accepts `invite_token`; a matching email is marked verified and the invite accepted in the same transaction.                                                                                                                      |

The token travels in the URL fragment (`/invite#t=…`) and is sent in the POST body, so it never reaches server logs or `Referer`.

### Assignment

| Method and path                  | Needs        | What it does                                                                                                                                                                 |
| -------------------------------- | ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /assignment`                | Member       | Settings, working hours, rules with conditions, weekly hit counts (Phase 5).                                                                                                 |
| `PUT /assignment/settings`       | `team.roles` |                                                                                                                                                                              |
| `PUT /assignment/working-hours`  | `team.roles` | Full week + timezone.                                                                                                                                                        |
| `POST /assignment/rules`         | `team.roles` | Inserted above the fallback.                                                                                                                                                 |
| `PATCH /assignment/rules/{rid}`  | `team.roles` | Validates targets (no Viewers, no pending invitees).                                                                                                                         |
| `DELETE /assignment/rules/{rid}` | `team.roles` | Not the fallback.                                                                                                                                                            |
| `PUT /assignment/rules/order`    | `team.roles` | `rule_ids` in order, fallback excluded.                                                                                                                                      |
| `POST /assignment/test`          | Member       | `message`, `context` (tags, labels, number, language, source, hours), optional unsaved `rules`. Returns matched rule, skipped rules, target, note. Pure function, no writes. |

### Access log

| Method and path          | Needs        | What it does                                                                                                       |
| ------------------------ | ------------ | ------------------------------------------------------------------------------------------------------------------ |
| `GET /access-log`        | `team.roles` | Cursor list (by `id`, newest first). Filters: `action`, `member`, `q`, `from`, `to` (≤ 12 months). Summary counts. |
| `GET /access-log/export` | `team.roles` | Streams CSV for the same filters, up to 12 months.                                                                 |

Errors this document adds: `permission_denied`, `member_suspended`, `cannot_change_self`, `cannot_grant`, `role_locked`, `role_changed`, `role_in_use`, `owner_must_transfer`, `not_enough_seats`, `already_member`, `invite_not_found`, `invite_expired`, `invite_revoked`, `invite_email_mismatch`, `team_in_use`, `invalid_assignment_target`, `confirm_name_mismatch`.

---

## 7. Code layout

```
apps/api/internal/
├── features/team/              # roles, members, overrides, teams, invitations, access log
│   ├── handler.go  service.go  repository.go  model.go  dto.go  errors.go  routes.go  jobs.go
│   ├── access.go               # AccessResolver: one query → authctx.Access
│   ├── audit.go                # writes access_log_events inside the caller's transaction
│   └── queries.sql
├── features/assignment/        # settings, hours, rules, conditions, tester
│   ├── handler.go  service.go  repository.go  model.go  dto.go  errors.go  routes.go
│   ├── match.go                # pure rule matcher (tester now, Inbox later)
│   └── queries.sql
├── features/workspaces/        # loses workspace_members writes except through team.Seeder
├── platform/middleware/workspace.go  # AccessResolver instead of MembershipChecker; RequirePermission
└── shared/authctx/
    ├── workspace.go            # carries Access instead of a role string
    └── permissions.go          # permission keys, bits, Access.Has
```

No feature imports another. Interfaces, wired in `internal/app`:

| Declared in           | Interface          | Implemented by | Used for                                                                       |
| --------------------- | ------------------ | -------------- | ------------------------------------------------------------------------------ |
| `workspaces`          | `TeamSeeder`       | `team`         | System roles + Owner membership inside the create-workspace transaction        |
| `workspaces`          | `AssignmentSeeder` | `assignment`   | Settings, hours, fallback rule in the same transaction                         |
| `team`                | `AccountDirectory` | `auth`         | Look up users by email, display names, verify the Owner's password on transfer |
| `team`                | `NumberDirectory`  | `whatsapp`     | Check number IDs belong to the workspace; show their labels                    |
| `team`                | `AssignmentGuard`  | `assignment`   | Refuse deleting a team or removing a member that a rule targets                |
| `assignment`          | `MemberDirectory`  | `team`         | Validate rule targets (active, not Viewer)                                     |
| `auth`                | `InviteAcceptor`   | `team`         | Accept an invite inside the sign-up transaction                                |
| `platform/middleware` | `AccessResolver`   | `team`         | Section 5                                                                      |

`onboarding` and `whatsapp` move from role names to permissions (`Owner` routes become `team.roles` or `integrations.manage`).

### Factory check (CLAUDE.md section 2)

| Thing           | Factory? | Why                |
| --------------- | -------- | ------------------ |
| Access resolver | No       | One implementation |
| Rule matcher    | No       | Pure function      |
| Everything else | No       | Plain constructors |

---

## 8. Background jobs (River)

| Job                       | When                      | What                                                                                                                                               |
| ------------------------- | ------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------- |
| `send_email` (exists)     | On demand                 | New templates: invite, invite accepted (to inviter), renewal request, role changed, permission granted/revoked, suspended, removed, expiry warning |
| `override_expired`        | Scheduled at `expires_at` | Bumps `access_version`, logs `permission_expired`, emails. Correctness doesn't depend on it (section 5).                                           |
| `override_expiry_warning` | Hourly                    | Overrides expiring in < 24 h with `expiry_notified_at IS NULL`: email member and granter                                                           |
| `access_log_partitions`   | Daily                     | Ensure the next two monthly partitions exist; drop partitions older than 12 months                                                                 |
| `invitation_cleanup`      | Nightly                   | Delete revoked and expired invitations older than 30 days (their log entries stay)                                                                 |

---

## 9. Security and CLAUDE.md updates

- **Secrets rule gains:** `invitations.token_hash`, read only by `GetInvitationByTokenHash`. Tokens are 32 random bytes, sent once in the email, stored as SHA-256.
- **Rate limits:** invite preview and accept 20/min per IP; `POST /invitations` 100 emails/hour per workspace; resend 5 per invitation.
- **Enumeration:** preview returns the same 404 for unknown, malformed and accepted-by-someone-else tokens.
- **Audit integrity:** the log is written in the same transaction as the change, so a change without an entry can't commit.
- **Seat races:** `SELECT … FROM workspaces WHERE id = $1 FOR UPDATE` before counting seats in invite, accept, role change and restore.
- **No escalation** and **no self-change** are tested for every write endpoint (section 11).

---

## 10. Open questions

Each has a recommendation; the plan above assumes it.

1. **Where do numbers and chat scope live?** The Roles editor sets them per role; Invite and the member page set them per member. _Recommend:_ scope on the role with a per-member override; numbers only per member (a role is "what you can do", numbers are "where").
2. **What can Viewers read?** The Team tab says all chats; the Roles editor says their team's. _Recommend:_ team, like the editor; the Team tab copy changes.
3. **Admin and billing.** The Roles editor gives Admins view and top-up; the rules box says "everything except billing". _Recommend:_ the editor's defaults (no `billing.pay`); change the rules copy to "can't pay or change the plan".
4. **Do pending invites hold a seat?** Revoking "frees the seat", but "3 of 5 used" ignores Karim's pending invite. _Recommend:_ yes, they hold one; the counter shows "3 used · 1 reserved".
5. **One Owner or several?** "Always at least one Owner" vs transfer that demotes the old Owner. _Recommend:_ exactly one.
6. **"Require two-factor sign-in".** We have no 2FA. _Recommend:_ hide the toggle until 2FA ships; keep the column.
7. **Suspend and remove: sign out of all devices?** Sessions belong to the account, which may be in other workspaces. _Recommend:_ don't revoke account sessions; access to this workspace is cut on the next request anyway. Revoke API keys made in this workspace when those exist.
8. **Where is ownership transfer?** Settings › Security (Roles page) or the Owner's profile (member page). _Recommend:_ Settings › Security, linked from the Owner's profile.
9. **When does a change apply?** Flow page: within seconds; role dialog: "next time he loads a page". _Recommend:_ the next request, and fix the dialog copy.
10. **Access log permission.** The page says "Owners and Admins". _Recommend:_ gate it on `team.roles` (which Owners and Admins have by default), not on role names.
11. **Logins in the access log.** The design shows sign-ins and failed logins, but sessions belong to accounts, not workspaces. _Recommend:_ phase 1 shows access changes only; a later change copies auth events into every workspace the user belongs to.
12. **Custom-role-only permissions.** The member page shows "Issue refunds in Shopify" and "Manage return templates", which aren't in the catalog. _Recommend:_ leave them out until the Shopify integration and template categories exist.
13. **Invite expiry choice.** The wizard offers 24 h / 7 d / 30 d; other copy says links last 7 days. _Recommend:_ keep the choice, default 7 days, and say "Links expire on {date}".

---

## 11. Testing

- **Unit:** effective-access computation (role × grant × deny × suspended × owner); bit catalog is unique and never shrinks (test compares against a committed list); rule matcher against every example in the prototype tester; diff (gains/losses) for role edits.
- **Authorization matrix:** a table test that calls every endpoint as Owner, Admin, Agent, Viewer, a custom role, a suspended member and a non-member, and checks status codes. New endpoints fail the test until added to the table.
- **Escalation and self-change:** Admin can't grant `billing.pay`, edit the Owner, change their own role, or invite someone as Owner.
- **Repository** (real Postgres): every query; `workspace_id` scoping; the access log rejects UPDATE/DELETE; partitions are created and dropped; one fallback rule per workspace.
- **Concurrency:** two invites racing for the last seat; role edit with a stale `expected_version`.
- **Access:** a change applies on the next request; an expiring grant disappears exactly at `expires_at`; members of a custom role (NULL `system_key`) resolve correctly (this broke in the benchmark).
- **HTTP:** invite → preview → sign-up with token → accept, including expired, revoked, wrong-email and reused tokens.
- **End to end** (Playwright, Phase 4): invite an Agent, accept in a second browser, Admin removes a permission, the Agent's button disappears after the next request.

---

## 12. Build order

1. `shared/authctx` permissions and `Access`.
2. Migration `00010`; `team` roles, members, `AccessResolver`; middleware switches to permissions; `workspaces`, `onboarding`, `whatsapp` routes move to permission checks. Nothing else changes for current users (they're all Owners).
3. Access log (`00011`) and `audit.go`; every write from step 2 logs.
4. Overrides, suspend/restore, remove, ownership transfer, expiry jobs.
5. Teams and member numbers.
6. Invitations, invite emails, sign-up with `invite_token`.
7. Assignment (`00012`), rule tester.
8. Docs: ARCHITECTURE (features list, authorization section), CLAUDE.md secrets rule, ADR for the authorization model and permission bits, `.env.example`.

Suggested PRs: 1–3 (permissions in place, no visible change), 4–6 (team management), 7, 8.

---

## 13. Out of scope for this document

Executing assignment, reassigning open chats, open-chat counts and hit counts (Inbox, Phase 5); API keys (Integrations, Phase 5); seat limits by plan and buying seats (Phase 6); two-factor sign-in; login and security events in the access log (question 11); SSE push of access changes; the frontend pages (Phase 4).
