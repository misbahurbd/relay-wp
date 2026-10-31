# Automations plan

This program builds the Automations page from the design canvas. A workspace owner turns on six ready-made automations, tunes each in a drawer, and sees real counts on each card. The rule the program enforces is that every automated message passes through one run record, so a retry never sends twice and every send can be counted. The PR ids in order are AUTO-1, AUTO-2, AUTO-3, AUTO-4, AUTO-5 and AUTO-6.

## How to read this

One box is one unit of work. Every box names the evidence that checks it. A nested box is a sub-step of the box above it. Check a box only when its evidence exists, a file, a log line, a screenshot, a test run, or a SHA. The body is a how-to. The appendices explain and record.

The program runs `skills/poteto-mode/playbooks/autopilot-stack.md` from the installed plugin. The operator lands the stack. Every PR stops at merge-ready.

Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

## Program checklist

### Arm the program

- [ ] State the protocol and this plan to the operator, then stop. Start execution only on the operator's explicit go.
- [ ] On the operator's go, write this exact text into the standing orders and restate it in your todolist. "Plan is docs/PHASE-8-AUTOMATIONS.md. PRs in order are AUTO-1 to AUTO-6, stacked on feat/inbox. Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. The operator merges. Done means all six PRs are merge-ready with a clean verdict at their exact heads."
- [ ] Read these from the installed plugin at program start. Re-read them at every tick.
  - [ ] `skills/poteto-mode/playbooks/autopilot-stack.md`
  - [ ] `skills/swarm/SKILL.md`
  - [ ] the bundled `run` skill, used as the driver because the repo has no `.claude/skills/verify/`
  - [ ] `skills/poteto-mode/playbooks/opening-a-pr.md`
  - [ ] `skills/architect/SKILL.md`
  - [ ] `skills/interrogate/SKILL.md`
- [ ] Arm the hourly audit tick as a real `/loop` in dynamic mode, which schedules its own wake-up rather than blocking on a sleep. Never leave the cadence to memory.
- [ ] Use this tick prompt, verbatim. "Re-read the execution playbook from the installed plugin and the standing objective. Audit the operation against both and fix drift in this tick. Probe every active lane and judge progress by side effects only. Stand down a stuck lane and dispatch its replacement now. Then post a short status message to the operator in chat only when the audit found a tracked change that no earlier status message reported, such as a PR opened, a code-ready head, a round launched or closed, a verdict, a merge, a stuck agent and the action taken, a blocker added or cleared, or a decision only the operator can make. Name every such change and nothing else. Do not repeat a table, the merged list, or an unchanged blocker. If the audit found none, end the turn with no reply text. Either way, log this tick's row in your decision trail. The row names the items reported, or none."
- [ ] On the operator's hold or stand-down, send every owner a zero-writes order at once.

### Spawn owners

- [ ] Spawn one owner per PR with the full lifecycle the execution playbook names.
- [ ] Follow this dependency graph. Start dependent work only after its parent merges, or base it on the parent branch when the execution playbook stacks.
  - [ ] AUTO-1 is first and branches from `feat/inbox`.
  - [ ] AUTO-2 after AUTO-1.
  - [ ] AUTO-3 after AUTO-2.
  - [ ] AUTO-4 after AUTO-3.
  - [ ] AUTO-5 after AUTO-4.
  - [ ] AUTO-6 after AUTO-3, stacked on AUTO-5 so the migration order stays linear.
- [ ] Hold the file boundaries. Only AUTO-3 edits `features/inbox/`. Only AUTO-4 and AUTO-5 edit `features/integrations/`. Only AUTO-3 and AUTO-6 edit `features/contacts/`.
- [ ] Hold the review gate. AUTO-2, AUTO-3, AUTO-4, AUTO-5 and AUTO-6 change an interaction. They wait for the operator's review in chat with screenshots and a video before merge.

### PR mechanics, for every PR

- [ ] Resolve the forge once. Default to `gh`; if `command -v origin` succeeds and Origin can resolve the repository, use `origin pr` for every PR operation. Record any fallback to `gh`. Never require `gt`.
- [ ] Open the PR ready, never draft, per **Opening a PR**. Use the run's built-in PR tool when it has one, else `origin pr create --status open --base <base-branch>` or `gh pr create --base <base-branch>` according to the resolved forge. A stack child targets its parent branch.
- [ ] Run the repo's lint and typecheck once before the PR-facing push. Push with hooks on.
- [ ] Run `/deslop` before each commit and `/no-comments` before review.
- [ ] Triage every review-bot and security-reviewer comment per `../references/bugbot-triage.md`.
- [ ] Before the code-ready report and babysit, and again after merge prep, record the base and head SHAs prepared by the topology owner under the execution playbook.

### Verdict and merge, for every PR

- [ ] A full swarm runs once, at the code-ready head. Another full swarm runs only for new behavior, a redesign, or a large new diff. A rebase, a merge-conflict resolution, or a minor fix gets a single verifier at the new head, and a wording-only change gets none. Run each full swarm per `skills/swarm/SKILL.md`. One gates lane. The ten live lanes from the PR's **Verify, live** block. The perf lane from its **Verify, perf** block. Two or more audit lanes, each with its own focus, that read the diff and the receipts and distrust the PR body. The root audits the receipts in the merge-ready report before the verdict.
- [ ] Judge every finding by the execution playbook's blocking rule. The verdict is clean when none blocks. A failure on trunk behavior in a lane marked `Regression lane against trunk.` is recorded against trunk and does not block the PR. A new head gets a fresh verdict, except for results that stay valid under the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends each verified PR to the stack and the operator lands the contiguous verified run bottom-up, with verdicts carried across a rebase only under the patch-id rule in `playbooks/shipping.md`.

### Boot recipe, for every live lane

Each live lane runs in its own worktree at the PR head. Drive through the skill path or exact commands recorded in this boot recipe.

- [ ] `git fetch origin <head-branch> && git checkout <head SHA>`.
- [ ] Create a database named `relay_automations_<n>`, set both `DATABASE_URL` and `DATABASE_DIRECT_URL` to it, set a free `HTTP_ADDR`, then run `go run ./cmd/relay migrate up && go run ./cmd/relay all` in `apps/api` and `pnpm --filter web dev` with `VITE_API_URL` pointed at that API. Never migrate the shared `relay` database. Wait for `/healthz` to answer.
- [ ] Deliver input through the browser pane for the web surface, and through signed POSTs to the real webhook routes for WhatsApp and store events, using the `whatsapp` fake channel in `apps/api/internal/app/inbox_flow_test.go` as the model. Read-only diagnostics are `psql` selects on `automation_runs` and `messages`.
- [ ] Save every screenshot to `/tmp/swarm-<pr-id>/worker-<n>/<slug>.png` and return the paths with the report.

## Store settings and gate the feature by plan (AUTO-1)

**Depends on.** None.

**Files.**

- [ ] Create `apps/api/db/migrations/<next>_automations.sql`. Take the next free number at build time, because docs/PHASE-7-INBOX.md reserves 00022 for assignment.
- [ ] Create `apps/api/internal/features/automations/{handler,service,repository,dto,model}/`.
- [ ] Edit `apps/api/db/sqlc.yaml`, `apps/api/internal/app/app.go` and `apps/api/internal/features/billing/model/resources_model.go`.
- [ ] Edit `apps/api/internal/features/billing/service/grants_service.go` and `apps/api/internal/platform/middleware/workspace.go`.
- [ ] Create `docs/adr/0012-automations.md`.

**Build.**

- [ ] Add the `automations` table with `workspace_id`, `kind`, `enabled`, `settings` and `UNIQUE (workspace_id, kind)`.
- [ ] Add `model.Kind` with the six kinds and one typed settings struct per kind, each with `Validate()`, held in one `kinds` table in `model/kinds_model.go`.
- [ ] Add `GET /v1/workspace/automations`, which returns six entries and fills defaults for kinds with no row.
- [ ] Add `PUT /v1/workspace/automations/{kind}` under `automations.edit` and `PUT /v1/workspace/automations/{kind}/enabled` under `automations.publish`.
- [ ] Add `HasAccess` and `RequirePlan` as ADR 0008 describes, and set `automations` to `Enforced`.

**You see.**

- [ ] `curl` on the list route returns six entries, all `enabled: false`, for a new workspace.
- [ ] A workspace without the `automations` resource gets 402 with code `plan_required`.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `service/automations_service_test.go` gains settings validation for all six kinds, rejecting a keyword rule with no reply and a working day whose end precedes its start. Run `task test`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` model at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Run the list route at trunk and head. Trunk has no route and returns 404, recorded as such. Gate six default entries at head. Save `list.png`. Pass when the response has six kinds in design order.
- [ ] Lane 2. Save valid Away settings, then reload. Save `away-roundtrip.png`. Pass when every field returns unchanged.
- [ ] Lane 3. Send an invalid time range. Save `away-invalid.png`. Pass when the API answers 422 and names the field.
- [ ] Lane 4. Toggle with a role that has edit but not publish. Save `no-publish.png`. Pass when the API answers 403.
- [ ] Lane 5. Save with a role that has neither permission. Save `no-edit.png`. Pass when the API answers 403.
- [ ] Lane 6. Call as a workspace on a plan without the resource. Save `locked.png`. Pass when the API answers 402 `plan_required`.
- [ ] Lane 7. Read workspace B's automations with A's token and B's header. Save `cross-tenant.png`. Pass when the API answers 403.
- [ ] Lane 8. Enable an automation twice. Save `toggle-twice.png`. Pass when one row exists and it stays enabled.
- [ ] Lane 9. Grep the new SQL for `*`, and `{workspaceId}` in routes. Save `grep.png`. Pass when both greps find nothing.
- [ ] Lane 10. Run `task gen && git diff --exit-code`. Save `gen.png`. Pass when the diff is empty.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. List route p95 at head. Trunk lacks the route, so the diff-added work is the whole request.
- [ ] Probe. Run 200 sequential authenticated GETs with `hey` against a seeded workspace, interleaved with 200 GETs of `/v1/workspace/inbox/conversations` as the control.
- [ ] Baseline. Record the the control route p95 at trunk first.
- [ ] Rule. Fail when the list route p95 exceeds 50 ms locally, or the control route regresses by more than 10 percent.

**Review gate.** None. AUTO-1 is not review-gated.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Base and verdict are current under the execution playbook and the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends it to the base-branch stack and the operator lands it bottom-up.

## Build the Automations page and six drawers to the design (AUTO-2)

**Depends on.** AUTO-1.

**Files.**

- [ ] Create `apps/web/src/routes/_app/_workspace/_shell/automations/index.tsx`.
- [ ] Create `apps/web/src/features/automations/` with `api/`, `schemas/`, `components/cards/`, `components/drawers/`, `components/kit/`, `types.ts` and `index.ts`.
- [ ] Edit `apps/web/src/components/layout/app-shell.tsx`.
- [ ] Edit `apps/web/src/features/contacts/index.ts` to export the tag query and picker.
- [ ] Move `integrations/components/kit/toggle.tsx` to `apps/web/src/components/toggle.tsx`, because two features now use it.

**Build.**

- [ ] Copy the markup of `project/Automations.dc.html` into the card grid, the blue cost notice and the header pill.
- [ ] Build one drawer component per kind on `Sheet` with `@tanstack/react-form` and zod, overriding the width to 540px.
- [ ] Disable toggles without `automations.publish` and Save without `automations.edit`, using `useAccess`.
- [ ] Show the upgrade state on 402, and the connect-store state on Abandoned cart and Order confirmation when no store is connected.

**You see.**

- [ ] The sidebar item loses its Soon label and opens the page.
- [ ] The page matches the artboard at 1440px and at 360px in both themes.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] The web app has no test runner, so `schemas/automations.schema.ts` is covered by `task typecheck` and the Go validators. Run `task typecheck && task lint`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` model at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Open Automations at trunk and head. Trunk shows Soon, recorded as such. Gate the page at head. Save `page.png`. Pass when six cards render with the design's titles.
- [ ] Lane 2. Toggle Welcome on and reload. Save `toggle.png`. Pass when the pill reads 1 of 6 active after reload.
- [ ] Lane 3. Edit Welcome text, delay and first-time checkbox, then save. Save `welcome.png`. Pass when the values return on reopen.
- [ ] Lane 4. Edit Away hours, close Friday and save. Save `away.png`. Pass when the card description reflects the new days.
- [ ] Lane 5. Add a third keyword rule and save. Save `keyword.png`. Pass when the stat line shows the new count.
- [ ] Lane 6. Open Abandoned cart with no store. Save `cart-no-store.png`. Pass when the connect-store state appears and the toggle is disabled.
- [ ] Lane 7. Open Order confirmation with a fake connected store. Save `order.png`. Pass when the variable mapping lists four rows.
- [ ] Lane 8. Open Drip, add a step and save. Save `drip.png`. Pass when the timeline shows five steps on reopen.
- [ ] Lane 9. View as a role without publish. Save `readonly.png`. Pass when toggles are disabled and explain why.
- [ ] Lane 10. Resize to 360px in dark theme. Save `mobile-dark.png`. Pass when nothing overflows and the drawer fills the width.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Largest contentful paint of the Automations page. Trunk lacks the page, so the budget is absolute.
- [ ] Probe. Run the `chrome-devtools-mcp:debug-optimize-lcp` trace on the page and on Integrations, interleaved three times each.
- [ ] Baseline. Record the Integrations LCP at trunk first.
- [ ] Rule. Fail when Automations LCP exceeds Integrations by more than 200 ms, or the route chunk adds more than 60 KB gzipped.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 1 screenshots into `docs/media/auto-2-review-page.png`.
- [ ] Record a 30 to 60 second video of the change in a lane worktree. Save it as `docs/media/auto-2-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Base and verdict are current under the execution playbook and the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends it to the base-branch stack and the operator lands it bottom-up.

## Send Welcome, Away and Keyword replies through one run engine (AUTO-3)

**Depends on.** AUTO-2.

**Files.**

- [ ] Add a migration for `automation_runs` and `messages.automation_run_id`.
- [ ] Create `features/automations/service/{engine,replies_service,stats_service}.go`, `model/runs_model.go`, `model/job_args.go`, `jobs/jobs.go` and `repository/runs_queries.sql`.
- [ ] Edit `features/inbox/service/ingest_service.go` and `send_service.go`, plus `inbox/model` for `AutomatedSend`.
- [ ] Edit `features/contacts/service/` and `repository/` to add `SetOptOut`.
- [ ] Edit `internal/app/inbox_adapters.go` and `internal/app/workers.go`.
- [ ] Edit the inbox thread message bubble in `apps/web/src/features/inbox/components/`.

**Build.**

- [ ] Add `automation_runs` with `state`, `step`, `next_at`, `dedupe_key`, `skip_reason`, `context` and `UNIQUE (workspace_id, dedupe_key)`.
- [ ] Add an `Inbound` hook interface to inbox. Call it inside the ingest transaction after the message insert with `conv.Created` and the text. It inserts `AutomationInboundArgs` by `InsertTx`.
- [ ] Add `Service.SendAutomated(ctx, tx, in)` to inbox. It takes no `authctx.Access`, leaves `author_id` null, sets `automation_run_id`, and enforces the window and opt-out rules.
- [ ] Handle STOP before the chooser. A message that is only `STOP` or `UNSUBSCRIBE`, in any case, sets `opt_in = out` with source `whatsapp_reply`, cancels the contact's active runs, adds an event to the thread and sends no reply. This runs whatever the plan or toggles say, because the customer's right to leave does not depend on them. Agents can opt a contact back in from Contacts.
- [ ] Skip every automation for a contact with `opt_in = out`, free-text replies included.
- [ ] Add the reply chooser. It picks one reply per inbound message in the order Keyword, Away, Welcome, and skips all three when an agent replied in the last 30 minutes.
- [ ] Add `ProcessRunArgs` and a one-minute `automation_tick` periodic job that requeues due runs. The tick is a reconciler, and creation also enqueues immediately.
- [ ] Compute card stats from `automation_runs` joined to `messages` for the current month.

**You see.**

- [ ] A customer who replies STOP gets no further automated messages.
- [ ] A first message to a test number gets the Welcome reply in under 5 seconds.
- [ ] The Inbox thread labels the reply as automated, and the card reads Sent 1 times this month.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `service/replies_service_test.go` gains the chooser table, covering all eight on-or-off combinations, and the STOP matcher, where `please stop` does not match.
- [ ] `service/engine_test.go` gains a run processed twice, which sends once.
- [ ] `app/automations_flow_test.go` gains a real-database inbound-to-send flow with a fake `Channel`. Run `task test`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` model at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Send an inbound WhatsApp message at trunk and head. Trunk stores it and sends nothing, recorded as such. Gate the reply at head. Save `welcome-sent.png`. Pass when one outbound message has `automation_run_id` set.
- [ ] Lane 2. Replay the same webhook three times. Save `redelivery.png`. Pass when one run and one reply exist.
- [ ] Lane 3. Message outside working hours in Asia/Dhaka. Save `away-sent.png`. Pass when Away fires and Welcome is skipped with a reason.
- [ ] Lane 4. Message twice in one closed period. Save `away-once.png`. Pass when one Away reply exists.
- [ ] Lane 5. Send `price?` with an `any word` rule. Save `keyword-any.png`. Pass when that rule's reply is sent.
- [ ] Lane 6. Send text containing an exact phrase and text that only shares one word. Save `keyword-exact.png`. Pass when only the first matches.
- [ ] Lane 7. Have an agent reply, then send an inbound keyword. Save `agent-active.png`. Pass when the run is skipped with `agent_active`.
- [ ] Lane 8. Set a 5-minute Welcome delay, then have an agent reply within it. Save `delay-cancel.png`. Pass when no automated message is sent.
- [ ] Lane 9. Reply `stop` with every automation off. Save `stop.png`. Pass when the contact is `opt_in = out`, the thread shows the event, no reply is sent, and the next inbound creates no send.
- [ ] Lane 10. Kill the API between run creation and send, then restart. Save `crash.png`. Pass when the tick sends exactly one reply.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Time from webhook POST to the automated message row, and ingest latency with and without the hook.
- [ ] Probe. Post 200 signed webhooks with `hey`, alternating hook on and hook off, on trunk and head.
- [ ] Baseline. Record the ingest p95 at trunk first.
- [ ] Rule. Fail when ingest p95 regresses by more than 10 percent, or webhook-to-send p95 exceeds 5 seconds.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 1 screenshots into `docs/media/auto-3-review-welcome-sent.png`.
- [ ] Record a 30 to 60 second video of the change in a lane worktree. Save it as `docs/media/auto-3-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Base and verdict are current under the execution playbook and the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends it to the base-branch stack and the operator lands it bottom-up.

## Receive store orders and send Order confirmation (AUTO-4)

**Depends on.** AUTO-3.

**Files.**

- [ ] Add a migration for `commerce_events`.
- [ ] Create `features/integrations/handler/commerce_handler.go` and `service/commerce_service.go`, plus `repository/commerce_queries.sql`.
- [ ] Edit `features/integrations/service/shopify_oauth.go`, `woocommerce_oauth.go` and `connections_service.go`.
- [ ] Create `features/automations/service/commerce_service.go`.
- [ ] Edit `features/inbox/service/send_service.go` and `internal/app/app.go`.

**Build.**

- [ ] Add `POST /v1/webhooks/commerce/{provider}/{connectionId}`, which verifies the provider signature and stores the delivery ID in `commerce_events` before doing anything else.
- [ ] Register `orders/create` and `orders/paid` for Shopify and `order.created` for WooCommerce when a store connects, and add a Reconnect path for existing stores.
- [ ] Map provider payloads to one `model.OrderEvent` and pass it to automations through an interface wired in `internal/app`.
- [ ] Add `StartConversation` to inbox, so a template can reach a contact with no thread. Use the contact's latest channel, else the workspace's first connected number.
- [ ] Add Shopify's privacy webhooks `customers/data_request`, `customers/redact` and `shop/redact` at `/v1/webhooks/shopify/privacy`, signed the same way, because Shopify sends them to one app-wide address. `commerce_events` holds no customer data. `customers/redact` clears that customer's `automation_runs.context`. `shop/redact` clears it for the store's runs and deletes its `commerce_events` rows. `customers/data_request` is logged and queued for the operator, because Relay has no customer export yet.
- [ ] Resolve template variables from the kind's source catalogue. Tracking URL reads the store's order status URL, because a tracking number rarely exists at order creation.

**You see.**

- [ ] A signed test order creates one run and one template message.
- [ ] The Inbox shows a new thread with the confirmation in it.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `service/commerce_service_test.go` gains an `orders/paid` delivery ignored while the setting is `created`.
- [ ] `service/commerce_service_test.go` gains signature pass and fail cases for both providers, using captured sample payloads, plus redaction of one customer leaving another's rows untouched.
- [ ] `automations/service/commerce_service_test.go` gains the variable mapping table. Run `task test`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` model at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Post a signed Shopify order at trunk and head. Trunk has no route, recorded as such. Gate the send at head. Save `order-sent.png`. Pass when one template message exists with four variables filled.
- [ ] Lane 2. Post the same delivery twice. Save `order-dedupe.png`. Pass when one run exists.
- [ ] Lane 3. Post with a bad signature. Save `bad-sig.png`. Pass when the route answers 401 and stores nothing.
- [ ] Lane 4. Post a signed `customers/redact` for a customer with runs. Save `redact.png`. Pass when the route answers 200 and that customer's run context and commerce events are gone.
- [ ] Lane 5. Post a WooCommerce order. Save `woo.png`. Pass when the same template flow runs.
- [ ] Lane 6. Post an order with no phone. Save `no-phone.png`. Pass when the run is skipped with `no_phone`.
- [ ] Lane 7. Post an order for an opted-out contact. Save `opt-out.png`. Pass when the run is skipped with `opted_out`.
- [ ] Lane 8. Delete the selected template, then post an order. Save `template-gone.png`. Pass when the run fails with `template_unavailable` and the card shows it.
- [ ] Lane 9. Disconnect the store. Save `disconnected.png`. Pass when the card returns to the connect-store state.
- [ ] Lane 10. Connect a Shopify development store through the real OAuth flow. Save `shopify-dev.png`. Pass when both topics register, and record any protected-data 403 as a blocker.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Webhook acknowledgement time. Shopify expects a fast 2xx.
- [ ] Probe. Post 500 signed orders with `hey`, interleaved with 500 inbound WhatsApp webhooks as the control.
- [ ] Baseline. Record the control p95 at trunk first.
- [ ] Rule. Fail when the commerce route p95 exceeds 200 ms, or the control regresses by more than 10 percent.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 1 screenshots into `docs/media/auto-4-review-order-sent.png`.
- [ ] Record a 30 to 60 second video of the change in a lane worktree. Save it as `docs/media/auto-4-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Base and verdict are current under the execution playbook and the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends it to the base-branch stack and the operator lands it bottom-up.

## Remind shoppers about abandoned carts and count recoveries (AUTO-5)

**Depends on.** AUTO-4.

**Files.**

- [ ] Create `features/automations/service/cart_service.go`.
- [ ] Edit `features/integrations/service/commerce_service.go` and `shopify_oauth.go`.
- [ ] Edit `features/automations/service/stats_service.go`.

**Build.**

- [ ] Add `read_checkouts` to the Shopify scopes and register `checkouts/create` and `checkouts/update`.
- [ ] Upsert one run per checkout token. Each update pushes `next_at` to the chosen delay, because Shopify sends no abandoned event.
- [ ] Require the shopper's marketing consent from the checkout or `opt_in = in`, because the reminder is a marketing template.
- [ ] Cancel the run on an order carrying the same checkout token. When a reminder was already sent, store the order total in `recovered_cents`.
- [ ] Advance to step 2 at 24 hours when the second reminder is on.

**You see.**

- [ ] A checkout left for the delay gets one reminder.
- [ ] A later order turns the card stat into 1 sent · 1 carts recovered ($86.50).

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `service/cart_service_test.go` gains delay reset, cancel on order, recovery, and no consent. Run `task test`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` model at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Post a checkout at trunk and head. Trunk has no route topic, recorded as such. Gate the reminder at head with a 30-minute delay forced short by test clock. Save `cart-sent.png`. Pass when one reminder is sent.
- [ ] Lane 2. Post three updates in the delay. Save `cart-reset.png`. Pass when `next_at` moves and one reminder is sent.
- [ ] Lane 3. Post the order before the delay ends. Save `cart-cancel.png`. Pass when the run is cancelled with `order_placed`.
- [ ] Lane 4. Post the order after the reminder. Save `cart-recovered.png`. Pass when the card shows the recovered amount.
- [ ] Lane 5. Post a checkout with no marketing consent. Save `cart-consent.png`. Pass when the run is skipped with `no_consent`.
- [ ] Lane 6. Enable the second reminder and let it fire. Save `cart-second.png`. Pass when two messages exist on one run.
- [ ] Lane 7. Set a discount code and save. Save `cart-discount.png`. Pass when the variable carries the code.
- [ ] Lane 8. Pause the automation with a run due. Save `cart-pause.png`. Pass when the run cancels with `paused`.
- [ ] Lane 9. Look at a WooCommerce store. Save `cart-woo.png`. Pass when the card says carts need a Shopify store.
- [ ] Lane 10. Reconnect an old Shopify store. Save `cart-reconnect.png`. Pass when the new scope and topics apply.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Cost of a checkout update burst.
- [ ] Probe. Post 1000 updates across 100 checkouts, interleaved with the AUTO-4 order probe.
- [ ] Baseline. Record the order route p95 at the AUTO-4 head first.
- [ ] Rule. Fail when the update route p95 exceeds 200 ms, or one tick takes more than 5 seconds with 10000 due runs.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 1 screenshots into `docs/media/auto-5-review-cart-sent.png`.
- [ ] Record a 30 to 60 second video of the change in a lane worktree. Save it as `docs/media/auto-5-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Base and verdict are current under the execution playbook and the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends it to the base-branch stack and the operator lands it bottom-up.

## Run the Drip sequence from a contact tag (AUTO-6)

**Depends on.** AUTO-3, stacked on AUTO-5.

**Files.**

- [ ] Create `features/automations/service/drip_service.go`.
- [ ] Edit `features/contacts/service/contacts_service.go` and `internal/app/app.go`.
- [ ] Edit `apps/web/src/features/automations/components/cards/`.

**Build.**

- [ ] Add a `TagListener` interface to contacts. Call it inside the `linkTags` transaction for newly linked tags.
- [ ] Enrol a contact on a new tag link only, never by backfill, with dedupe key `drip:<automation>:<contact>`.
- [ ] Skip contacts without `opt_in = in`, because every step is a marketing template.
- [ ] Cancel on inbound when Stop on reply is on, inside the inbound job.
- [ ] Hold runs while paused. On resume, shift `next_at` forward by the paused time, so nobody gets a burst.
- [ ] Offer only templates with no variables or one, mapped to first name.

**You see.**

- [ ] Tagging a contact starts the sequence.
- [ ] The card reads Paused · 1 contacts in sequence when the automation is paused.

**Verify, unit.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] `service/drip_service_test.go` gains enrol once, step timing, stop on reply, and shift on resume. Run `task test`.

**Verify, live.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked. Ten lanes on the configured `swarm workers` model at the PR head, per the boot recipe.

- [ ] Lane 1. Regression lane against trunk. Tag a contact at trunk and head. Trunk does nothing, recorded as such. Gate the step 1 send at head. Save `drip-start.png`. Pass when the run is at step 1.
- [ ] Lane 2. Advance the test clock to day 2. Save `drip-step2.png`. Pass when step 2 is sent.
- [ ] Lane 3. Have the contact reply. Save `drip-stop.png`. Pass when the run cancels with `replied`.
- [ ] Lane 4. Turn Stop on reply off and reply. Save `drip-keep.png`. Pass when the run continues.
- [ ] Lane 5. Remove and re-add the tag. Save `drip-once.png`. Pass when no second run starts.
- [ ] Lane 6. Tag 500 contacts in a bulk action. Save `drip-bulk.png`. Pass when 500 runs exist and the bulk action still answers.
- [ ] Lane 7. Tag a contact with `opt_in = pending`. Save `drip-consent.png`. Pass when the run is skipped with `no_consent`.
- [ ] Lane 8. Pause, wait, and resume. Save `drip-resume.png`. Pass when the next step is shifted and none are lost.
- [ ] Lane 9. Enable with contacts already tagged. Save `drip-no-backfill.png`. Pass when no run starts for them.
- [ ] Lane 10. Delete a template used by step 2. Save `drip-template.png`. Pass when the run fails with `template_unavailable`.

**Verify, perf.** Tests alone are not sufficient verification. A PR is verified only when its unit, live, and perf boxes are all checked.

- [ ] Metric. Bulk tag time with the listener on.
- [ ] Probe. Tag 5000 contacts through the bulk route at trunk and head, interleaved three times.
- [ ] Baseline. Record the bulk tag time at trunk first.
- [ ] Rule. Fail when bulk tagging regresses by more than 20 percent, or one tick takes more than 5 seconds with 100000 scheduled runs.

**Review gate.** The operator reviews before merge.

- [ ] Copy lane 1 screenshots into `docs/media/auto-6-review-drip-start.png`.
- [ ] Record a 30 to 60 second video of the change in a lane worktree. Save it as `docs/media/auto-6-review.mp4`.
- [ ] Post the screenshots and the video in chat. Stop at merge-ready. Wait for the operator's click.

**Merge.**

- [ ] Root's clean verdict at the exact head SHA.
- [ ] Bugbot triage done.
- [ ] Base and verdict are current under the execution playbook and the patch-id rule in `playbooks/shipping.md`.
- [ ] The root appends it to the base-branch stack and the operator lands it bottom-up.

## Close the program

- [ ] Every box above is checked with its evidence.
- [ ] Reply to the operator with the report the execution playbook names.

## Appendix A. Prototype evidence

No prototype ran, because no open question is an observable fork that a sketch could settle cheaply. Each load-bearing fact was read from the code. Three stay unproven until AUTO-4 or AUTO-5 touches a real store. Whether `orders/create` and `checkouts/update` need Shopify's protected customer data approval is unconfirmed, and community reports of 403 errors on these topics point that way. Whether Shopify checkout payloads carry the marketing consent field needs a sample. Whether WooCommerce offers an abandoned event is assumed to be no, from memory. Whether Shopify requires the three privacy webhooks for this app, and whether they are set in the app's configuration or per shop, is also from memory, so the AUTO-4 owner confirms both first.

## Appendix B. Alternatives rejected

Cancellable River jobs per message lost to `automation_runs.next_at`. A job would need cancelling on every pause, order or reply, and a checkout update would need rescheduling. A row makes each of those one `UPDATE`, and a crashed run is found by the next tick.

A generic trigger, condition and action rules engine lost to six typed kinds. The design offers six presets and sends anything more to the Chatbot builder, so generality would have no user.

Building on the outbound integrations event stream lost to a direct interface. `Publish` writes deliveries for customers' webhooks, and reusing it would couple internal behavior to a public contract.

Treating `first-time contact` as `FindOrCreateByIdentity` created lost to `conv.Created`. A contact imported from CSV has never messaged, yet is not created by that call.

## Appendix C. Risks

- AUTO-4. Shopify's protected customer data review may block non-development stores. The owner tests on a development store and records the outcome. The operator files the review.
- AUTO-4. Manual-token stores use a custom app whose webhook secret Relay may not hold. The owner limits webhooks to OAuth connections unless a test proves otherwise.
- AUTO-4. Utility sends to contacts with `opt_in = pending` rest on the store's own consent. See the operator decision in the artifact.
- AUTO-3. The hook runs inside ingest. It only inserts a job, so a slow engine cannot slow ingest.
- AUTO-1. Setting `automations` to `Enforced` could lock existing workspaces whose plan lacks it. The owner checks seeded plans before merge.
- AUTO-3 and slice B. Assignment may take migration 00022 first. Each owner takes the next free number at build time.
- AUTO-3. A customer's own auto-responder could loop with Keyword replies. The chooser caps automated replies at three per conversation per hour.

## Appendix D. Links and reading list

Read `docs/ARCHITECTURE.md`, `docs/adr/0005-workspace-header.md`, `docs/adr/0008-plan-resource-catalog.md`, `docs/adr/0010-omnichannel-inbox.md` and `docs/PHASE-5-INTEGRATIONS.md`. The design source is `project/Automations.dc.html` on the canvas. AUTO-3 and AUTO-4 get `skills/how/SKILL.md` and `skills/interrogate/SKILL.md`. Keep a trail per `skills/show-me-your-work/SKILL.md`.
