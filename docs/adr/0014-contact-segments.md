# 0014. Contact segments

- **Status:** accepted
- **Date:** 2026-10-31

## Context

The Broadcasts design picks an audience from tags, uploaded lists and **segments**: saved rules such as "opted in, and last wrote more than 30 days ago" that update on their own. The Segments design builds them from groups of conditions (each group matches ALL or ANY of its conditions; groups are joined by OR) and shows a live count and sample.

Every other query in the API is static sqlc. A rule set the user builds can't be: its shape varies. The design also lists commerce conditions (last order, total spent, order count, abandoned cart) and broadcast engagement, but Relay keeps no orders or carts per contact (commerce data only reaches automation runs), and engagement lives in broadcasts' tables.

## Decision

1. **Segments belong to contacts.** `contact_segments(id, workspace_id, name, description, rules jsonb, …)`, served under `/v1/workspace/contact-segments` (beside `/contact-tags`; `/contacts/segments` would collide with `/contacts/{contactId}`) with `contacts.view` to read and `contacts.edit` to change. The web page is a Segments tab beside All contacts.
2. **Rules compile to one parameterised `WHERE`** in `contacts/repository/segments_match.go`, the second hand-written SQL in the API after team's partition DDL. Fields and operators come from a fixed Go catalog; values only ever travel as `$n` arguments, never in the SQL text. The rules are validated in the model before they reach the builder, and the builder refuses anything outside the catalog.
3. **Fields, v1:** tag (is / is not), opt-in status, last message (within / more than N days), added to contacts (within / more than N days), source, language, phone country code (starts with), custom field (is / contains). At most 5 groups of 10 conditions.
4. **Not in v1:** commerce conditions and broadcast engagement, until orders and carts are stored per contact (a commerce phase) and engagement has a contacts-side view. The catalog is where they will be added.
5. **Broadcasts use segments by ID.** `Audience.SegmentIDs` (at most 10). When counting or taking the audience, broadcasts asks contacts for the members through a small interface (`Segments.Members`) and passes their IDs to the snapshot query with the tags. Opt-in and the recent-marketing rule are still applied by broadcasts, so a segment can't widen consent.
6. **A segment is evaluated when used.** The audience is the segment's members at the moment sending starts; nothing is cached.

## Consequences

- One builder to keep safe: every new field is a catalog entry plus a `case` in the builder, with a test.
- Counting runs the rules once per segment on each list load; fine at today's size (≤50 segments, tens of thousands of contacts). At 10× it needs a cached count.
- Members are passed to the snapshot as a UUID array; at 100k+ members that query should move to a temporary table.
