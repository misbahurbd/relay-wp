---

status: final
updated: 2026-10-29
scope: Contacts and message templates (backend + web)
sources: ChatPulse design canvas boards "Contacts", "Templates", "Create Template"
---

# Phase 4: Contacts and templates

The contract that the four builds (contacts API, templates API, contacts web, templates web) share. It fixes only what two builders could choose differently. Everything else follows [CLAUDE.md](../CLAUDE.md) and [ARCHITECTURE.md](ARCHITECTURE.md).

**Paradigm:** the existing one, a modular monolith of vertical feature slices (`handler → service → repository`). Two new slices, `contacts` and `templates`. Neither imports the other or `whatsapp`; `internal/app` connects them.

```mermaid
flowchart LR
  subgraph web[apps/web]
    CW[features/contacts] --> AC[@relay/api-client]
    TW[features/templates] --> AC
  end
  AC -->|/v1/workspaces/:id/contacts| CH[contacts handler]
  AC -->|/v1/workspaces/:id/templates| TH[templates handler]
  CH --> CS[contacts service] --> CR[(contacts tables)]
  CS -->|import job| Q[(River)]
  CS --> S[(storage)]
  TH --> TS[templates service] --> TR[(message_templates)]
  TS -->|MetaTemplates iface| APP{{internal/app adapter}} --> WS[whatsapp service] --> META[Meta Graph API]
  META -->|webhook message_template_status_update| WS -->|TemplateStatusSink iface| APP --> TS
```

## Inherited invariants

CLAUDE.md sections 1 to 4 apply in full: layer folders, no cross-feature imports, `workspace_id` on every tenant row and repository method, no `SELECT *` or `RETURNING *`, secrets stay in their query, UUIDv7, `timestamptz`, `snake_case` JSON, cursor pagination, no new factory.

## Decisions

### AD-1: Ownership and files

- **Binds:** both API builds.
- **Prevents:** migration number clashes and two features editing one table.
- **Rule:** `contacts` owns `db/migrations/00011_contacts.sql` and its tables. `templates` owns `db/migrations/00012_templates.sql` and its table. Each feature adds its own `repository/` line to `db/sqlc.yaml`. `whatsapp` stays the only package that calls Meta.

### AD-2: Contact identity is the E.164 phone

- **Binds:** contacts API (create, edit, import), contacts web.
- **Prevents:** duplicates such as `+44 7700 900412` and `447700900412` both existing.
- **Rule:** store `phone` as E.164 (`+447700900412`), parsed with `github.com/nyaruka/phonenumbers`. Put `UNIQUE (workspace_id, phone)` on it. A number that doesn't parse is a validation error (`invalid_phone`) on create or edit, and a skipped row on import. A number written without a country code is invalid; there is no default region. The UI shows the number formatted internationally, using a `phone_display` field that the API returns.

### AD-3: Contacts schema

- **Binds:** contacts API, contacts web.
- **Prevents:** builds inventing different columns for tags, opt-in or custom fields.
- **Rule:**
  - `contacts`: `id, workspace_id, name, phone, email (citext, nullable), opt_in ('in'|'out'|'pending', default 'pending'), opt_in_source text, opt_in_at, source ('manual'|'import'|'inbox'|'api'), language (nullable), attributes jsonb default '{}' (custom fields, string values only, for example City), assigned_to uuid → users (nullable, ON DELETE SET NULL), last_message_at (nullable; Inbox will fill it), created_at, updated_at`.
  - `contact_tags`: `id, workspace_id, name citext, UNIQUE (workspace_id, name)`.
  - `contact_tag_links`: `workspace_id, contact_id, tag_id, PRIMARY KEY (contact_id, tag_id)`, cascading from both sides.
  - `contact_imports` (see AD-5).
  - Indexes: `(workspace_id, id DESC)` for paging, `(workspace_id, opt_in)`, `(workspace_id, last_message_at)`.

### AD-4: Contacts API

- **Binds:** contacts API, contacts web.
- **Prevents:** the web build guessing endpoint shapes.
- **Rule:** every route is under `/v1/workspace` (workspace in the `X-Workspace-ID` header, ADR 0005) with Auth and Member, plus the permission shown.

| Method + path                  | Permission        | Notes                                                    |
| ------------------------------ | ----------------- | -------------------------------------------------------- |
| `GET /contacts`                | `contacts.view`   | Filters, paging and the response are below the table.    |
| `GET /contacts/summary`        | `contacts.view`   | `{total, opted_in}` for the page header.                 |
| `GET /contacts/{contactId}`    | `contacts.view`   | Fields, tags, `attributes`, `assigned_to` `{id, name}`.  |
| `POST /contacts`               | `contacts.edit`   | `409 contact_exists` if the phone is already used.       |
| `PATCH /contacts/{contactId}`  | `contacts.edit`   | Partial update with the same fields.                     |
| `DELETE /contacts/{contactId}` | `contacts.delete` |                                                          |
| `POST /contacts/bulk`          | depends on action | See below. Returns `{affected}`.                         |
| `GET /contacts/export`         | `contacts.export` | Streams `text/csv` with the list's filters.              |
| `GET /contact-tags`            | `contacts.view`   | `[{id, name, contact_count}]`.                           |
| `POST /contact-tags`           | `contacts.edit`   | `{name}`. Returns the existing tag if the name is taken. |
| Import routes                  | `contacts.import` | AD-5.                                                    |
- **List:** returns `{items, next_cursor, total}`. Query `q` (name, phone or email, case-insensitive), `opt_in`, `tag_id`, `last_message` (`24h`, `7d`, `30d`, `over_90d`), `limit` (default 25, max 100) and `cursor`. `total` counts matches for the filter. Order is `id DESC`.
- **Create:** body `{name, phone, email?, opt_in?, language?, attributes?, tag_ids?}`.
- **Bulk:** body `{action, contact_ids (1 to 500), tag_id?, user_id?}`, where `action` is `add_tag`, `remove_tag`, `assign` or `delete`. `delete` needs `contacts.delete`; the other actions need `contacts.edit`.
- **Export columns:** name, phone, email, opt_in, tags, source, language, created_at, plus one column per attribute key.

### AD-5: Import is an upload, then a mapping, then a River job

- **Binds:** contacts API, contacts web (the four-step wizard).
- **Prevents:** a 50,000-row file processed inside a request on a 0.5 vCPU box, and a preview whose counts disagree with the result.
- **Rule:**
  1. `POST /contacts/imports`: multipart field `file`. Accepts `.csv` and `.xlsx` (`github.com/xuri/excelize/v2`), at most 10 MB and 50,000 data rows. Legacy `.xls` is refused with `unsupported_file`. The file goes to `platform/storage`. The response `{id, file_name, row_count, columns: [{name, samples[3], guess}]}` guesses each column as `name|phone|email|tag|attribute|skip` from its header.
  2. `PUT /contacts/imports/{importId}/mapping`: `{columns: [{name, field, attribute_key?}], tag_name?, opt_in_confirmed}`. `opt_in_confirmed` must be true (Meta requires it); otherwise `422 opt_in_required`.
  3. `GET /contacts/imports/{importId}/preview`: `{rows_found, ready (new), updates (phone exists), invalid, sample: first 10 rows with status new|update|invalid}`. It is computed from the whole file using one `phone = ANY($2)` lookup per 1,000 phones.
  4. `POST /contacts/imports/{importId}/start`: enqueues `contacts_import` in a transaction. The worker upserts in batches of 500 rows, one transaction per batch. An existing contact keeps its values except for the mapped non-empty fields, and gains the tag. Skipped rows go to a CSV in storage.
  5. `GET /contacts/imports/{importId}`: `{status: uploaded|mapped|importing|done|failed, created, updated, skipped, error?}`. The web polls it every 2 s while `importing`.
  6. `GET /contacts/imports/{importId}/skipped`: streams the skipped-rows CSV.
  - Opt-in is set to `in` with `opt_in_source = 'import'` for every imported row. An existing `out` stays `out`: an import never re-subscribes anyone.

### AD-6: Template data model

- **Binds:** templates API, templates web.
- **Prevents:** two shapes for components, and drafts colliding with Meta's names.
- **Rule:** `message_templates`: `id, workspace_id, account_id → whatsapp_accounts (nullable until submitted), name, language, category ('marketing'|'utility'|'authentication'), status ('draft'|'pending'|'approved'|'rejected'|'paused'|'disabled'), meta_template_id (nullable), rejected_reason (nullable), components jsonb, header_media_key (nullable, storage key of the sample), submitted_at, created_by, created_at, updated_at`. Add `UNIQUE (workspace_id, name, language)`. `language` is Meta's code; the API and web share one list: `en_US, en_GB, es, pt_BR, hi, bn, ar`. `components` is our own JSON shape, converted to Meta's format only inside the adapter:

```json
{
  "header": { "type": "none|text|image|video|document", "text": "", "example": "" },
  "body": { "text": "Hi {{1}}…", "examples": ["Daniel", "#4821"] },
  "footer": { "text": "" },
  "buttons": [{ "type": "quick_reply|url|phone", "text": "", "value": "" }]
}
```

### AD-7: Template validation rules (server is the authority, web mirrors them)

- **Binds:** templates API (service), templates web (zod schema in `templates.schema.ts`).
- **Prevents:** the form accepting what the server refuses, or the reverse.
- **Rule:**
  - `name` matches `^[a-z0-9_]{1,512}$`.
  - Header text is at most 60 characters with at most one variable `{{1}}`. A media header needs an uploaded sample before submitting.
  - Body is required, at most 1024 characters. Variables are `{{n}}`, numbered 1..N with no gaps, and the body can't start or end with one. `examples` length equals N, with each example non-empty.
  - Footer is at most 60 characters with no variables.
  - At most 10 buttons: at most 2 `url`, at most 1 `phone`, and quick replies grouped together. Button text is at most 25 characters. A `url` value must be `https://`. A `phone` value must be E.164.
  - Errors are `422 invalid_template` with huma's per-field error locations (`body.text`, `buttons[1].value`), which the web maps to fields.
  - Short-link domains (`bit.ly`, `tinyurl.com`, `t.co`, `goo.gl`) are a warning returned with the template (`warnings[]`), not an error.

### AD-8: Templates API and lifecycle

- **Binds:** templates API, templates web.
- **Prevents:** editing a template Meta is reviewing, and submit paths that disagree.
- **Rule:** every route is under `/v1/workspace` (workspace in the `X-Workspace-ID` header, ADR 0005) with Auth and Member.

| Method + path                                | Permission         | Notes                                     |
| -------------------------------------------- | ------------------ | ----------------------------------------- |
| `GET /templates`                             | member             | Filters and response below the table.     |
| `GET /templates/{templateId}`                | member             |                                           |
| `POST /templates`                            | `templates.create` | Creates a draft.                          |
| `PATCH /templates/{templateId}`              | `templates.create` | Not while pending; see below.             |
| `POST /templates/{templateId}/header-sample` | `templates.create` | Multipart media sample; limits below.     |
| `POST /templates/{templateId}/submit`        | `templates.submit` | Validates, then sends to Meta; see below. |
| `DELETE /templates/{templateId}`             | `templates.delete` | Deletes on Meta first if it was sent.     |
| `POST /templates/sync`                       | `templates.create` | Pulls every template from Meta.           |
- **List:** query `status`, `category`, `q`, `limit` and `cursor`. Returns `{items, next_cursor, counts: {all, approved, pending, rejected}}`.
- **Patch:** allowed for `draft`, `rejected` and `approved` only; `pending` returns `409 template_in_review`. Editing an approved template keeps it approved until resubmitted.
- **Header sample:** image up to 5 MB (jpg/png), video up to 16 MB (mp4), or document up to 100 MB (pdf), kept in storage.
- **Submit:** validates (AD-7), then calls `MetaTemplates.Submit`, a create, or an edit when `meta_template_id` is set. Sets `pending` and `submitted_at`. Needs a connected WhatsApp account; otherwise `409 whatsapp_not_connected`.
- **Sync:** upserts by `(name, language)`, so existing templates such as `hello_world` appear.

The web's "Save draft" is POST or PATCH. "Submit to Meta" is save, then submit.

### AD-9: Meta boundary through two interfaces wired in `internal/app`

- **Binds:** templates API, whatsapp, `internal/app`.
- **Prevents:** `templates` importing `whatsapp` (CLAUDE.md), and Graph calls spreading out of `whatsapp`.
- **Rule:**
  - `templates/service` declares `MetaTemplates { Submit(ctx, workspaceID, RemoteTemplate) (metaID, status string, err error); Delete(ctx, workspaceID, metaID, name string) error; List(ctx, workspaceID) ([]RemoteTemplate, error); UploadSample(ctx, workspaceID, contentType string, data io.Reader, size int64) (handle string, err error) }`, with `RemoteTemplate` in `templates/model`.
  - `whatsapp/service` gets exported methods taking plain arguments and its own types. These cover the template endpoints on the WABA (`/{waba}/message_templates` for create, list and delete; `/{template_id}` for edit) and the app's Resumable Upload API for header samples.
  - `whatsapp/service` declares `TemplateStatusSink { TemplateStatusChanged(ctx, workspaceID uuid.UUID, metaTemplateID, status, reason string) error }` and adds `message_template_status_update` to its webhook registry.
  - `internal/app/templates_meta.go` holds the two small adapters that convert between the types. That is the only file that knows both features.

### AD-10: Web routes and feature layout

- **Binds:** both web builds.
- **Prevents:** route collisions and features reaching into each other.
- **Rule:**
  - Routes under `routes/_app/_workspace/_shell/`: `contacts/index.tsx` (filters in validated search params: `q, opt_in, tag_id, last_message, cursor`), `templates/index.tsx` (`status, category, q`), `templates/new.tsx`, `templates/$templateId.tsx` (edit).
  - Features live in `features/contacts/` and `features/templates/` (ADR 0004). Each feature has its own `api/` keys, queries and mutations.
  - The sidebar in `components/layout/app-shell.tsx` gets `to` for the `contacts` and `templates` items; nothing else in that file changes.
  - Use only `components/ui` (shadcn) and `#/lib`. Never import `features/team` kit pieces (only `useAccess`, below). If a piece is needed by both new features, each keeps its own copy in its `components/kit/` until a later cleanup promotes it.
  - Actions are hidden when the member lacks the permission: use `useAccess(workspaceId).can('contacts.edit')` from `#/features/team` (its public `index.ts`; never a deeper path).

### AD-11: Out of scope in this phase

- **Binds:** all builds.
- **Prevents:** building UI for data that doesn't exist yet.
- **Rule:** the following are hidden, not stubbed:
  - in the contact panel: Orders, Spent, Broadcasts, and Recent activity (needs commerce and Inbox);
  - "Open chat" and "Send broadcast" buttons;
  - "Use in broadcast";
  - the Authentication category in Create Template (Meta's OTP templates have a fixed shape; shown disabled with "coming soon"). Synced authentication templates still list.

## Deferred

| Item                                                                   | Revisit when                                                                             |
| ---------------------------------------------------------------------- | ---------------------------------------------------------------------------------------- |
| Full-text or trigram contact search                                    | `q` with `ILIKE` gets slow above ~200k contacts in a workspace                           |
| Contact activity timeline                                              | Inbox and Broadcasts exist                                                               |
| Custom field definitions (typed, per workspace)                        | A feature needs non-string attributes                                                    |
| Several WhatsApp accounts per workspace picking the template's account | A workspace connects a second WABA; today submit uses the workspace's one active account |
| Authentication (OTP) template builder                                  | Product asks for it                                                                      |
| `.xls` import                                                          | Users ask for it                                                                         |

## Open questions

None blocking. Assumptions: 25 rows per contacts page (the design shows 10), and import file size 10 MB.

## Changes made while building

The decisions above stand; these are the refinements the builds made.

| AD   | Change                                                                                                                                                                                                                                                                                                                                                                                                |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| AD-2 | A number must parse, have a known country code and the right length for that country. `IsValidNumber` is not used, because it rejects numbers in ranges set aside for examples, such as `+447700900412`. `447700900412` and `00447700900412` are accepted as international; `07700 900412` is invalid.                                                                                                |
| AD-5 | A phone repeated within one file is skipped after its first row, and the preview counts it as invalid so the preview and the result agree. A bad email skips the row. Tag cells split on commas, semicolons and vertical bars. An empty cell never overwrites a stored value. Uploaded files and skipped-row CSVs are kept under `contact-imports/{workspace}/{import}/`; nothing cleans them up yet. |
| AD-4 | List items carry `tags` and `assigned_to` as on the detail. Bulk `assign` without `user_id` unassigns.                                                                                                                                                                                                                                                                                                |
| AD-6 | `message_templates` also stores `header_media_type` and `header_media_size`, because Resumable Upload needs them before streaming the sample. The name check in SQL is `~ '^[a-z0-9_]+$' AND char_length(name) <= 512`.                                                                                                                                                                               |
| AD-7 | Drafts are checked for format only; completeness (body, examples, button values, media sample) is checked at submit.                                                                                                                                                                                                                                                                                  |
| AD-8 | A submitted template's name and language can't change. Editing on Meta sends components only. The list's `counts` ignore the `status` filter. Header-sample upload returns `200` with the template. Sync overwrites local components for a matching `(name, language)`.                                                                                                                               |
| AD-9 | No status-sink adapter: `templates/service` already matches `TemplateStatusSink`, so it is passed in directly, with a compile-time assertion in `internal/app/templates_meta.go`. Meta's `REINSTATED` maps to `approved`; `PENDING_DELETION` and `DELETED` are ignored.                                                                                                                               |
