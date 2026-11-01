# 0013. Broadcasts

- **Status:** accepted
- **Date:** 2026-10-31

## Context

A broadcast sends one approved WhatsApp template to many contacts at once. The pieces it needs mostly exist:

- The inbox already sends templates on a contact's behalf without an agent (`inbox.SendAutomated`, used by automations). It finds or starts the conversation, refuses opted-out contacts, checks the template's variables, writes a `pending` message and enqueues `inbox_send` in the caller's transaction.
- Meta delivery statuses already land on `messages.status` (sent → delivered → read, or failed).
- The permissions `broadcasts.create`, `broadcasts.send` and `broadcasts.reports` are seeded (Phase 3), the billing catalog lists a monthly `broadcasts` resource (ADR 0008, not enforced), and integrations defines `broadcast.completed` (no emitter).

What is new is volume. One automation run sends one message; one broadcast sends thousands. Today every job runs on River's single default queue with `QUEUE_MAX_WORKERS` (10) workers, so a 10,000-message broadcast would queue 10,000 `inbox_send` jobs ahead of agent replies and automations for several minutes.

## Decision

1. **A new `broadcasts` feature owns the campaign, not the message.** It stores the broadcast and one recipient row per contact. Every message still goes through `inbox.SendAutomated`, so opt-out, template checks, conversations, statuses, SSE and the `message.status` webhook stay in one place. Broadcasts never imports inbox, contacts, templates or billing; it declares small interfaces and `internal/app` adapts them.

2. **Recipient rows are the idempotency record.** `broadcast_recipients` has primary key `(broadcast_id, contact_id)`. The audience is snapshotted into it once (`INSERT … ON CONFLICT DO NOTHING`). A batch job claims `pending` rows `FOR UPDATE SKIP LOCKED`, calls `SendAutomated` and stores the `message_id` in the same transaction. A retried job finds those rows no longer `pending`, so a contact is never sent the same broadcast twice.

3. **Bulk sends run on their own River queue.** `platform/queue` gains a second queue, `bulk` (`QUEUE_BULK_MAX_WORKERS`, default 4). Broadcast jobs and the `inbox_send` jobs they create go on `bulk`; `SendAutomated` takes a `Bulk` flag that sets the queue. Agent replies and automations keep the default queue to themselves. This is not a factory: one River client, two queues.

4. **The monthly limit counts broadcasts, not messages.** One send consumes one `broadcasts` unit through billing's `usage_counters` (ADR 0008's `Consume`/`Release`, built here as the first enforced monthly resource). It is consumed when sending starts, released if the broadcast is cancelled before any recipient was queued. `broadcasts` becomes `Enforced`.

5. **Audience = opted-in contacts from named sources, counted once.** The audience is everyone (`all`), or the union of tags, segments (ADR 0014) and uploaded lists (an import that tags every row). Only `opt_in = 'in'` contacts get it; with `skip_recent_marketing`, not those sent a marketing template in the last 24 hours. Pending and opted-out contacts are never in it; the editor shows how many were left out. An audience with no source reaches nobody.

6. **Status is derived, not copied.** Delivery counts on a broadcast come from `messages.status` joined through `broadcast_recipients.message_id`, as automations do with `automation_run_id`. The broadcast row stores only its lifecycle (`draft → scheduled → sending → completed`, or `cancelled` / `failed`).

7. **Completion is decided by a periodic tick**, once every recipient is no longer `pending` and none of its messages is still `pending`. The same transaction publishes `broadcast.completed` and, when at least one message was accepted by Meta, calls onboarding's `MarkActivated`.

8. **Progress is polled, not pushed.** The report page refetches every 5 s while a broadcast is `sending`. A broadcasts SSE channel (ADR 0009) waits until polling proves too slow.

## Consequences

- Broadcasts can't change how a message is sent; that stays the inbox's job. The cost is a small inbox change (`Bulk`, return the message id, conversation source `broadcast`).
- A huge broadcast slows only other broadcasts. Throughput is bounded by `QUEUE_BULK_MAX_WORKERS`; at ~4 workers and ~200 ms per Graph call that is roughly 20 msg/s, about 8 minutes for 10,000 recipients.
- Meta's tier limit (unique users per 24 h) is not enforced. The editor warns when the audience is larger than the number's `messaging_limit`; Meta's own refusals fail those messages permanently.
- Cancelling stops new recipients; messages already queued still go out.
- Recipient rows grow with every send (~150 B each). At 10× today's volume they need pruning or partitioning by month.
