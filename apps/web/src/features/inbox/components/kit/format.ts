const DAY = 86_400_000

function startOfDay(d: Date) {
  return new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime()
}

/** Whole days between two dates' local midnights (0 = same day, 1 = b is the day after a). */
function dayDiff(a: Date, b: Date) {
  return Math.round((startOfDay(b) - startOfDay(a)) / DAY)
}

export function formatClock(iso: string | Date) {
  return new Date(iso).toLocaleTimeString(undefined, { hour: 'numeric', minute: '2-digit' })
}

/** List time: "2:41 PM" today, "Yesterday", "Mon" this week, then "Oct 21". */
export function formatListTime(iso: string, now: number) {
  const d = new Date(iso)
  const days = dayDiff(d, new Date(now))
  if (days <= 0) return formatClock(d)
  if (days === 1) return 'Yesterday'
  if (days < 7) return d.toLocaleDateString(undefined, { weekday: 'short' })
  return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
}

/** Day separator in the thread: "Today", "Yesterday", or "Mon, Oct 21". */
export function formatDay(iso: string, now: number) {
  const d = new Date(iso)
  const days = dayDiff(d, new Date(now))
  if (days === 0) return 'Today'
  if (days === 1) return 'Yesterday'
  const sameYear = d.getFullYear() === new Date(now).getFullYear()
  return d.toLocaleDateString(undefined, {
    weekday: 'short',
    month: 'short',
    day: 'numeric',
    year: sameYear ? undefined : 'numeric',
  })
}

export function sameDay(a: string, b: string) {
  return dayDiff(new Date(a), new Date(b)) === 0
}

/** A moment ahead: "Today, 3:00 PM", "Tomorrow, 9:00 AM" or "Mon, Nov 3, 9:00 AM". */
export function formatWhen(iso: string, now: number) {
  const d = new Date(iso)
  const days = dayDiff(new Date(now), d)
  const clock = formatClock(d)
  if (days === 0) return `Today, ${clock}`
  if (days === 1) return `Tomorrow, ${clock}`
  return `${d.toLocaleDateString(undefined, { weekday: 'short', month: 'short', day: 'numeric' })}, ${clock}`
}

/** "12m" or "2h 5m". */
export function formatDuration(ms: number) {
  const min = Math.max(0, Math.floor(ms / 60_000))
  if (min < 60) return `${min}m`
  return `${Math.floor(min / 60)}h ${min % 60}m`
}

/** Minutes a customer has waited for a reply, or null when there is nothing to wait for. */
export function waitingMinutes(waitingSince: string | null, now: number) {
  if (!waitingSince) return null
  return Math.max(0, Math.floor((now - new Date(waitingSince).getTime()) / 60_000))
}

/** Badge tone: muted under 30 minutes, amber from 30, red from 60 (F2). */
export function waitTone(min: number) {
  if (min >= 60) return 'text-destructive'
  if (min >= 30) return 'text-warning'
  return 'text-muted-foreground'
}

/** What's left of a reply window of `hours`, as a duration and a share of the whole. */
export function windowLeft(expiresAt: string | null, now: number, hours: number) {
  if (!expiresAt || hours <= 0) return { ms: 0, pct: 0 }
  const ms = Math.max(0, new Date(expiresAt).getTime() - now)
  return { ms, pct: Math.min(100, (ms / (hours * 3_600_000)) * 100) }
}

export function formatSize(bytes: number) {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1 << 20) return `${Math.round(bytes / 1024)} KB`
  return `${(bytes / (1 << 20)).toFixed(1)} MB`
}

export function firstName(name: string) {
  return name.trim().split(/\s+/)[0] || name
}

/** Snooze presets (F7), as local times: in an hour, tomorrow 9:00 and next Monday 9:00. */
export function snoozeOptions(now: Date) {
  const at9 = (d: Date) => {
    d.setHours(9, 0, 0, 0)
    return d
  }
  const tomorrow = at9(new Date(now.getFullYear(), now.getMonth(), now.getDate() + 1))
  const toMonday = (8 - now.getDay()) % 7 || 7
  const monday = at9(new Date(now.getFullYear(), now.getMonth(), now.getDate() + toMonday))
  return [
    { label: '1 hour', until: new Date(now.getTime() + 3_600_000) },
    { label: 'Tomorrow, 9:00 AM', until: tomorrow },
    { label: 'Next Monday', until: monday },
  ]
}

/** Short assignee label: "You", a first name, or "Unassigned". */
export function assigneeShort(assignee: { id: string; name: string } | null, meId: string) {
  if (!assignee) return 'Unassigned'
  return assignee.id === meId ? 'You' : firstName(assignee.name)
}
