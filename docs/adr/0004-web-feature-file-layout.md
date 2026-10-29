# 0004. Web feature files named by role and grouped by area

- **Status:** accepted
- **Date:** 2026-10-29

## Context

Every web feature had files named only after the feature: `api/team.ts`, `schemas/team.ts`, `api/auth.ts`. With several open, editor tabs and search results all read `team.ts` or `auth.ts`, so finding the right one meant reading the folder path. The team feature also put ten unrelated components in one flat `components/` folder next to a 558-line `parts.tsx` that mixed buttons, dialogs, avatars, date formatting and log wording, and a 491-line `api/team.ts` that held keys, queries and mutations together.

## Decision

Inside `apps/web/src/features/<feature>/`:

| Folder        | Files                                                                                       |
| ------------- | ------------------------------------------------------------------------------------------- |
| `api/`        | `<feature>.api.ts`; split into `<feature>.keys.ts`, `.queries.ts`, `.mutations.ts` when big |
| `schemas/`    | `<feature>.schema.ts`                                                                       |
| `hooks/`      | `use-<thing>.ts` (unchanged)                                                                |
| `components/` | `<area>/<component>.tsx` once the feature has 2+ areas; `kit/` for pieces the areas share   |
- `kit/` has one small file per piece (`btn.tsx`, `modal.tsx`, `format.ts`) and no barrel, so imports show where each piece lives.
- A feature with one area keeps a flat `components/`.
- `index.ts` stays the only way in from outside, and its exports did not change.

Applied now:

- auth: `sign-in/`, `recovery/`, `verify/`, `kit/`
- onboarding: `setup/`, `welcome/`, `dashboard/`
- team: `members/`, `teams/`, `roles/`, `invite/`, `log/`, `kit/`; `api/team.ts` became `team.keys.ts`, `team.queries.ts`, `team.mutations.ts`; `components/parts.tsx` became ten `kit/` files.
- workspaces: `api/workspaces.api.ts`.

## Consequences

- File names say which feature and which role they belong to, and big files no longer mix unrelated code.
- Imports inside a feature are a little longer (`../kit/btn`).
- New components must pick an area folder; when none fits, that is a sign of a new area or a new feature.
