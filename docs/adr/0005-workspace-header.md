# 0005. Workspace in a header, not the URL

- **Status:** accepted
- **Date:** 2026-10-29

## Context

Workspace-scoped routes took the workspace from the path: `/v1/workspaces/{workspaceId}/...`. Every client had to build URLs around the ID, and route paths, rate-limit keys and logs all carried it. The mobile app needs the same API as the web app, so whatever we choose has to work without a browser.

We looked at three places for the workspace:

- **Path** (what we had): explicit, but every route and client URL repeats it.
- **Cookie:** browser-only in practice. Mobile clients would need a cookie jar, two web tabs on different workspaces would overwrite each other, and a cookie that changes what a request does widens the CSRF surface.
- **Header:** works the same for web, mobile and scripts, is per request (so tabs stay independent), and isn't sent cross-site by a browser on its own.

## Decision

Workspace-scoped routes live under `/v1/workspace/...` (singular: "the workspace this request names") and read the workspace from the `X-Workspace-ID` header.

- `platform/middleware.RequireWorkspace` reads `middleware.WorkspaceHeader`. A missing header gets 422 `workspace_required`; an unknown or malformed ID, or one the user isn't a member of, gets 404 `workspace_not_found`, as before.
- Responses carry `Vary: X-Workspace-ID`, and CORS allows the header.
- DTOs declare the header (`header:"X-Workspace-ID" required:"true"`), so the OpenAPI spec and the generated client require it per call. Handlers still read `in.WorkspaceID`.
- The web app passes `headers: inWorkspace(workspaceId)` (`#/lib/api`) on each call rather than setting a global default, so a stale default can never send a request to the wrong workspace.
- Web page URLs don't carry the workspace either (`/`, `/setup`, `/team`, …). The `_app/_workspace` layout route picks it: this tab's choice in the sidebar's workspace switcher (sessionStorage), else the last choice in any tab (localStorage), else the most recently used. It hands it to pages as `context.workspaceId`. Switching goes to the new workspace's dashboard.
- Email links name their workspace once as `?workspace=<id>` (e.g. `/setup?workspace=…`); the layout selects it and removes it from the URL. A workspace the user isn't in falls back as above.
- Routes that aren't about one workspace (`/v1/workspaces` list and create, `/v1/me`, `/v1/invitations/*`, auth) take no header.

## Consequences

- Clients set one header instead of building paths; web and mobile both keep a current workspace and send it.
- A copied page URL opens in whichever workspace the reader has selected, not necessarily the sender's.
- A route is workspace-scoped exactly when it's under `/v1/workspace`, which is easy to check in review.
- The workspace isn't in access logs' paths any more; log it from `authctx` where it matters.
- This was a breaking API change; there were no external clients yet.
