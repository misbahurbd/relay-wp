/** Zones offered for scheduling; the viewer's own zone is added first when it isn't here. */
const ZONES = [
  'Asia/Dhaka',
  'Asia/Kolkata',
  'Asia/Karachi',
  'Asia/Dubai',
  'Asia/Riyadh',
  'Asia/Singapore',
  'Asia/Jakarta',
  'Europe/London',
  'Europe/Berlin',
  'Africa/Lagos',
  'Africa/Nairobi',
  'America/New_York',
  'America/Chicago',
  'America/Los_Angeles',
  'America/Sao_Paulo',
  'Australia/Sydney',
  'UTC',
]

export const browserZone = () => Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'

/** "(GMT+6) Dhaka" for a zone, its offset as of now. */
export function zoneLabel(zone: string, at = new Date()) {
  const offset =
    new Intl.DateTimeFormat('en-US', { timeZone: zone, timeZoneName: 'shortOffset' })
      .formatToParts(at)
      .find((p) => p.type === 'timeZoneName')?.value ?? 'GMT'
  const city = zone.split('/').pop()?.replaceAll('_', ' ') ?? zone
  return `(${offset}) ${city}`
}

export function zoneOptions() {
  const own = browserZone()
  return (ZONES.includes(own) ? ZONES : [own, ...ZONES]).map((z) => ({
    value: z,
    label: zoneLabel(z),
  }))
}

/** The date ("2026-10-30") and time ("10:00") an instant shows in a zone. */
export function partsIn(iso: string, zone: string) {
  const p = Object.fromEntries(
    new Intl.DateTimeFormat('en-CA', {
      timeZone: zone,
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hourCycle: 'h23',
    })
      .formatToParts(new Date(iso))
      .map((x) => [x.type, x.value]),
  )
  return { date: `${p.year}-${p.month}-${p.day}`, time: `${p.hour}:${p.minute}` }
}

/** The instant a wall-clock date and time in a zone means, as ISO. Twice round for DST edges. */
export function zonedToIso(date: string, time: string, zone: string) {
  const [y, m, d] = date.split('-').map(Number)
  const [h, min] = time.split(':').map(Number)
  const wall = Date.UTC(y, m - 1, d, h, min)
  let utc = wall
  for (let i = 0; i < 2; i++) {
    const shown = partsIn(new Date(utc).toISOString(), zone)
    const [sy, sm, sd] = shown.date.split('-').map(Number)
    const [sh, smin] = shown.time.split(':').map(Number)
    utc += wall - Date.UTC(sy, sm - 1, sd, sh, smin)
  }
  return new Date(utc).toISOString()
}
