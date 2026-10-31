# 0012. Automations

- **Status:** accepted
- **Date:** 2026-10-31

## Context

The Automations design offers six presets, each with a toggle, settings and a live count: Welcome, Away, Keyword replies, Abandoned cart, Order confirmation and Drip. Anything beyond them goes to the Chatbot builder. Messages can start from an inbound message, a store event or a tag, and some wait minutes or days. Retries and redelivered webhooks are normal, so one trigger must never send twice.

## Decision

- **Six typed kinds, not a rules engine.** `automations` holds one row per kind per workspace, with `settings` validated by a typed struct per kind (`model.Settings`). A kind with no row is off, with its defaults. No user asks for a generic trigger and action builder here, because the Chatbot builder is that product.
- **Separate permissions.** `automations.edit` saves settings and `automations.publish` turns a kind on or off. Turning on needs the settings complete.
- **Gated by plan.** Routes use `RequirePlan("automations")`, the first enforced `access` resource from ADR 0008, answering `402 plan_required`.
- **One run record for every message** (added with the first sending kind). `automation_runs` holds one row per contact per pass, and its `next_at` column is the schedule. A unique `dedupe_key` stops double sends. Pausing, cancelling and rescheduling are single `UPDATE`s, with no queued job to find.
- **Features stay apart.** Inbox, contacts and integrations call automations through small interfaces wired in `internal/app`. This uses the approved event-type handler registry, so it adds no new factory.

## Consequences

- A new preset is a new kind, one settings struct and one migration to its `CHECK`.
- Settings are `jsonb`, so changing a struct needs defaults for rows that lack the new field. `model.Decode` starts from the kind's defaults for that reason.
- A workspace that loses the plan keeps its rows and settings, and they apply again if it upgrades.
