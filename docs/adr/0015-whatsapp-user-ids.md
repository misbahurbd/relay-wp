# 0015. WhatsApp senders are identified by business-scoped user ID; phone is optional

- **Status:** accepted
- **Date:** 2026-11-06

## Context

WhatsApp usernames shipped in 2026. Every Cloud API webhook now carries a business-scoped user ID (BSUID) for the user, `messages[].from_user_id` and `contacts[].user_id`, for example `BD.2294857751239750`. The phone (`messages[].from`, `contacts[].wa_id`) is only included when it is "available": the business number messaged or called that user in the last 30 days, heard from them in the last 30 days, or has them in its contact book. Otherwise a user with a username arrives with a BSUID and no phone.

Relay assumed a phone for every WhatsApp sender ([ADR 0010](0010-omnichannel-inbox.md): "for WhatsApp, the external ID is the E.164 phone"). A username-only message fails the `whatsapp_webhook` job with `inbox: find contact: invalid_phone`, and River retries it until it gives up. The message never reaches the Inbox.

Facts from Meta's BSUID and webhook reference pages (read 2026-11-06):

- Format: an ISO 3166 alpha-2 country code, a period, then up to 128 alphanumeric characters (`US.13491208655302741918`). It must be sent back exactly as received.
- Scope: unique per business portfolio and user. It changes if the user changes their phone number (Meta sends a `system` webhook). Changing a username doesn't change it.
- Sending: `POST /{phone_number_id}/messages` takes `"recipient": "<BSUID>"` in place of `"to"`. If both are given, `to` wins. Every message type works except one-tap, zero-tap and copy-code authentication templates. Error `131062` means a BSUID recipient isn't supported for that message.
- Statuses: `statuses[].recipient_user_id` is always the BSUID; `recipient_id` (phone) is omitted when the phone is unavailable. Statuses are matched by wamid, which Relay already does.
- Meta recommends sending to the phone when you have it, so phones keep arriving in webhooks.

## Decision

1. **A WhatsApp sender can have two identities.** `contact_identities` already keys senders by `(workspace_id, channel_type, external_id)`. For `whatsapp`, `external_id` is either the E.164 phone (starts with `+`) or the BSUID (contains a `.`). The two never collide, so no schema change is needed there. The identity's `handle` stores the WhatsApp `username` when Meta sends one.
2. **Lookup order for an inbound WhatsApp sender:** the BSUID identity, then the phone identity, then a contact with that phone. Whichever identities are missing are linked to the contact found. If neither identity nor phone matches, a new contact is created, with no phone if Meta sent none. A later message that brings the phone links it to the same contact and fills `contacts.phone` when it is empty.
3. **`contacts.phone` becomes nullable**, as ADR 0010 foresaw. `UNIQUE (workspace_id, phone)` is unchanged (NULLs don't collide). Contacts created by hand, by import, by the API or by commerce events still require a phone; only the inbox can create a contact without one.
4. **Sending prefers the phone.** A message goes to the contact's phone (WhatsApp phone identity, else `contacts.phone`) and falls back to the BSUID only when there is no phone. The WhatsApp Graph client sends a BSUID as `recipient` and a phone as `to`; the channel decides by the address's shape, so the inbox's `Channel` interface doesn't change.
5. **Broadcasts skip contacts without a phone** for now: they are never snapshotted as recipients, and the audience preview counts them as `no_phone`. Marketing to BSUIDs works at Meta, but Meta recommends phones, and broadcast variables offer `phone`. A later change can include them.
6. **The API and web show a phone-less contact by name and `@username`.** `phone` stays a string in contact and conversation DTOs and is empty when unknown (read queries `COALESCE` it, so generated Go and TypeScript types don't change). A contact carries `whatsapp_username` and a conversation's contact carries `username` when known; search matches usernames.

## Consequences

- Username-only customers reach the Inbox, and replies, automations and quick replies work for them through the BSUID.
- One user can become two contacts when they first write without a phone (BSUID-only contact) and are also imported by phone. A later message that carries both IDs links the BSUID to whichever contact it finds first. Merging duplicates is left out of this decision.
- A workspace that connects numbers from two business portfolios sees two BSUIDs for one person. Without a phone they become two contacts.
- Code that reads `contacts.phone` must handle NULL: search, export, segments (`phone starts with` never matches a NULL), broadcast audience, and the web.
- When a user changes their phone, Meta issues a new BSUID; the next message creates a new identity. Handling the `system` webhook to relink is left for later.
