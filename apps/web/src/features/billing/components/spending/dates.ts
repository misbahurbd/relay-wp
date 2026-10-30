// Dates on this page are calendar days (YYYY-MM-DD), so they're read and printed in UTC: a day
// never shifts with the viewer's time zone.

function utc(iso: string) {
  return new Date(`${iso.slice(0, 10)}T00:00:00Z`)
}

/** "Oct 28". */
export function shortDay(iso: string) {
  return utc(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' })
}

/** "October". */
export function monthName(iso: string, month: 'long' | 'short' = 'long') {
  return utc(iso).toLocaleDateString(undefined, { month, timeZone: 'UTC' })
}

/** The month's last day, YYYY-MM-DD. */
export function monthEnd(iso: string) {
  const d = utc(iso)
  return new Date(Date.UTC(d.getUTCFullYear(), d.getUTCMonth() + 1, 0)).toISOString().slice(0, 10)
}
