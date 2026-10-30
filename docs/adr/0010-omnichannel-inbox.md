# 0010. The inbox is omnichannel: channel-neutral data, a channel registry, contact identities

- Status: Accepted
- Date: 2026-10-30
- Context: [PHASE-7-INBOX.md](../PHASE-7-INBOX.md)

## Context

The Team Inbox ships with WhatsApp, but Facebook Messenger, Instagram and other channels will follow.

The first build of slice A was WhatsApp-shaped:

- conversations keyed by `number_id`;
- messages deduplicated on `wamid`;
- a sender with WhatsApp's methods;
- the 24-hour window and templates hard-coded;
- contacts identified only by phone.

Messenger and Instagram users have page-scoped IDs and no phone number. Their windows and features differ: a 24-hour window with message tags, no templates, different media limits.

None of this data exists in production yet, so this is the cheapest moment to choose the seams.

## Decision

1. **Channel-neutral inbox data.**
   - `conversations` gets `channel_type` (`whatsapp` | `messenger` | `instagram`) and `channel_id`. `channel_id` is the provider's own account row: `whatsapp_numbers.id` today, a page or IG account row later. It has no FK, because the reference is polymorphic: the type says which table. Chats outlive a disconnected channel and stay readable.
   - `messages.wamid` becomes `external_id`, `UNIQUE (workspace_id, external_id)`, used for idempotent ingest and status matching on every channel.
2. **A channel registry in the inbox.**
   - The inbox declares a `Channel` interface: send text, media, template and reaction; mark read; upload and fetch media; and `Capabilities()`.
   - Capabilities cover the window length (0 = always open), templates yes/no, reactions, read receipts, allowed media kinds and sizes, and the maximum text length.
   - `internal/app` fills a `map[channelType]Channel` at startup. WhatsApp is the only entry today. The service picks the adapter by the conversation's `channel_type`.
   - Every rule that differed by channel now reads capabilities, never a channel name: the window check (AD-6), template availability, media validation (AD-11) and reactions.
   - This is a "registry keyed by a value, filled at startup" and is added to CLAUDE.md's approved factory points.
3. **Contact identities.**
   - Contacts gains `contact_identities (workspace_id, contact_id, channel_type, external_id, handle)` with PK `(workspace_id, channel_type, external_id)`.
   - Inbound messages resolve the sender through `FindOrCreateByIdentity`. For WhatsApp, the external ID is the E.164 phone. A missing identity links to the existing contact with that phone, or creates one, so no backfill is needed. A later phone edit doesn't lose the chat.
4. **The API exposes channel and capabilities**, not WhatsApp fields. A conversation carries `channel {type, id, name, handle}` and `capabilities`, and the web renders the window bar, template button, reactions and attachment kinds from those.

## Not decided here (when the second channel lands)

- `contacts.phone` becomes nullable, because Messenger and Instagram contacts have no phone.
- Team access generalizes from "numbers" to "channels": `member_numbers`, `teams.number_id` and the invitation numbers. Until then, a member's number list is also their WhatsApp channel list, and other channel types are visible to anyone in scope.
- The assignment condition "WhatsApp number" becomes "Channel" (slice B ships it as a channel condition from the start).
- Each new channel brings its own feature package (Meta webhooks, its Graph client), an adapter that implements `Channel`, and one registry entry.

## Consequences

- Adding a channel touches no inbox schema or service code, only a new adapter, its webhook feature and the registry entry.
- One more lookup per inbound message (the identity). It's an indexed primary-key read.
- The polymorphic `channel_id` has no database-enforced integrity. The service checks the channel through its adapter when sending.
