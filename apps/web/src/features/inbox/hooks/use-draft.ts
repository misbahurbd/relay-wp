import { useCallback, useState } from 'react'

const KEY = 'relay.inbox.draft.'

function read(conversationId: string) {
  try {
    return sessionStorage.getItem(KEY + conversationId) ?? ''
  } catch {
    return ''
  }
}

/**
 * The composer's text for one chat, kept for this tab (sessionStorage) so switching chats or
 * reloading doesn't lose it. Storage can be unavailable; the draft then lives in memory only.
 */
export function useDraft(conversationId: string) {
  const [text, setText] = useState(() =>
    typeof window === 'undefined' ? '' : read(conversationId),
  )
  const update = useCallback(
    (next: string) => {
      setText(next)
      try {
        if (next) sessionStorage.setItem(KEY + conversationId, next)
        else sessionStorage.removeItem(KEY + conversationId)
      } catch {
        // Private mode or blocked storage: keep the draft in memory.
      }
    },
    [conversationId],
  )
  return [text, update] as const
}
