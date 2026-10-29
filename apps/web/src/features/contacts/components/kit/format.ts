const DAY = 86_400_000

export const formatNumber = (n: number) => n.toLocaleString()

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

/** "Today, 2:41 PM", "Yesterday", "Mon, Oct 26", "Oct 9" or "Oct 9, 2025", like the design. */
export function lastMessageLabel(iso: string | null, now = Date.now()) {
  if (!iso) return '—'
  const t = Date.parse(iso)
  const d = new Date(t)
  const today = new Date(now)
  if (d.toDateString() === today.toDateString()) return `Today, ${formatTime(iso)}`
  if (d.toDateString() === new Date(now - DAY).toDateString()) return 'Yesterday'
  if (now - t < 7 * DAY && now - t > 0)
    return d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })
  if (d.getFullYear() === today.getFullYear())
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  return formatDate(iso)
}

export function formatBytes(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

export const plural = (n: number, one: string, many = `${one}s`) =>
  `${formatNumber(n)} ${n === 1 ? one : many}`
