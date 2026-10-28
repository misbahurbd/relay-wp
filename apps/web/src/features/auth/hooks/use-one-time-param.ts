import { useEffect, useState } from 'react'

/**
 * Keeps a one-time value from the URL (an emailed token, a sign-in step's token)
 * and takes it out of the address bar, so it isn't left in browser history or
 * visible over a shoulder. Returns the value from the first render.
 *
 * history.replaceState rather than a router navigation: the page keeps showing
 * what it showed, and the router reads the URL afresh on the next navigation.
 */
export function useOneTimeParam(name: string, value: string | undefined) {
  const [kept] = useState(value)
  useEffect(() => {
    const url = new URL(window.location.href)
    if (!url.searchParams.has(name)) return
    url.searchParams.delete(name)
    window.history.replaceState(window.history.state, '', url)
  }, [name])
  return kept
}

/** A same-site path to go to after signing in, or undefined for anything else. */
export function safeRedirect(value: unknown) {
  return typeof value === 'string' && /^\/(?![/\\])/.test(value) ? value : undefined
}
