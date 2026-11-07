import type { CreatedIds } from '../../api/plan-publish'
import type { PlanDraft, PlanEditorMode } from './plan-draft'

/**
 * A plan publish that has started, kept in localStorage so a reload reopens the editor and
 * finishes it. The draft carries its publish key, so even steps whose reply was lost find what
 * the server already created instead of creating it twice.
 */
export type SavedPublish = {
  mode: Extract<PlanEditorMode, { kind: 'create' | 'version' }>
  draft: PlanDraft
  ids: CreatedIds
}

const KEY = 'relay.crm.plan-publish'

export function loadPublish(): SavedPublish | null {
  try {
    const raw = localStorage.getItem(KEY)
    return raw ? (JSON.parse(raw) as SavedPublish) : null
  } catch {
    return null
  }
}

export function savePublish(saved: SavedPublish) {
  try {
    localStorage.setItem(KEY, JSON.stringify(saved))
  } catch {
    // Storage is off (a private window): the publish still runs, it just can't resume.
  }
}

export function clearPublish() {
  try {
    localStorage.removeItem(KEY)
  } catch {
    // Nothing was stored.
  }
}

/** The saved publish when it belongs to mode: the same new plan, or the same version's successor. */
export function savedFor(mode: PlanEditorMode): SavedPublish | null {
  const saved = loadPublish()
  if (!saved || saved.mode.kind !== mode.kind) return null
  if (
    mode.kind === 'version' &&
    saved.mode.kind === 'version' &&
    saved.mode.from.id !== mode.from.id
  ) {
    return null
  }
  return saved
}
