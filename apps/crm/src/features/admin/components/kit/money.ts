/** "$49.00" from 4900 minor units. */
export function formatMoney(minor: number, currency: string) {
  return new Intl.NumberFormat(undefined, { style: 'currency', currency }).format(minor / 100)
}

/** Minor units from what staff typed in major units ("29" or "29.99"); NaN when it isn't a number. */
export function toMinor(major: string) {
  const n = Number(major.trim())
  if (major.trim() === '' || !Number.isFinite(n)) return Number.NaN
  return Math.round(n * 100)
}
