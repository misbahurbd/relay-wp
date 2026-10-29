# Phase 3 plan: onboarding

Status: **Draft for review.** Nothing built yet. Covers everything a new user goes through from sign-up to their first WhatsApp message, as designed in the ChatPulse prototype ("0 · Journey map", "Sign up", "2 · Verify email", "3 · Welcome", "Onboarding setup", "Connect WhatsApp – full flow", "Connection problems", "5 · First-run dashboard", "Log in"). Follows the rules in [CLAUDE.md](../CLAUDE.md) and the architecture in [ARCHITECTURE.md](ARCHITECTURE.md).

Phase 3 in ARCHITECTURE.md is "Workspaces". This document covers the part of it that onboarding needs (creating a workspace, its owner membership, setup progress, trial dates) plus the WhatsApp connection that setup step 2 depends on. Invites, roles and the rest of team access get their own document. The frontend pages are Phase 4; they are described here so the API matches them.

Decisions marked **(proposed)** depend on an open question in section 9.

---

## 1. Decisions

| Topic                   | Decision                                                                                                                                                                                                              |
| ----------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Email verification      | **6-digit code** by email, valid 10 minutes, 5 tries per code. Entering the right code **signs the user in**. Replaces the link for sign-up; reset stays a link.                                                      |
| Social sign-in buttons  | Sign up and Log in show a button for **every configured provider** (Google, Facebook, Apple), not only Google as in the design. The list comes from `GET /auth/providers`, so an unconfigured provider has no button. |
| Who skips verification  | Social sign-ups whose provider vouches for the email (Google, Apple). Facebook: see question 2.                                                                                                                       |
| When a workspace exists | Created when the user leaves the Welcome screen (either button), with a default name. Setup step 1 fills in the business details.                                                                                     |
| Setup steps             | 3 steps saved on the server: business details → connect WhatsApp → send test message. Resuming always returns to the first unfinished step.                                                                           |
| Skipping WhatsApp       | Allowed on step 2 only. Skipping **ends setup** and lands on the first-run dashboard with the "Connect WhatsApp" banner (step 3 needs a number, so it is skipped too). **(proposed)**                                 |
| Connecting WhatsApp     | Meta Embedded Signup (Facebook JS SDK pop-up). The backend exchanges the code, stores the business token encrypted, then registers the number in a background job.                                                    |
| Number types            | New number (default), WhatsApp Business app number (coexistence), number from another provider (migration). See question 4 for what ships first.                                                                      |
| Test message            | Pre-approved `hello_world` template to a number the user types, which must differ from the connected one. Delivery status comes from Meta's webhook.                                                                  |
| Trial                   | 14 days per workspace on the Growth plan, starting when the workspace is created. Phase 3 only stores the dates and sends reminders; enforcement is Phase 6 (Billing).                                                |
| Activated               | Connected number **and** at least one message to a real customer (broadcast or inbox reply). The 7-day window is a metric, not a gate.                                                                                |
| Checklist               | Derived on the server from real data; not ticked by hand. **(proposed)**                                                                                                                                              |
| Product tour, checklist | Shown/hidden **per member**, not per workspace.                                                                                                                                                                       |
| Terms acceptance        | Required at sign-up; time and version stored on `users`.                                                                                                                                                              |

---

## 2. The flow

### Main path

| #   | Screen              | Route (Phase 4)                        | Time    | User does                                                                                            | Server effect                                                                                        |
| --- | ------------------- | -------------------------------------- | ------- | ---------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| 1   | Sign up             | `_auth/signup`                         | 1 min   | Full name, work email, password, accepts terms. Or "Continue with Google / Facebook / Apple".        | `POST /auth/signup` → user, profile, preferences; emails a code. Social: signed in, skips to 3.      |
| 2   | Verify email        | `_auth/verify-email`                   | 30 sec  | Types the 6-digit code. "Change email" goes back to 1. Resend after 60 s.                            | `POST /auth/verify-email/code` → verified **and signed in**.                                         |
| 3   | Welcome             | `_app/welcome`                         | 15 sec  | Picks a first goal (Send offers / Answer customers / Automate replies), then Start setup or Explore. | `POST /workspaces` → workspace, owner membership, onboarding row, trial dates.                       |
| 4   | Business details    | `_app/w/$workspaceId/setup` (step 1)   | 1 min   | Business name, industry, website, country, team size, goals.                                         | `PUT /workspace/onboarding/business`                                                                 |
| 5   | Connect WhatsApp    | `_app/w/$workspaceId/setup` (step 2)   | 3 min   | Logs in with Meta, picks the business, adds and verifies the number. Or "Skip for now".              | `POST /workspace/whatsapp/connections`, then the setup job. Skip: `POST …/onboarding/skip-whatsapp`. |
| 6   | Send test message   | `_app/w/$workspaceId/setup` (step 3)   | 30 sec  | Enters their own number, sends `hello_world`, sees "Delivered", clicks Go to dashboard.              | `POST …/whatsapp/numbers/{id}/test-messages`; `POST …/onboarding/complete`.                          |
| 7   | First-run dashboard | `_app/w/$workspaceId/` (not activated) | 2 min   | Takes the 4-step tour, works through the checklist.                                                  | `GET …/onboarding` drives the checklist; `PATCH …/onboarding/me` stores tour/checklist visibility.   |
| 8   | Everyday dashboard  | `_app/w/$workspaceId/` (activated)     | Ongoing | Normal use.                                                                                          | `activated_at` set on the workspace.                                                                 |

### Where to send a signed-in user

Run on every app load and after every sign-in:

1. Password user with an unverified email → **Verify email** (the login endpoint already returns `403 email_not_verified`).
2. No workspace membership → **Welcome**.
3. Last-used workspace has `onboarding.completed_at IS NULL` → **Setup**, at `onboarding.current_step`, with saved answers prefilled.
4. Otherwise → the workspace dashboard (first-run or everyday, by `activated_at`).

Invitees who sign up from an invite link join that workspace and skip 3–6 (team access document).

### Side paths

| Path                     | Sequence                                                                   | Notes                                                                                                                                    |
| ------------------------ | -------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------- |
| Explore with sample data | Welcome → first-run dashboard with `sample_data = true`                    | Banner: "You're viewing sample data. Connect your number to go live." Links to the full connect flow. Setup is marked started, not done. |
| Skip WhatsApp            | Setup step 2 → Skip → first-run dashboard                                  | Amber banner "Connect WhatsApp to start messaging". Sending is locked; everything else works.                                            |
| Returning user           | Log in → routing rules above                                               | Unfinished setup resumes at the saved step.                                                                                              |
| Abandoned setup          | Leaves during setup → email after 24 h → email on day 3 → logs in → resume | Day 1: "You're 1 step away from WhatsApp messaging." Day 3: offers a free 15-minute setup call. Both stop once setup is done.            |

### Screen details that affect the API

**Sign up.** One "Continue with …" button per provider from `GET /auth/providers`, in the order Google, Facebook, Apple, above "or sign up with email". Each starts `GET /auth/oauth/{provider}/start` with `redirect_to` set to the post-sign-in routing page; a new social user then lands on Welcome. "Full name" is one field; the frontend splits it into `first_name` and `last_name` (question 6). The password hint in the design says "8 characters, a number and a symbol", but our rules also require upper and lower case (question 7). One checkbox accepts the Terms of Service, the Privacy Policy and the WhatsApp Business messaging policy; the button stays disabled until it is ticked.

**Verify email.** Six single-digit inputs with paste support and `autocomplete="one-time-code"`. Errors: "That code doesn't match. Check the latest email and try again." Tips panel: check spam, code expires in 10 minutes, sender is `no-reply@…`. Success: "Email verified", then Continue → Welcome.

**Welcome.** Shows the trial start, the 3 setup steps with times (1 + 3 + 1 min) and what to have ready (a Facebook admin account, a phone that can get an SMS or call). The first-goal choice decides which features the dashboard highlights:

| `first_goal` | Label            | Highlights                   |
| ------------ | ---------------- | ---------------------------- |
| `offers`     | Send offers      | Broadcasts and Templates     |
| `support`    | Answer customers | Team Inbox and quick replies |
| `automate`   | Automate replies | Automations and Chatbots     |

**Setup step 1, business details.**

| Field       | Required | Values                                                                                                  |
| ----------- | -------- | ------------------------------------------------------------------------------------------------------- |
| `name`      | Yes      | 2–80 characters. Tip: use the name customers know, so Meta approves the display name faster.            |
| `industry`  | Yes      | `retail_ecommerce`, `food_restaurants`, `education`, `health_clinics`, `travel`, `real_estate`, `other` |
| `website`   | No       | `https` URL                                                                                             |
| `country`   | Yes      | ISO 3166-1 alpha-2. Prefilled from the browser locale.                                                  |
| `team_size` | Yes      | `just_me`, `2_5`, `6_20`, `20_plus`                                                                     |
| `goals`     | Yes, ≥ 1 | Any of `marketing`, `support`, `sales`. Only changes which starter templates are suggested.             |

**Setup step 2, connect WhatsApp.** Summarises Meta's 4 steps, lists what to have ready, and warns that the number must not be active on the WhatsApp app. "Connect with Meta" opens the pop-up. Once connected it shows the number, the display name ("In review by Meta"), and the messaging limit ("250 customers / 24h"). "Skip for now" shows only while nothing is connected; Continue is disabled until connected. See question 3 for whether this step embeds the full 5-step connect flow below.

**Setup step 3, test message.** Country code + number; must differ from the connected number. Shows the `hello_world` template ("Approved, pre-approved by Meta") and a live phone preview. After sending: "Delivered to +880 1712 345 678. Check your phone. Reply to it and the reply will show up in your Inbox." Then Go to dashboard.

The setup page also shows a step-specific tips card, "Step N of 3 · about X minutes left", and "Need help? Book a free setup call" / "Chat with support" links.

### Full connect flow (Settings › WhatsApp channel › Connect a number)

Also reached from the dashboard banner, the checklist, and the sample-data banner.

| Step | Name              | Content                                                                                                                                                                                        |
| ---- | ----------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1    | Choose number     | `new` (recommended), `coexistence` (WhatsApp Business app number, scan a QR code, recent chats sync), `migration` (from another API provider; turn off two-step verification there first).     |
| 2    | Get ready         | 4 checkboxes (Facebook admin, business name/website/address, a phone that can get a code, about 5 minutes). Continue is disabled until all are ticked. Client-side only.                       |
| 3    | Connect with Meta | Meta's pop-up, 6 sub-steps: log in → business portfolio → WhatsApp Business account (name, time zone) → phone number, display name, category, code by SMS or call → verify code → permissions. |
| 4    | Setting up        | Server job progress: register number, subscribe to webhooks, sync templates, import business profile, send test message. "Keep this page open."                                                |
| 5    | Connected         | Number, display name status ("usually 1–2 days"), quality rating, messaging limit. Next steps: payment method in Meta, profile photo, first template, import contacts. Go to dashboard.        |

### Connection errors

Every error says what happened and offers one fix and one alternative. Most happen inside Meta's window, where Meta shows its own message; we only learn the user cancelled (with the sub-step they stopped at).

| Error                                | Where it happens        | How we know                                                                    | Fix / alternative                                   |
| ------------------------------------ | ----------------------- | ------------------------------------------------------------------------------ | --------------------------------------------------- |
| Number already on WhatsApp           | Step 1 or Meta's window | Meta's window, or `register` fails with Meta's "number in use" error           | Connect it with the Business app / How to delete it |
| Didn't get the code                  | Meta's window           | Meta handles it                                                                | Resend in 0:45 / Call me instead                    |
| Wrong code (n tries left)            | Meta's window           | Meta handles it; locks for a few hours after too many                          | Try again / Send a new code                         |
| Meta's window closed or blocked      | Step 3                  | Frontend: SDK `CANCEL` event, or no response / pop-up blocked                  | Open Meta's window again / How to allow pop-ups     |
| Display name not approved            | After connecting        | `phone_number_name_update` webhook                                             | Edit and resubmit / See naming rules                |
| No admin access / portfolio full     | Meta's window           | Meta handles it                                                                | Ask an admin to invite you / Create a new portfolio |
| Disconnected (token expired/revoked) | Any page, as a banner   | Graph API error 190 or 10, or `account_update` webhook → number `disconnected` | Reconnect with Meta                                 |

### First-run dashboard

- **Trial banner**: "Free trial: N days left", link to plans.
- **Skipped banner** (no number): "Connect WhatsApp to start sending messages".
- **Checklist** "N of 6 done", collapsible and hideable per member. When all 6 are done: "You're all set" and a link to the everyday dashboard.

  | Item                         | Done when                                          | Available               |
  | ---------------------------- | -------------------------------------------------- | ----------------------- |
  | Create your account          | Always                                             | Phase 3                 |
  | Connect your WhatsApp number | A number in the workspace is `connected`           | Phase 3                 |
  | Create your first template   | A template is approved                             | With Templates          |
  | Import contacts              | The workspace has ≥ 1 contact                      | With Contacts           |
  | Send your first broadcast    | A broadcast or inbox reply went to a real customer | With Broadcasts / Inbox |
  | Invite a teammate            | The workspace has ≥ 2 members or a pending invite  | With Team               |

- **Product tour**, 4 steps: Team inbox → Broadcasts → Templates → Your setup checklist. Skip or Finish both mark it seen; "Take the tour" restarts it.
- **Empty states**: 5 stat cards at 0 or "—"; Inbox with the `wa.me` link, QR code and copy button; Broadcasts; Contacts.
- Help: a 2-minute setup video, the getting-started guide, "Book a free 15-min onboarding call", and the chat widget.

### Log in (returning users)

"Welcome back", the same provider buttons as Sign up, email + password, "Remember me on this device for 30 days" (question 8), forgot password → "Check your email" (link valid 30 minutes in the design, 1 hour in Phase 2; question 9). Under the button: "Didn't finish setting up? We'll take you back to the step you left, with everything you entered saved."

### Trial timeline

| Day | What happens                                                             | Phase 3 does      |
| --- | ------------------------------------------------------------------------ | ----------------- |
| 1   | Trial starts on Growth, all features on                                  | Stores the dates  |
| 10  | In-app banner and email: 4 days left, pick a plan                        | Sends the email   |
| 13  | Last reminder, with a summary of what they've sent                       | Sends the email   |
| 14  | Trial ends. Inbox stays readable; sending pauses until a plan is chosen. | Nothing (Phase 6) |

---

## 3. Database

All IDs are UUIDv7. All times are `timestamptz` in UTC. Tables with `updated_at` use the `set_updated_at()` trigger.

### Changes in `features/auth`

**`users`**: two new columns.

| Column              | Type                  | Notes                                    |
| ------------------- | --------------------- | ---------------------------------------- |
| `terms_accepted_at` | timestamptz, nullable | Set at sign-up. NULL for existing users. |
| `terms_version`     | text, nullable        | e.g. `2026-10`, from config              |

**`email_codes`**: new; one per verification code sent.

| Column       | Type                      | Notes                                                                                                          |
| ------------ | ------------------------- | -------------------------------------------------------------------------------------------------------------- |
| `id`         | uuid, PK                  |                                                                                                                |
| `user_id`    | uuid, FK → users, CASCADE |                                                                                                                |
| `purpose`    | text                      | `verify` (CHECK; room for others later)                                                                        |
| `email`      | citext                    | Address the code was sent to                                                                                   |
| `code_hash`  | bytea                     | **Secret.** HMAC-SHA256 of the code with `EMAIL_CODE_KEY`. A plain hash of 6 digits could be reversed offline. |
| `attempts`   | int, NOT NULL, default 0  | Wrong tries; the code is dead at 5                                                                             |
| `expires_at` | timestamptz               | 10 minutes                                                                                                     |
| `used_at`    | timestamptz, nullable     |                                                                                                                |
| `created_at` | timestamptz               |                                                                                                                |

Index `(user_id, purpose) WHERE used_at IS NULL`. A new code invalidates the user's earlier unused codes. Not unique on `code_hash`: different users can get the same 6 digits.

### Owned by `features/workspaces`

**`workspaces`**

| Column                     | Type                             | Notes                                     |
| -------------------------- | -------------------------------- | ----------------------------------------- |
| `id`                       | uuid, PK                         |                                           |
| `name`                     | text, NOT NULL                   | Default "Shakil's workspace" until step 1 |
| `industry`                 | text, nullable                   | CHECK on the values in section 2          |
| `website`                  | text, nullable                   |                                           |
| `country`                  | text, nullable                   | ISO alpha-2                               |
| `team_size`                | text, nullable                   | CHECK                                     |
| `goals`                    | text[], NOT NULL, default `{}`   | Subset of `marketing`, `support`, `sales` |
| `plan`                     | text, NOT NULL, default `growth` | Billing takes this over in Phase 6        |
| `trial_ends_at`            | timestamptz, nullable            | created + `TRIAL_DAYS`                    |
| `activated_at`             | timestamptz, nullable            | Section 5                                 |
| `created_by`               | uuid, FK → users, SET NULL       |                                           |
| `created_at`, `updated_at` | timestamptz                      |                                           |

**`workspace_members`**: Phase 3 creates only the owner row here; the team document adds roles and invites.

| Column                     | Type                           | Notes                                           |
| -------------------------- | ------------------------------ | ----------------------------------------------- |
| `workspace_id`             | uuid, FK → workspaces, CASCADE | PK part                                         |
| `user_id`                  | uuid, FK → users, CASCADE      | PK part; indexed on its own for "my workspaces" |
| `role`                     | text, NOT NULL                 | `owner` for now                                 |
| `last_active_at`           | timestamptz                    | Picks the workspace to open after login         |
| `created_at`, `updated_at` | timestamptz                    |                                                 |

### Owned by `features/onboarding`

**`workspace_onboarding`**: one per workspace (`workspace_id` is PK and FK, CASCADE).

| Column                     | Type                     | Notes                                             |
| -------------------------- | ------------------------ | ------------------------------------------------- |
| `workspace_id`             | uuid, PK                 |                                                   |
| `first_goal`               | text, NOT NULL           | `offers`, `support`, `automate`                   |
| `sample_data`              | bool, NOT NULL           | True when started with "Explore with sample data" |
| `business_done_at`         | timestamptz, nullable    | Step 1                                            |
| `whatsapp_skipped_at`      | timestamptz, nullable    | Step 2 skipped                                    |
| `test_message_sent_at`     | timestamptz, nullable    | Step 3, set when Meta reports `delivered`         |
| `completed_at`             | timestamptz, nullable    | Setup finished or skipped; dashboard from now on  |
| `reminder_count`           | int, NOT NULL, default 0 | Abandoned-setup emails sent (max 2)               |
| `created_at`, `updated_at` | timestamptz              |                                                   |

`current_step` is computed, not stored: `business` until `business_done_at`, then `whatsapp` until a number is connected or skipped, then `test_message`, then `done` once `completed_at` is set.

**`onboarding_member_state`**: per member UI state.

| Column                | Type                  | Notes                        |
| --------------------- | --------------------- | ---------------------------- |
| `workspace_id`        | uuid, FK, CASCADE     | PK part                      |
| `user_id`             | uuid, FK, CASCADE     | PK part                      |
| `tour_seen_at`        | timestamptz, nullable | Finished or skipped the tour |
| `checklist_hidden_at` | timestamptz, nullable |                              |

### Owned by `features/whatsapp`

**`whatsapp_accounts`**: one per WhatsApp Business account (WABA) connected to a workspace.

| Column                     | Type                           | Notes                                                                           |
| -------------------------- | ------------------------------ | ------------------------------------------------------------------------------- |
| `id`                       | uuid, PK                       |                                                                                 |
| `workspace_id`             | uuid, FK → workspaces, CASCADE | Indexed                                                                         |
| `waba_id`                  | text, unique                   | Meta's ID. Unique: one WABA belongs to one workspace.                           |
| `business_id`              | text                           | Meta business portfolio ID                                                      |
| `name`                     | text                           |                                                                                 |
| `access_token`             | bytea                          | **Secret.** Business integration token, AES-256-GCM with `TOKEN_ENCRYPTION_KEY` |
| `status`                   | text                           | `active` or `disconnected`                                                      |
| `connected_by`             | uuid, FK → users, SET NULL     |                                                                                 |
| `created_at`, `updated_at` | timestamptz                    |                                                                                 |

**`whatsapp_numbers`**

| Column                     | Type                                  | Notes                                                                                                        |
| -------------------------- | ------------------------------------- | ------------------------------------------------------------------------------------------------------------ |
| `id`                       | uuid, PK                              |                                                                                                              |
| `workspace_id`             | uuid, FK → workspaces, CASCADE        |                                                                                                              |
| `account_id`               | uuid, FK → whatsapp_accounts, CASCADE |                                                                                                              |
| `phone_number_id`          | text, unique                          | Meta's ID                                                                                                    |
| `display_phone_number`     | text                                  | e.g. `+1 415 555 0142`                                                                                       |
| `number_type`              | text                                  | `new`, `coexistence`, `migration`                                                                            |
| `verified_name`            | text                                  | Display name                                                                                                 |
| `name_status`              | text                                  | `pending_review`, `approved`, `declined`, …                                                                  |
| `quality_rating`           | text, nullable                        | `green`, `yellow`, `red`                                                                                     |
| `messaging_limit`          | text, nullable                        | Meta's tier, e.g. `TIER_250`                                                                                 |
| `registration_pin`         | bytea                                 | **Secret.** Two-step verification PIN we set at registration, encrypted. Needed to re-register.              |
| `status`                   | text                                  | `setting_up`, `connected`, `failed`, `disconnected`                                                          |
| `setup_tasks`              | jsonb                                 | `{register, subscribe_webhooks, sync_templates, import_profile}` → `waiting` / `running` / `done` / `failed` |
| `last_error`               | jsonb, nullable                       | Meta error code and our error code. **Never tokens.**                                                        |
| `created_at`, `updated_at` | timestamptz                           |                                                                                                              |

**`whatsapp_test_messages`**: until the Inbox feature owns messages.

| Column                     | Type                                 | Notes                                           |
| -------------------------- | ------------------------------------ | ----------------------------------------------- |
| `id`                       | uuid, PK                             |                                                 |
| `workspace_id`             | uuid, FK, CASCADE                    |                                                 |
| `number_id`                | uuid, FK → whatsapp_numbers, CASCADE |                                                 |
| `to_phone`                 | text                                 | E.164                                           |
| `wamid`                    | text, unique, nullable               | Meta's message ID                               |
| `status`                   | text                                 | `queued`, `sent`, `delivered`, `read`, `failed` |
| `error_code`               | text, nullable                       |                                                 |
| `created_at`, `updated_at` | timestamptz                          |                                                 |

### Migrations

| File                    | Contents                                                          |
| ----------------------- | ----------------------------------------------------------------- |
| `00005_email_codes.sql` | `email_codes`; `users.terms_accepted_at`, `terms_version`         |
| `00006_workspaces.sql`  | `workspaces`, `workspace_members`                                 |
| `00007_onboarding.sql`  | `workspace_onboarding`, `onboarding_member_state`                 |
| `00008_whatsapp.sql`    | `whatsapp_accounts`, `whatsapp_numbers`, `whatsapp_test_messages` |

Each with a working `Down`.

---

## 4. API endpoints

All under `/v1`. "Auth" means a valid access token. "Member" means the workspace middleware found a membership; "Owner" means role `owner` (later: `owner` or `admin`).

### Auth changes

| Method and path                  | Auth | What it does                                                                                                                                                                                                                       |
| -------------------------------- | ---- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `POST /auth/signup`              | –    | Adds `accept_terms` (must be `true`, else `422`). Emails a 6-digit code instead of a link. Response unchanged: same 202 whether or not the email exists.                                                                           |
| `POST /auth/verify-email/code`   | –    | `email`, `code`. Right code → marks verified, creates a session, returns tokens/cookies like login. Wrong email or wrong code → the same `400 invalid_code`. 5 wrong tries on a code → `429 too_many_attempts`, request a new one. |
| `POST /auth/verify-email/resend` | –    | Sends a new code. Always 202 with `retry_after_seconds: 60`. Max 5 per hour per email.                                                                                                                                             |
| `POST /auth/verify-email`        | –    | Kept for links already sent before the switch; removed later.                                                                                                                                                                      |

### Workspaces

| Method and path    | Auth   | What it does                                                                                                                                                                                         |
| ------------------ | ------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /workspaces`  | Auth   | My workspaces with `role`, `last_active_at` and an onboarding summary (`current_step`, `completed`, `activated`), so the frontend can route after login.                                             |
| `POST /workspaces` | Auth   | `first_goal`, `start_mode` (`setup` or `sample`). Creates the workspace (default name), owner membership, onboarding row and trial in one transaction; schedules reminders. Verified email required. |
| `GET /workspace`   | Member | Business details, plan, `trial_ends_at`, `activated_at`.                                                                                                                                             |
| `PATCH /workspace` | Owner  | Edit business details later from Settings. Same fields as setup step 1.                                                                                                                              |

Limit: one workspace per user while it is still in trial and not activated (question 10).

### Onboarding

| Method and path                            | Auth   | What it does                                                                                                                                                                                        |
| ------------------------------------------ | ------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /workspace/onboarding`                | Member | `current_step`, the 3 steps with status, `first_goal`, `sample_data`, the checklist (6 items, done flags, links), `trial_ends_at`, `activated`, and this member's `tour_seen` / `checklist_hidden`. |
| `PUT /workspace/onboarding/business`       | Owner  | Step 1 fields. Saves them on the workspace and sets `business_done_at`. Can be called again (Back button).                                                                                          |
| `POST /workspace/onboarding/skip-whatsapp` | Owner  | Only on step 2 with no connected number. Sets `whatsapp_skipped_at` and `completed_at`.                                                                                                             |
| `POST /workspace/onboarding/complete`      | Owner  | From step 3 ("Go to dashboard"). Requires a connected number. Sets `completed_at`. The test message is not required (question 5).                                                                   |
| `PATCH /workspace/onboarding/me`           | Member | `tour_seen`, `checklist_hidden`.                                                                                                                                                                    |

### WhatsApp

| Method and path                                             | Auth   | What it does                                                                                                                                                                                                                           |
| ----------------------------------------------------------- | ------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `GET /whatsapp/signup-config`                               | Auth   | Public values the frontend needs to open Embedded Signup: `app_id`, `config_id`, `graph_version`.                                                                                                                                      |
| `POST /workspace/whatsapp/connections`                      | Owner  | `code`, `waba_id`, `phone_number_id`, `number_type`. Exchanges the code for a business token, saves account + number (`setting_up`), enqueues `whatsapp_setup`. 202 with the number. Reconnecting an existing WABA replaces its token. |
| `GET /workspace/whatsapp/numbers`                           | Member | Numbers with status, `setup_tasks`, display name and its status, quality, limit, and the `wa.me` link. Polled every 2 s during "Setting up".                                                                                           |
| `GET /workspace/whatsapp/numbers/{numberId}`                | Member | One number.                                                                                                                                                                                                                            |
| `POST /workspace/whatsapp/numbers/{numberId}/test-messages` | Owner  | `to` (E.164). Refused if `to` is the connected number (`422 same_number`) or the number isn't `connected`. Sends `hello_world`. Max 5 per hour per workspace. 202 with the test message.                                               |
| `GET /workspace/whatsapp/test-messages/{messageId}`         | Member | Status, polled until `delivered` or `failed`.                                                                                                                                                                                          |
| `GET /webhooks/whatsapp`                                    | Meta   | Verification handshake: echoes `hub.challenge` when `hub.verify_token` matches.                                                                                                                                                        |
| `POST /webhooks/whatsapp`                                   | Meta   | Checks `X-Hub-Signature-256` against the app secret, answers 200 at once, enqueues processing. Fields used now: `messages` (statuses), `phone_number_name_update`, `phone_number_quality_update`, `account_update`.                    |

Errors this phase adds: `invalid_code`, `too_many_attempts`, `terms_required`, `workspace_limit_reached`, `onboarding_step_invalid`, `whatsapp_code_exchange_failed`, `whatsapp_number_in_use`, `whatsapp_already_connected` (WABA belongs to another workspace), `whatsapp_not_connected`, `same_number`, `whatsapp_disconnected`.

---

## 5. Flows

### Connect a number (Embedded Signup)

1. Frontend loads the Facebook JS SDK with `GET /whatsapp/signup-config` and calls `FB.login` with the `config_id`, `response_type: code`, `override_default_response_type: true`, and `extras.sessionInfoVersion: 3`. Coexistence adds `featureType: whatsapp_business_app_onboarding`.
2. Meta's window posts `WA_EMBEDDED_SIGNUP` messages: `FINISH` (or `FINISH_WHATSAPP_BUSINESS_APP_ONBOARDING`) with `waba_id` and `phone_number_id`, or `CANCEL` with `current_step`. The `FB.login` callback returns the `code`. The frontend checks the message origin is `facebook.com`.
3. `POST …/whatsapp/connections`. In one transaction: exchange the code (`GET /oauth/access_token` with app ID and secret), read the WABA and number from Graph, refuse if the WABA belongs to another workspace, insert account + number, enqueue `whatsapp_setup`.
4. `whatsapp_setup` job, each task idempotent and retried with backoff; `setup_tasks` updated after each:
   - **register**: `POST /{phone_number_id}/register` with a new random 6-digit PIN (skipped for coexistence, which stays registered on the app);
   - **subscribe_webhooks**: `POST /{waba_id}/subscribed_apps`;
   - **sync_templates**: read existing templates (stored by the Templates feature once it exists; counted only for now);
   - **import_profile**: read the business profile and the number's name status, quality and limit.
     All done → `connected`. A task out of retries → `failed` with `last_error`; the UI shows the matching error and "Try again" re-enqueues.
5. The frontend moves from "Setting up" to "Connected" when `status = connected`.

### Test message

1. Insert `whatsapp_test_messages` (`queued`), send the `hello_world` template (`en_US`) through Graph, store the `wamid` (`sent`).
2. Status webhooks update it to `delivered` / `read` / `failed`. `delivered` sets `workspace_onboarding.test_message_sent_at`.
3. A reply from that number arrives as a normal inbound message (Inbox feature; until then it is only logged).

### Activation

`activated_at` is set once, the first time a workspace has a connected number and an outbound message to a real customer is accepted by Meta. Test messages don't count. The Broadcasts and Inbox features call `onboarding.MarkActivated(ctx, workspaceID)` through an interface `internal/app` wires; Phase 3 ships the method and its test, with no caller yet.

### Disconnection

A Graph call failing with an auth error (code 190, or 10 for removed permission), or an `account_update` webhook saying access was removed, sets the account to `disconnected` and its numbers to `disconnected`. `GET /workspace/whatsapp/numbers` reports it; the frontend shows the "WhatsApp is disconnected" banner on every page. Reconnecting runs Embedded Signup again for the same WABA.

---

## 6. Code layout

```
apps/api/internal/
├── features/auth/                # + email codes, terms acceptance
├── features/workspaces/          # workspaces, workspace_members (owner only for now)
├── features/onboarding/          # progress, checklist, member UI state, reminders, activation
│   ├── handler.go  service.go  repository.go  model.go  dto.go  errors.go  routes.go  jobs.go
│   └── queries.sql
├── features/whatsapp/            # connections, numbers, setup job, test messages, webhooks
│   ├── handler.go  service.go  repository.go  model.go  dto.go  errors.go  routes.go  jobs.go
│   ├── graph.go                  # Meta Graph API client (one shared http.Client; token passed per call)
│   ├── webhook.go                # signature check + handler registry by webhook field
│   └── queries.sql
└── platform/middleware/workspace.go   # membership check, workspaceID + role into authctx
```

No feature imports another. Interfaces each side declares, wired in `internal/app`:

| Declared in           | Interface             | Implemented by | Used for                                                      |
| --------------------- | --------------------- | -------------- | ------------------------------------------------------------- |
| `workspaces`          | `OnboardingStarter`   | `onboarding`   | Create the onboarding row in the create-workspace transaction |
| `onboarding`          | `WorkspaceDetails`    | `workspaces`   | Save step 1 fields; read name and trial dates                 |
| `onboarding`          | `ChannelStatus`       | `whatsapp`     | Is a number connected? (step 2, checklist)                    |
| `whatsapp`            | `TestMessageNotifier` | `onboarding`   | Set `test_message_sent_at` on delivery                        |
| `platform/middleware` | `MembershipChecker`   | `workspaces`   | Workspace middleware lookup (cached briefly)                  |

### Factory check (CLAUDE.md section 2)

| Thing                     | Factory?                                         | Why                                                                                                                                                                           |
| ------------------------- | ------------------------------------------------ | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Meta Graph client         | **No**                                           | One implementation; the token is a call argument, so one shared client serves every workspace. Becomes the approved per-workspace factory only if a second provider is added. |
| WhatsApp webhook handlers | Yes (approved: "Webhook / job handler registry") | Chosen by webhook `field`, map filled at startup                                                                                                                              |
| Everything else           | No                                               | Plain constructors                                                                                                                                                            |

CLAUDE.md's secrets rule gains: `email_codes.code_hash`, `whatsapp_accounts.access_token`, `whatsapp_numbers.registration_pin`.

---

## 7. Background jobs (River)

| Job                     | When                                               | What                                                                                           |
| ----------------------- | -------------------------------------------------- | ---------------------------------------------------------------------------------------------- |
| `send_email` (exists)   | On demand                                          | New templates: verification code, setup reminders, trial reminders                             |
| `whatsapp_setup`        | After a connection                                 | Section 5                                                                                      |
| `whatsapp_webhook`      | Per webhook delivery                               | Dispatch by field; idempotent on `wamid` / event ID                                            |
| `onboarding_reminder`   | Scheduled at workspace creation: +24 h and +3 days | Sends only if setup is still unfinished and the owner hasn't opted out; bumps `reminder_count` |
| `trial_reminder`        | Scheduled at workspace creation: day 10 and day 13 | Day 13 includes counts of messages sent. Skipped if a plan was chosen (Phase 6).               |
| `auth_cleanup` (extend) | Nightly                                            | Also delete used and expired `email_codes`                                                     |

---

## 8. Configuration (new environment variables)

| Variable                         | Example          | Notes                                                                    |
| -------------------------------- | ---------------- | ------------------------------------------------------------------------ |
| `EMAIL_CODE_KEY`                 | base64, 32 bytes | HMAC key for verification codes                                          |
| `TERMS_VERSION`                  | `2026-10`        | Stored at sign-up                                                        |
| `TRIAL_DAYS`                     | `14`             |                                                                          |
| `META_APP_ID`, `META_APP_SECRET` |                  | The Meta app with WhatsApp; may be the Facebook Login app (question 11)  |
| `META_ES_CONFIG_ID`              |                  | Embedded Signup configuration ID                                         |
| `META_GRAPH_VERSION`             | `v24.0`          | Pinned                                                                   |
| `META_WEBHOOK_VERIFY_TOKEN`      |                  | For the GET handshake                                                    |
| `WHATSAPP_ENABLED`               | `false` locally  | Off: the WhatsApp routes return 404 and setup step 2 can only be skipped |

`TOKEN_ENCRYPTION_KEY` (Phase 2) also encrypts the WABA token and PIN.

---

## 9. Open questions

Each has a recommendation; the plan above assumes it.

1. **Code instead of link for verification.** The design uses a 6-digit code; Phase 2 built links. _Recommend:_ code only for sign-up (it keeps the user in the same tab, which the flow depends on), keep the link endpoint until old links expire.
2. **Do Facebook sign-ups verify?** Facebook doesn't vouch for emails, so Phase 2 marks them unverified. _Recommend:_ send them the code screen too; Google and Apple skip it.
3. **Which connect UI does setup step 2 use?** The setup screen shows a short version; the journey map links the step to the full 5-step flow (number type, checklist). _Recommend:_ the short version in setup, defaulting to "new number", with a "Using your WhatsApp Business app number?" link that opens the full flow. Same API either way.
4. **Coexistence and migration at launch?** Both need extra Meta setup and app review, and coexistence needs history/contact sync. _Recommend:_ launch with new numbers only; show the other two as "Coming soon".
5. **Is the test message required to finish setup?** _Recommend:_ no. "Go to dashboard" works once a number is connected, so a failed SMS or a user without a second phone isn't stuck.
6. **Full name field.** Our API takes `first_name` and `last_name`. _Recommend:_ the frontend splits on the first space; the API stays as is.
7. **Password rules vs the design's hint.** We require upper, lower, digit and symbol; the design says "a number and a symbol". _Recommend:_ keep our rules and change the hint copy.
8. **"Remember me for 30 days".** Sessions are 30 days already. _Recommend:_ unticked means a browser-session refresh cookie (no `Max-Age`) and a 24-hour server-side session.
9. **Reset link lifetime.** Design says 30 minutes, Phase 2 uses 1 hour. _Recommend:_ change the copy to 1 hour.
10. **Trial abuse.** One person could create many workspaces for many trials. _Recommend:_ one trialing, non-activated workspace per user; more need a plan (Phase 6).
11. **Two goal questions.** Welcome asks offers/support/automate; step 1 asks marketing/support/sales. _Recommend:_ keep both as designed (one drives the dashboard, the other the template suggestions), but confirm with design whether to merge them.
12. **Manual checklist ticks.** The prototype lets users tick items by hand. _Recommend:_ derived from data only; "Hide checklist" covers users who don't care.
13. **Sample data.** Contacts, Inbox and Broadcasts don't exist yet. _Recommend:_ Phase 3 stores the `sample_data` flag; the frontend shows fixture data until those features exist, then we decide whether to seed real rows.
14. **Same Meta app for login and WhatsApp?** _Recommend:_ one Meta app (simpler review) if business verification allows it; otherwise two, which is why the config keeps `META_*` separate from `FACEBOOK_*`.
15. **Reminder emails are marketing-ish.** _Recommend:_ treat them as transactional (they are about the user's own unfinished setup), with an unsubscribe link, and stop after two.
16. **Changing email on the verify screen.** "Change email" goes back to sign-up and creates a second unverified user. _Recommend:_ allow it; the nightly cleanup deletes unverified users with no workspace after 7 days.
17. **Login page copy.** The design mentions two-factor sign-in, which we don't have. _Recommend:_ remove that line until 2FA exists.
18. **Support links.** "Book a free setup call", "Chat with support" and the setup video need real targets (Calendly, a chat widget). Product to provide.

---

## 10. Testing

- **Unit**: current-step computation (every combination), checklist derivation, code generation and HMAC check, routing decision table (section 2) in the frontend.
- **Repository** (real Postgres): every query; `workspace_id` scoping on every tenant table; cascades.
- **HTTP**: each endpoint's success, validation and permission cases; enumeration-safe verify-code and resend; attempt limit; skip/complete refused in the wrong step.
- **Meta**: a fake Graph server (`httptest`) for code exchange, register, subscribe, profile and send, including "number in use" and token errors. Webhook signature valid and forged; duplicate deliveries.
- **Jobs**: `whatsapp_setup` resumes after a failed task; reminders skip when setup is done.
- **End to end** (Playwright, Phase 4): sign-up → code → welcome → step 1 → skip → dashboard banner; and with a mocked Embedded Signup, the full connected path.

---

## 11. Build order

1. Auth: `email_codes`, verify-by-code endpoints, terms acceptance; update the verification email.
2. `workspaces`: tables, create/list/get/patch, workspace middleware.
3. `onboarding`: progress, business step, skip, complete, member state, checklist (items that exist).
4. `whatsapp`: config, connections, setup job, numbers, Graph client fake.
5. Webhooks and test messages.
6. Reminder and trial jobs, email templates.
7. Docs: README, ARCHITECTURE (feature list), CLAUDE.md secrets rule, `.env.example`.

Suggested PRs: 1–3 (account to setup without WhatsApp, usable with `WHATSAPP_ENABLED=false`), 4–5 (WhatsApp), 6–7.

### Before WhatsApp can be tested

- A Meta app with the WhatsApp product, **Tech Provider** onboarding and business verification completed.
- An Embedded Signup configuration (gives `META_ES_CONFIG_ID`).
- App review for `whatsapp_business_management` and `whatsapp_business_messaging` before real customers can connect.
- A public HTTPS URL for the webhook (a tunnel locally).
- A spare phone number that has never been on WhatsApp.

---

## 12. Out of scope for this document

Invites, roles and the rest of team access (own document); the frontend pages (Phase 4); templates, contacts, broadcasts, inbox and chatbots (Phase 5); plans, payment and enforcing the end of the trial (Phase 6); onboarding analytics (funnel events).
