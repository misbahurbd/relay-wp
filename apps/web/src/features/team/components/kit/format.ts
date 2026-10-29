const MIN = 60_000

export function formatDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, {
    month: 'short',
    day: 'numeric',
    year: 'numeric',
  })
}

export function formatShortDate(iso: string) {
  return new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

export function formatTime(iso: string) {
  return new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

export function formatDateTime(iso: string) {
  return `${formatShortDate(iso)}, ${formatTime(iso)}`
}

/** "Now", "12 min ago", "3 h ago", "Yesterday, 6:40 PM", "Oct 24". */
export function relativeTime(iso: string, now = Date.now()) {
  const t = Date.parse(iso)
  const diff = now - t
  if (diff < 2 * MIN) return 'Now'
  if (diff < 60 * MIN) return `${Math.round(diff / MIN)} min ago`
  const d = new Date(t)
  if (d.toDateString() === new Date(now).toDateString())
    return `${Math.round(diff / (60 * MIN))} h ago`
  if (d.toDateString() === new Date(now - 86_400_000).toDateString())
    return `Yesterday, ${formatTime(iso)}`
  return formatShortDate(iso)
}

/** "Chrome · macOS" from a user agent. */
export function deviceLabel(ua: string) {
  if (!ua) return ''
  const browser = /Edg\//.test(ua)
    ? 'Edge'
    : /Firefox\//.test(ua)
      ? 'Firefox'
      : /Chrome\//.test(ua)
        ? 'Chrome'
        : /Safari\//.test(ua)
          ? 'Safari'
          : 'Browser'
  const os = /iPhone|iPad/.test(ua)
    ? 'iPhone'
    : /Android/.test(ua)
      ? 'Android'
      : /Mac OS X/.test(ua)
        ? 'macOS'
        : /Windows/.test(ua)
          ? 'Windows'
          : /Linux/.test(ua)
            ? 'Linux'
            : ''
  return [browser, os].filter(Boolean).join(' · ')
}

// ---------- access log wording ----------
