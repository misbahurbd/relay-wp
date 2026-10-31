const count = new Intl.NumberFormat()

export const formatCount = (n: number) => count.format(n)

/** "Oct 28, 3:40 PM" in the viewer's time zone. */
export function formatWhen(iso: string | null | undefined) {
  if (!iso) return '—'
  return new Date(iso).toLocaleString(undefined, {
    month: 'short',
    day: 'numeric',
    hour: 'numeric',
    minute: '2-digit',
  })
}

/** A datetime-local value (local time, no zone) for an instant, or for an hour from now. */
export function localInputValue(iso?: string | null) {
  const d = iso ? new Date(iso) : new Date(Date.now() + 60 * 60 * 1000)
  const pad = (n: number) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`
}
