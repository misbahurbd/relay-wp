const MIN = 60_000

/** "just now", "4 min ago", "3 h ago", "Oct 24". */
export function relativeTime(iso: string, now = Date.now()) {
  const diff = now - Date.parse(iso)
  if (diff < 2 * MIN) return 'just now'
  if (diff < 60 * MIN) return `${Math.round(diff / MIN)} min ago`
  if (diff < 24 * 60 * MIN) return `${Math.round(diff / (60 * MIN))} h ago`
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function formatDateTime(iso: string) {
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
    second: '2-digit',
  })
}

export function plural(n: number, one: string, many = `${one}s`) {
  return `${n.toLocaleString()} ${n === 1 ? one : many}`
}

/** "99.8%", or "100%" without a needless decimal. */
export function percent(part: number, total: number) {
  if (total === 0) return '—'
  const p = (part / total) * 100
  return `${p === 100 || p === 0 ? p.toFixed(0) : p.toFixed(1)}%`
}
