import { useCallback, useRef } from 'react'

/**
 * One Idempotency-Key per purchase intent. The key stays the same while `scope` does, so a
 * retried request returns the same checkout instead of starting a second one; `renew` makes a
 * fresh key once the last attempt is settled and the member starts over.
 */
export function useIdempotencyKey(scope: string) {
  const ref = useRef<{ scope: string; key: string } | null>(null)
  if (ref.current?.scope !== scope) ref.current = { scope, key: crypto.randomUUID() }
  const renew = useCallback(() => {
    ref.current = { scope, key: crypto.randomUUID() }
  }, [scope])
  return { key: ref.current.key, renew }
}
