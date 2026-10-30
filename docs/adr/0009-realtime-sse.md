# 0009. Live updates over SSE, fanned out with Postgres LISTEN/NOTIFY

- Status: Proposed
- Date: 2026-10-30
- Context: [PHASE-7-INBOX.md](../PHASE-7-INBOX.md) AD-8

## Context

The Team Inbox must show new messages, status ticks and assignment changes to every agent who can see a chat within about 2 s, without a reload.

- The API runs as several stateless instances (0.5 vCPU / 1 GB each), so an event written by one instance must reach browsers connected to any other.
- There is no broker or Redis today, and ARCHITECTURE asks us not to add infrastructure before it's needed.
- Peak load is about 3,000 open connections and fewer than 100 events a second.

## Decision

- **Transport:** Server-Sent Events from `GET /v1/workspace/inbox/events`. It uses the same cookie auth and middleware as every route, sends a heartbeat every 20 s, and has no write deadline. The browser's `EventSource` reconnects automatically.
- **Fan-out:**
  - Writers call `pg_notify('inbox_events', payload)` inside their transaction, so a notification is sent only if the write commits.
  - Each API instance runs one `platform/realtime` hub. The hub holds one `LISTEN` connection from the direct (unpooled) pool and passes events to that instance's subscribers after a per-member scope filter.
- **Payloads** are signals (`{ws, conversation_id, kind, team_id, assignee_id, number_id}`), never content. Clients refetch through the normal scoped endpoints.

## Alternatives

| Option                       | Why not now                                                                                                                                  |
| ---------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------- |
| WebSocket                    | We only need server-to-client. It needs its own auth handshake, proxy upgrades and reconnect logic.                                          |
| Polling only                 | ~3,000 clients every 2–5 s is 600–1,500 req/s of mostly empty reads, and it still has up to 5 s of lag. It's kept only as the fallback.      |
| Redis pub/sub                | New infrastructure. Publish isn't tied to the transaction commit.                                                                            |
| Push data (patches) over SSE | Every event would need a per-member render and scope check, events would have to be ordered, and the client would hold two sources of truth. |

## Consequences

- No new infrastructure. An event is delivered only once its transaction commits.
- Every instance receives every workspace's events and filters them in memory. This is fine below ~1,000 events/s; past that, shard the channels by workspace hash or move to Redis.
- Each instance holds one direct connection for LISTEN (count it in the connection budget), and the 30 s server write timeout is lifted for this one route.
- Rolling deploys drop streams. Clients reconnect and refetch, so they miss nothing that matters.
- Other features (contacts, broadcasts) can reuse the hub with their own channel.
