import { useSyncExternalStore } from 'react'

// Whether the live event stream is connected. One stream per tab (use-inbox-events), read by
// every list and thread query to decide whether to poll instead (AD-8).
let connected = false
const listeners = new Set<() => void>()

export function setLiveConnected(next: boolean) {
  if (next === connected) return
  connected = next
  for (const l of listeners) l()
}

function subscribe(listener: () => void) {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

/** True while live updates flow; false means queries should poll. Server render: not connected. */
export function useLiveConnected() {
  return useSyncExternalStore(
    subscribe,
    () => connected,
    () => false,
  )
}
