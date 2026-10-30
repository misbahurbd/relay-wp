import { useState } from 'react'
import type { Member, QuickReply } from '../types'

const MENTION = /(^|\s)@([^\s@]*)$/

export type SuggestionState =
  | { kind: 'none' }
  | { kind: 'slash'; items: QuickReply[] }
  | { kind: 'mention'; items: Member[]; start: number }

/**
 * What the composer suggests for the text before the caret: quick replies after a leading "/"
 * (reply mode), members after "@" (note mode). Esc dismisses until the text changes again.
 */
export function useSuggestions({
  text,
  caret,
  mode,
  quickReplies,
  members,
  meId,
}: {
  text: string
  caret: number
  mode: 'reply' | 'note'
  quickReplies: QuickReply[]
  members: Member[]
  meId: string
}) {
  const [dismissedFor, setDismissedFor] = useState<string | null>(null)
  const [active, setActive] = useState(0)
  const key = `${mode}|${text}|${caret}`

  let state: SuggestionState = { kind: 'none' }
  if (dismissedFor !== key) {
    if (mode === 'reply' && text.startsWith('/') && !text.includes('\n')) {
      const q = text.slice(1).trim().toLowerCase()
      state = {
        kind: 'slash',
        items: quickReplies.filter(
          (r) => !q || r.shortcut.includes(q) || r.title.toLowerCase().includes(q),
        ),
      }
    } else if (mode === 'note') {
      const match = MENTION.exec(text.slice(0, caret))
      if (match) {
        const q = match[2].toLowerCase()
        state = {
          kind: 'mention',
          start: caret - match[2].length - 1,
          items: members
            .filter((m) => m.id !== meId && m.name.toLowerCase().includes(q))
            .slice(0, 8),
        }
      }
    }
  }

  const count = state.kind === 'none' ? 0 : state.items.length
  const index = count === 0 ? 0 : Math.min(active, count - 1)

  return {
    state,
    active: index,
    move: (delta: number) => count > 0 && setActive((index + delta + count) % count),
    reset: () => setActive(0),
    dismiss: () => {
      setDismissedFor(key)
      setActive(0)
    },
  }
}
