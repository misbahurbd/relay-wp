function time(d: Date) {
  return d.toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

/** "Today, 11:04 AM", "Yesterday, 6:40 PM" or "Oct 26, 2026". */
export function formatUpdated(iso: string, now = new Date()) {
  const d = new Date(iso)
  if (d.toDateString() === now.toDateString()) return `Today, ${time(d)}`
  const yesterday = new Date(now)
  yesterday.setDate(now.getDate() - 1)
  if (d.toDateString() === yesterday.toDateString()) return `Yesterday, ${time(d)}`
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric', year: 'numeric' })
}

export function formatClock(d = new Date()) {
  return time(d)
}

/** "1.2 MB", "340 KB". */
export function formatBytes(bytes: number) {
  if (bytes >= 1024 * 1024) return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
  return `${Math.max(1, Math.round(bytes / 1024))} KB`
}
