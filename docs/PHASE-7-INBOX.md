# Phase 7 plan: Team Inbox

Status: **Slice A built; slice B next.** Covers the "Team Inbox" and "Assignment rules builder" boards of the ChatPulse prototype. It follows [CLAUDE.md](../CLAUDE.md), [ARCHITECTURE.md](ARCHITECTURE.md) and the inbox parts of [PHASE-3-TEAM-ACCESS.md](PHASE-3-TEAM-ACCESS.md), which defined the scopes, the inbox permissions and the assignment tables. Real-time delivery is recorded in [ADR 0009](adr/0009-realtime-sse.md). The inbox is omnichannel ([ADR 0010](adr/0010-omnichannel-inbox.md)): WhatsApp is the first channel, and Messenger and Instagram follow. Where this document says "number" or "wamid", read "channel" and "external ID" (section 11).

The phase ships in two slices, each its own PR:

- **Slice A, the Inbox:** conversations, messages, replies, templates, notes, statuses, manual assignment and live updates.
- **Slice B, Assignment:** the rules engine, the builder, working hours and no-reply reassignment.

Slice A has everything an agent needs to talk to customers. Slice B decides who gets each chat automatically.

---

## 1. Requirements

### Functional, slice A

| #   | Requirement                                                                                                                                                                                                                                                                                                  |
| --- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| F1  | Every inbound WhatsApp message lands in a conversation for that contact and number, creating the contact (`source = inbox`) if needed. A webhook delivered twice is stored once.                                                                                                                             |
| F2  | Conversation list with tabs **Open**, **Pending**, **Closed**, **Assigned to me** (with counts), search by name, phone or message, and sort by **Newest** or **Waiting longest**. Each row shows the unread count, last message, time, a "Waiting Xm" badge (amber ≥ 30 min, red ≥ 60 min) and the assignee. |
| F3  | Thread renders inbound and outbound text, image, video, audio, document, sticker, location, contact card, reply quotes, reactions, interactive replies and templates, plus system events and private notes. Outbound messages show sent, delivered, read or failed.                                          |
| F4  | Reply with text or a file (image, video, document) while the 24-hour window is open; quote a message; react with an emoji. Outside the window, only an approved template can be sent (with its variables filled in), and that moves a closed chat to Pending.                                                |
| F5  | 24-hour window bar: time left, urgent under 25 %, closed, or "template sent, waiting for reply".                                                                                                                                                                                                             |
| F6  | Private notes with @mentions; a mentioned member is notified in the app.                                                                                                                                                                                                                                     |
| F7  | Assign to a member or unassign; snooze (1 hour, tomorrow 9:00, next Monday 9:00), which moves the chat to Pending until then; close; reopen. Close and snooze can be undone within 5 s. Each action writes a system event.                                                                                   |
| F8  | Quick replies: `/` menu and chips in the composer; members with `inbox.reply` manage them.                                                                                                                                                                                                                   |
| F9  | Opening a chat clears its unread count for everyone and marks the customer's last message read on WhatsApp (blue ticks).                                                                                                                                                                                     |
| F10 | Live updates: new messages, status ticks, assignment and status changes appear for every agent who can see the chat within 2 s, without reloading.                                                                                                                                                           |
| F11 | Right panel: contact, opt-in, assignee, tags (editable), custom fields and contact notes. These are the Contacts feature's data, shown in the inbox.                                                                                                                                                         |
| F12 | Webhook events `message.received` and `message.status` are published to Integrations.                                                                                                                                                                                                                        |

### Functional, slice B

| #   | Requirement                                                                                                                                                                                                                                                                                                                                                                                                                                                                               |
| --- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| F13 | Ordered rules, where the first match wins, plus an undeletable fallback. Conditions are label, keyword, WhatsApp number, contact tag, language, source and business hours, matched with AND or OR. The target is a team (round-robin, least busy or manual queue) or an agent. Rules also set "only online agents", "max open chats" (1–50), no-reply reassignment (5–120 min: next agent, escalate or back to queue) and an after-hours action (away message, assign anyway or chatbot). |
| F14 | Rules run when a new chat opens (a first message, or a message to a closed chat). Edits apply only to new chats.                                                                                                                                                                                                                                                                                                                                                                          |
| F15 | Rule tester (a pure function), weekly hit counts per rule, and working hours with a timezone.                                                                                                                                                                                                                                                                                                                                                                                             |

### Non-functional

- **Latency:** webhook acknowledged in < 200 ms. Inbound message on screen < 2 s at p95. Send click to "sent" tick < 3 s at p95 (bounded by Meta).
- **Correctness:**
  - Each message is stored exactly once (unique `wamid`).
  - Status ticks never go backwards (sent → delivered → read; failed is final).
  - Unread counts and the last message stay consistent with the messages.
- **Isolation:** every query is scoped by `workspace_id`, and by the member's chat scope (all, team or assigned) and numbers (PHASE-3 §5). Real-time events reach only members who could fetch the chat.
- **Availability:** same as the API, with no new single point of failure. A dropped live connection only delays updates, because the client refetches on reconnect.
- **Cost:** no new infrastructure. Postgres (including LISTEN/NOTIFY), River and S3 only; no Redis, no broker (ARCHITECTURE: 0.5 vCPU / 1 GB per instance).

### Out of scope (deferred, section 9)

- AI copilot (the InboxAI board).
- Customer typing indicator: the Cloud API doesn't send it.
- Conversation labels (no board sets them).
- Past orders (needs Commerce).
- Collision indicator, chatbot handoff, audio recording, and sending location, contact cards or catalog products.

## 2. Estimates

The assumptions match PHASE-5:

- 10,000 workspaces, of which 1,000 actively message;
- 500 messages a day each, half inbound;
- about 2 status webhooks per outbound message;
- 3 agents online per active workspace at peak.

| Quantity         | Estimate                                                                                                                                  |
| ---------------- | ----------------------------------------------------------------------------------------------------------------------------------------- |
| Messages         | 1,000 × 500 = 500 K/day ≈ **6/s average, ~30/s peak**                                                                                     |
| Webhook calls    | 250 K inbound + 500 K statuses ≈ 750 K/day ≈ **9/s average, ~45/s peak** (each is one River job)                                          |
| Sends to Meta    | 250 K/day ≈ 3/s average. Far below Meta's 80 messages/s per number                                                                        |
| Message rows     | ~600 B, plus ~400 B of indexes ≈ 1 KB → **0.5 GB/day, ~180 GB/year**                                                                      |
| Media            | ~10 % of messages × ~250 KB ≈ **12 GB/day** to S3. Bytes never go in Postgres                                                             |
| Live connections | 1,000 × 3 = **3,000 SSE streams at peak**. About 3,000 goroutines plus ~10 KB of buffers each ≈ 30–60 MB in total; split across instances |
| NOTIFY volume    | One per message, status or action ≈ 15/s average, 75/s peak; each payload < 200 B                                                         |
| List reads       | 3,000 agents refetching on events and opening chats ≈ **50–100 reads/s at peak**; each is an index range scan of ≤ 50 rows                |

**What follows from these numbers:**

- One Postgres primary handles this load.
- `messages` is the only table that grows fast. It gets a `(conversation_id, created_at)` access path and is ready for monthly partitioning (AD-4) when it passes ~100 GB.
- SSE fits the memory budget.
- NOTIFY is far below its practical ceiling: thousands per second, with an 8 KB payload limit.

## 3. High-level design

```text
 Meta ──POST /v1/webhooks/whatsapp──► whatsapp handler: verify HMAC → enqueue whatsapp_webhook (tx) → 200
                                                         │
                                       River worker: whatsapp.ProcessWebhook
                                         │ messages[] / statuses[] (parsed, workspace + number resolved)
                                         ▼
                                     InboundSink (interface, wired in internal/app)
                                         ▼
                 inbox.service: one tx ── find/create contact (ContactDirectory)
                                       ── upsert conversation, insert message (ON CONFLICT wamid DO NOTHING)
                                       ── media? enqueue inbox_fetch_media
                                       ── new chat? AssignmentRouter.Route (slice B; slice A: none)
                                       ── EventPublisher.Publish(message.received)       (Integrations outbox)
                                       ── pg_notify('inbox', {ws, conv, team, assignee, number, kind})
                                         ▼
                 realtime hub (one LISTEN connection per API instance) ──► SSE streams of members in scope
                                         ▼
                 web: EventSource /v1/workspace/inbox/events → invalidate TanStack queries → refetch

 Agent reply: POST …/conversations/{id}/messages ─► tx: insert message(status=pending) + River inbox_send + NOTIFY
                 inbox_send worker ─► Messenger.Send (whatsapp Graph) ─► wamid, status=sent | failed ─► NOTIFY
                 Meta status webhook ─► StatusSink ─► monotonic status update ─► NOTIFY
```

**Building blocks and why each one:**

- **River** (the queue) separates Meta from us in both directions. Inbound is already queued. Sends get retries and survive restarts.
- **Postgres LISTEN/NOTIFY** fans events out across API instances without a broker (ADR 0009).
- **SSE**, not WebSocket: the traffic is one-way (server to client), it is plain HTTP through the existing middleware and cookies, and the browser reconnects on its own.
- **S3** (platform/storage) holds media.
- **No cache layer:** the list is served from the denormalized fields on `conversations` plus composite indexes. That denormalization is the cache, and it stays consistent because it is written in the same transaction as the message.

## 4. Architecture decisions

### AD-1: Feature boundaries

- **Binds:** the backend build.
- **Prevents:** inbox code reaching into whatsapp, contacts or templates, and webhook parsing in two places.
- **Rule:**
  - New feature `features/inbox` owns conversations, messages, notes, quick replies and inbox actions.
  - Slice B adds `features/assignment`, which owns settings, working hours, rules and the engine.
  - `features/whatsapp` keeps the Meta boundary. It parses webhooks into plain types and gains send, read and media methods. It never stores chats.
  - The inbox declares the interfaces it needs, and `internal/app/inbox_adapters.go` is the only file that knows both sides:
    - `ContactDirectory`: `FindOrCreateByPhone`, `Get` (from contacts).
    - `Messenger`: `SendText`, `SendMedia`, `SendTemplate`, `SendReaction`, `MarkRead`, `FetchMedia`, `UploadMedia` (from whatsapp).
    - `TemplateCatalog`: approved templates with their components (from templates).
    - `EventPublisher` (from integrations).
    - `MemberDirectory`: names and scopes (from team).
    - `AssignmentRouter`: slice B; slice A passes a no-op.
  - whatsapp hands parsed events over through `SetInboundSink(InboundSink)`, the same pattern as `TemplateStatusSink`.

### AD-2: Conversation model, one thread per contact and number

- **Binds:** schema, list and assignment.
- **Prevents:** a new row for every Meta "conversation" (its 24 h billing window), and duplicate threads.
- **Rule:** `conversations` has `UNIQUE (workspace_id, contact_id, number_id)`. Status `open | pending | closed` and the assignee are mutable.
  - An inbound message to a **closed** chat reopens it (status open). Slice B re-routes it, and it counts as "new" for assignment.
  - An inbound message to a pending chat makes it open.
  - Snooze sets `pending` and `snoozed_until`. A periodic River job reopens chats whose snooze has expired.
- **Denormalized on the row** and updated in the same transaction as each message: `last_message_at`, `last_message_preview` (≤ 120 chars), `last_message_direction`, `last_inbound_at`, `window_expires_at` (= last inbound + 24 h), `unread_count`, `waiting_since` (the first unanswered inbound message; cleared by an outbound reply). The list never touches `messages`.

### AD-3: Ingest is idempotent and runs in one transaction

- **Binds:** webhook processing.
- **Prevents:** duplicates on Meta retries, and half-written messages.
- **Rule:**
  - `messages.wamid` is UNIQUE (nullable for notes, system events and unsent messages). Ingest is `INSERT … ON CONFLICT (wamid) DO NOTHING`. When nothing was inserted, the counters, NOTIFY and Publish are all skipped.
  - The contact, conversation, message, media job, Integrations outbox row and `pg_notify` run in one transaction. NOTIFY is delivered on commit, so listeners never see uncommitted rows.
  - This satisfies ARCHITECTURE rule 8: the River job args are the stored raw event, and `wamid` is the external ID.

### AD-4: Messages schema

- **Binds:** schema.
- **Prevents:** per-type tables and a content shape that drifts between the API and the web.
- **Rule:** one `messages` table with these columns:
  - `id` (uuidv7, time-ordered, which is also the cursor), `workspace_id`, `conversation_id`;
  - `direction` (`in | out`), `kind` (`message | note | event`), `type` (WhatsApp type, `text`, … `template`, `reaction`, `unsupported`);
  - `body` text (text, caption, or the rendered template), `content jsonb` (our own shape per type: media `{storage_key, mime, size, filename, sha256}`, location, contacts, interactive, template `{name, language, category, components, params}`, event `{action, actor_id, target_id, until}`);
  - `wamid`, `reply_to_id`, `status` (`pending | sent | delivered | read | failed`, outbound only), `error_code`, `error_title`;
  - `author_id` (member, for outbound and notes), `mentions uuid[]`, `created_at`, `status_at`.
- **Reactions** are stored on the target message in `reactions jsonb {in?: emoji, out?: emoji}`. A reaction message updates its target and isn't a row of its own, matching "one emoji per message, toggle off".
- **Index:** `(conversation_id, id DESC)` for the thread. Body search uses a trigram index on `conversations.last_message_preview` plus the contact's name and phone. Full-text search over all messages is deferred.
- **Partitioning:** `id` is uuidv7, so `RANGE (created_at)` monthly partitioning can be added later without changing the API.

### AD-5: Sending is an outbox through River

- **Binds:** replies, templates, reactions and read receipts.
- **Prevents:** an HTTP request blocked on Meta, and sends lost on a crash.
- **Rule:**
  - `POST …/messages` checks the permission, the window (AD-6), the opt-in and the plan state. In one transaction it inserts the message with `status = pending`, enqueues `inbox_send` (unique per message ID) and sends NOTIFY, then returns 201 with the message.
  - The worker calls `Messenger`. On success it stores the `wamid` and sets `sent`.
  - On a permanent Meta error it sets `failed` with Meta's code and title, and does not retry. Examples: 131047 (re-engagement, window closed), 131026 (undeliverable). Rate limits (130429, 131056) and temporary errors (131000, 131016) retry like transport errors.
  - Transport errors and 5xx retry with River's backoff, up to 5 attempts. Only then is the message failed.
  - Media to send is uploaded to S3 first (`POST …/media`, which returns a key). The worker uploads it to Meta (`/{phone_number_id}/media`) and sends it by media ID.
  - Read receipts (`MarkRead`) use their own job (`inbox_mark_read`), which is fire-and-forget and dropped on failure.

### AD-6: 24-hour window and template rules

- **Binds:** compose and send.
- **Prevents:** sends Meta will reject, and charges the user didn't expect.
- **Rule:**
  - Free-form send (text, media, reaction) needs `now < window_expires_at`. Otherwise it returns 409 `window_closed`.
  - Templates are allowed at any time with `inbox.send_template`. They must be approved in the conversation's language, or in any language if the user picks one. Every `{{n}}` variable must be filled. A closed chat moves to `pending`.
  - The client shows the window, but the server decides: if Meta rejects with 131047, the message is marked failed and NOTIFY tells the client to refresh the window.
  - The cost hints in the UI come from a static per-category price table in the web app. They are estimates, never billed by us.

### AD-7: Status updates only move forward

- **Binds:** the status webhook.
- **Prevents:** ticks moving backwards when Meta's webhooks arrive out of order.
- **Rule:** `UPDATE messages SET status = $new … WHERE wamid = $1 AND rank(status) < rank($new)`, with rank pending 0 < sent 1 < delivered 2 < read 3, and failed 4, which is final. A status for an unknown wamid (a test message, or one sent outside Relay) goes to whatsapp's existing test-message handling and is otherwise ignored. A `read` status needs no counter change, because unread counts only cover inbound messages.

### AD-8: Live updates are a signal followed by a refetch, over SSE, fanned out with LISTEN/NOTIFY (ADR 0009)

- **Binds:** real-time delivery, API and web.
- **Prevents:** chat data leaking through the event stream, cross-instance gaps, a broker dependency, and two sources of truth on the client.
- **Rule:**
  - Every write to the inbox calls `pg_notify('inbox_events', json)` in its transaction. The payload is `{ws, conversation_id, kind, team_id, assignee_id, number_id}` and **never content**. `kind` is one of `message`, `status`, `conversation`, `note`.
  - **Hub:** `platform/realtime` runs one goroutine per API instance. It `LISTEN`s on a dedicated connection from the direct pool (not the pooled URL, per ARCHITECTURE) and reconnects with backoff. It fans events out to subscribers by workspace, and each subscriber's filter applies the member's scope and numbers (PHASE-3 §5) to the payload's team, assignee and number.
  - **Endpoint:** `GET /v1/workspace/inbox/events` (session or cookie auth, `RequireWorkspace`) is SSE.
    - It clears the 30 s write deadline with `http.ResponseController`.
    - It sends a heartbeat comment every 20 s.
    - Each subscriber has a buffer of 64 events. A slow client is dropped, and the browser reconnects and resyncs.
    - The stream is refreshed every 15 min, so access changes take effect: the member's access is reloaded on reconnect.
  - **Web:** one `EventSource` per workspace tab. Events invalidate the matching TanStack queries: the list on any event; the thread and the conversation only when it's open.
    - On `open` or reconnect it invalidates everything.
    - While disconnected it falls back to polling every 15 s.
  - **No ordering guarantee is needed:** the refetch reads the truth.

### AD-9: Assignment (slice B)

- **Binds:** the routing engine and its tables.
- **Prevents:** two agents getting the same chat, and round-robin cursors drifting apart under concurrency.
- **Rule:**
  - The tables are as in PHASE-3 §4: `assignment_settings`, `working_hours`, `assignment_rules`, `assignment_rule_conditions`, plus `teams.round_robin_cursor`.
  - The engine runs inside the ingest transaction for new chats. Rules are evaluated in order; the first match wins; the fallback is last.
  - Candidates are active, non-Viewer team members (filtered to online ones if the rule says so) with fewer than `max_open_chats` open chats. The open count comes from `COUNT(*) … WHERE assignee_id = ANY($members) AND status = 'open'` on the assignee index.
  - Round-robin locks the team row (`SELECT … FOR UPDATE`) and advances the cursor. Least busy picks the fewest open chats, breaking ties by `last_assigned_at`. Manual queue leaves the chat unassigned with the team set.
  - Nobody eligible means the chat waits in the team queue (team set, assignee null).
  - **No-reply:** when a chat is assigned, a River job `assignment_no_reply` is scheduled at `+N min`. When it runs, it acts only if `waiting_since` is still before it was scheduled and the assignee is unchanged.
  - **After hours:** "away message" sends the workspace's away text (a free-form message, since the window is open after an inbound message) and keeps the chat in the queue. "Chatbot" is treated as the queue until Chatbots exist.

### AD-10: Permissions and scope

- **Binds:** API and web.
- **Prevents:** agents seeing or acting on chats outside their scope.
- **Rule:**
  - Reading needs only membership, filtered by scope and numbers in SQL (the PHASE-3 §5 shape, as one query per team merged, per ADR 0002).
  - Each write action needs its permission:

    | Action                                  | Permission             |
    | --------------------------------------- | ---------------------- |
    | Reply, react, mark read                 | `inbox.reply`          |
    | Send a template                         | `inbox.send_template`  |
    | Close, reopen, snooze, assign, unassign | `inbox.close_reassign` |
    | Notes                                   | `inbox.notes`          |

  - Agents may always assign an unassigned chat **to themselves**, and close chats assigned to them, so that a manual queue works.
  - Quick replies: everyone reads them; `inbox.reply` creates, edits and deletes them.
  - A chat outside the member's scope answers 404, not 403.

### AD-11: Media

- **Binds:** inbound media, outbound media and display.
- **Prevents:** expiring Meta media URLs on screen, and storing bytes in Postgres.
- **Rule:**
  - An inbound media message stores the Meta media ID in `content` and enqueues `inbox_fetch_media`. The job gets the URL from Graph, downloads the file (≤ 100 MB, streamed), stores it at `inbox/{ws}/{conversation}/{message}{ext}` in S3, and records the key, MIME type, size and sha256. Then it sends NOTIFY.
  - The client gets media through `GET …/messages/{id}/media`, which streams from storage after a scope check. It never gets an S3 or Meta URL.
  - Outbound uploads are checked against Meta's types and limits: image 5 MB (jpeg, png); video 16 MB (mp4, 3gp); document 100 MB; audio 16 MB.

### AD-12: Mentions and notifications

- **Binds:** notes.
- **Prevents:** notifying people who can't see the chat.
- **Rule:**
  - `mentions` are member IDs. Each must be an active member who can see the chat, otherwise the request returns 422.
  - Slice A notifies in the app only: a `mention` NOTIFY addressed to the user shows a toast and adds to the bell count, with a `GET …/inbox/mentions` list.
  - Email and push notifications are deferred.

## 5. Data

Migrations `00020_contact_identities.sql` and `00021_inbox.sql` (slice A), and `00022_assignment.sql` (slice B).

**`conversations`**

- Columns: the AD-2 fields, plus:
  - `team_id` (FK teams, SET NULL), `assignee_id` (FK users, SET NULL);
  - `source` (`organic`, `ctwa_ad` or `template`, from the referral on the first message);
  - `snoozed_until`, `closed_at`.
- UNIQUE `(workspace_id, contact_id, number_id)`.
- Indexes:
  - `(workspace_id, status, last_message_at DESC, id)`
  - `(workspace_id, assignee_id, status, last_message_at DESC)`
  - `(workspace_id, team_id, status, last_message_at DESC)`
  - `(snoozed_until) WHERE snoozed_until IS NOT NULL`

**`messages`**

- Columns: the AD-4 fields. UNIQUE `wamid`.
- Indexes: `(conversation_id, id DESC)`, and `(workspace_id, author_id) WHERE kind = 'note'` for mentions.

**`quick_replies`**

- Columns: `workspace_id`, `shortcut` (`^[a-z0-9_-]{1,32}$`, UNIQUE per workspace), `title`, `body` (≤ 1024 characters), `created_by`.
- None are seeded.

**`inbox_mentions`**

- Columns: `workspace_id`, `message_id`, `user_id`, `seen_at`.
- PK `(message_id, user_id)`. Index `(workspace_id, user_id, seen_at)`.

Changes elsewhere:

- `contacts.last_message_at` is set by the ingest (the column already exists).
- `whatsapp_test_messages` stays as it is.

## 6. API (`/v1/workspace/inbox`, `X-Workspace-ID`)

| Method and path                                                                                            | Permission / notes                                                                                                                                    |
| ---------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /conversations` (query: `tab` open, pending, closed or mine; `q`; `sort` newest or waiting; `cursor`) | Member, scoped. Returns `{items, next_cursor, counts{open,pending,closed,mine}}`                                                                      |
| `GET /conversations/{id}`                                                                                  | Member, scoped. The conversation with its window, assignee, team and contact summary                                                                  |
| `GET /conversations/{id}/messages?before=`                                                                 | Member, scoped. 50 per page, newest first                                                                                                             |
| `POST /conversations/{id}/messages`                                                                        | `type` (text, media or template), `text`, `media_key`, `caption`, `reply_to_id`, `template_id`, `params`. Permission by type (AD-10); window per AD-6 |
| `POST /conversations/{id}/notes`                                                                           | `inbox.notes`. `{text, mentions[]}`                                                                                                                   |
| `POST /conversations/{id}/messages/{mid}/reaction`                                                         | `inbox.reply`. `{emoji}`; empty removes it                                                                                                            |
| `POST /conversations/{id}/read`                                                                            | Member, scoped. Clears unread and queues the read receipt                                                                                             |
| `POST /conversations/{id}/assign`                                                                          | `inbox.close_reassign` (or self-assign). `user_id` (null unassigns)                                                                                   |
| `POST /conversations/{id}/status`                                                                          | `inbox.close_reassign`. `status` (open or closed) or `snooze_until`                                                                                   |
| `POST /media`                                                                                              | `inbox.reply`. Multipart upload; returns `{key, mime, size, filename}`                                                                                |
| `GET /conversations/{id}/messages/{mid}/media`                                                             | Member, scoped. Streams the file                                                                                                                      |
| `GET, POST /quick-replies`; `PATCH, DELETE /quick-replies/{id}`                                            | Read: member. Write: `inbox.reply`                                                                                                                    |
| `GET /templates`                                                                                           | `inbox.send_template`. Approved templates with variables                                                                                              |
| `GET /mentions`; `POST /mentions/seen`                                                                     | Member                                                                                                                                                |
| `GET /events`                                                                                              | Member. SSE (AD-8)                                                                                                                                    |

Errors: `conversation_not_found`, `window_closed`, `contact_opted_out` (for marketing templates only), `template_not_approved`, `template_params_missing`, `media_too_large`, `media_type_not_allowed`, `invalid_mention`, `sending_paused` (trial over, per PHASE-3-ONBOARDING).

## 7. Reliability and operations

- **Redundancy:**
  - API instances are stateless apart from SSE streams; a client reconnects to any instance.
  - Each instance's hub reconnects its LISTEN connection with backoff. While it's down, clients poll (AD-8).
  - River retries sends and media fetches.
  - Postgres is the existing managed primary with automatic failover.
- **Failure modes:**

  | Failure              | Effect                                                                                          |
  | -------------------- | ----------------------------------------------------------------------------------------------- |
  | Meta down            | Sends stay pending and retry for about 5 attempts, then fail with a message the user can act on |
  | S3 down              | Media fetch retries; the bubble shows "Media is loading"                                        |
  | NOTIFY listener lost | Clients fall back to 15 s polling until it's back                                               |
  | Webhook burst        | River absorbs it; workers drain at `QUEUE_MAX_WORKERS`                                          |

- **Observability:** structured logs at each step, with `workspace_id`, `conversation_id`, `wamid`, the job and its duration, and never message bodies. Metrics (when Phase 7's OpenTelemetry lands):
  - `inbox_ingest_seconds`
  - `inbox_send_seconds{result}`
  - `inbox_send_failed_total{code}`
  - `realtime_subscribers`
  - `realtime_dropped_total`
  - `river_queue_depth{kind}`
- **Alerts:**
  - Send failure rate > 5 % over 10 min.
  - The `whatsapp_webhook` queue older than 60 s.
  - The listener reconnecting more than 5 times in 5 min.
- **Deployment:**
  - Migrations are additive only. The same release can run before the web is deployed.
  - Rolling deploys drop SSE streams, and clients reconnect and resync. No feature flag is needed: the Inbox nav item replaces "Soon".
  - Rollback is the previous binary. The new tables stay unused.
- **Data:**
  - Messages are kept while the workspace exists. Deleting a workspace cascades.
  - Retention settings are deferred.
  - Backups follow the database's existing RPO and RTO.

## 8. Build plan

The build runs as parallel tracks against this contract.

| Track | Owner area                                                                                                                                                                                                |
| ----- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| A1    | whatsapp: parse inbound messages and statuses into types; `InboundSink`; Graph send, read and media methods; tests with a fake Graph                                                                      |
| A2    | inbox backend: migration, sqlc, service (ingest, send, actions, notes, quick replies, mentions), jobs, handlers, `platform/realtime` hub and SSE endpoint, `internal/app` adapters and wiring, flow tests |
| A3    | web: `features/inbox` (list, thread, composer, template modal, right panel, quick replies, SSE hook), route `/inbox`, nav item                                                                            |
| B     | (next PR) `features/assignment` and the builder UI                                                                                                                                                        |

## 9. Trade-offs and later work

- **SSE with signal-then-refetch, not pushing data:**
  - Each event costs one extra read, about 100 reads/s at peak, which is cheap on an index. In return there is one source of truth, no content in the event stream, and no ordering bugs.
  - Pushing patches can come later if reads become the bottleneck.
- **LISTEN/NOTIFY, not Redis pub/sub:**
  - No new infrastructure, and events are delivered only on commit.
  - The limits are that every instance receives every workspace's events (fine at < 100/s) and that it needs a direct, unpooled connection.
  - Revisit at ~1,000 events/s, or when Redis arrives for the cache.
- **One thread per contact and number:** simple, and matches the boards. Meta's billing conversations aren't modelled, so pricing analytics would need Meta's `pricing` objects, which the status webhooks already carry; we drop them for now.
- **Deferred:**
  - AI copilot (the InboxAI board needs `ai_generated`, translations, mood, summaries, a knowledge base).
  - Conversation labels.
  - Commerce orders panel.
  - Collision indicator.
  - Email and push notifications for mentions.
  - Message search beyond the last-message preview.
  - Retention settings.
  - Monthly partitioning of `messages` (at ~100 GB).
  - Chatbot handoff.
  - Outbound typing indicator.

## 10. Changes made while building slice A

- **Search** uses ILIKE on the last-message preview, contact name and phone digits. `pg_trgm` isn't installed, which is fine at this scale.
- **Scope:**
  - Team scope runs one query per team, plus one for chats assigned to the member, merged in Go (ADR 0002).
  - A member always sees chats assigned to them.
  - An empty number list means every number, and owners see everything.
- **Contacts:** `FindOrCreateByPhone` runs before the ingest transaction. It is idempotent, so that is safe. `SetLastMessage` runs inside the transaction and only moves the time forward.
- **System events** (all written in the transaction that causes them):
  - `assigned`, `unassigned`, `closed`, `reopened`, `snoozed`, `unsnoozed`;
  - `template_pending`: a template moved a closed chat to Pending;
  - `window_reopened`: the customer replied after the window had closed.
- **Errors:**
  - 131056 and 130429 (rate limits) are retried.
  - whatsapp's own errors are permanent failures: number not connected, account disconnected, file too large.
- **Live stream:**
  - `EventSource` can't send `X-Workspace-ID`, so the web reads the stream with `fetch` and parses it itself; one stream per tab, mounted in the workspace shell.
  - On an assignment change, the old assignee also receives an event.
  - Worker-only processes answer `GET /events` with 503 `live_updates_unavailable`.
- **Uploads:** the upload key encodes the workspace, size and file name, so there is no uploads table. A key from another workspace is refused.
- **Not enforced yet:** `sending_paused` exists as an error, but billing has no "paused" state to check.
- **Panel:** the contacts feature has no contact notes, so the panel shows the contact, its tags and custom fields through the contacts feature's exported `ContactDetails`.

## 11. Omnichannel seams (ADR 0010)

- **Conversations:** `channel_type` (`whatsapp` | `messenger` | `instagram`) and `channel_id` replace `number_id`. UNIQUE `(workspace_id, contact_id, channel_type, channel_id)`. For WhatsApp, `channel_id` is `whatsapp_numbers.id`. Scope filtering treats a member's numbers as their WhatsApp channels.
- **Messages:** `external_id` replaces `wamid`, UNIQUE `(workspace_id, external_id)`. Ingest and status matching use it for every channel.
- **Channel interface:** the inbox declares one `Channel` interface, and `internal/app` registers one adapter per channel type. Capabilities replace every hard-coded WhatsApp rule:

  | Capability             | WhatsApp                                              |
  | ---------------------- | ----------------------------------------------------- |
  | Window                 | 24 h                                                  |
  | Templates              | yes                                                   |
  | Reactions              | yes                                                   |
  | Read receipts          | yes                                                   |
  | Media kinds and limits | image 5 MB, video 16 MB, audio 16 MB, document 100 MB |
  | Max text length        | 4096                                                  |

  A free-form send needs an open window only when the window is above 0. Templates are offered only when the channel supports them.
- **Contacts:** inbound senders resolve through `contact_identities` (`FindOrCreateByIdentity`). The WhatsApp identity is the E.164 phone; a missing identity links to the contact with that phone, or creates one.
- **API:** a conversation exposes `channel {type, id, name, handle}` and `capabilities`, and the web drives the window bar, template button, reactions and attachments from them. Integrations' `message.received` carries `channel` and the sender's `external_id`, plus `phone` when there is one.
- **Assignment (slice B):** the "WhatsApp number" condition is a "Channel" condition.
