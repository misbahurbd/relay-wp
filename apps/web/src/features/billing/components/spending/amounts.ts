// Number formatting for the Spending page. Money stays in integer minor units until printed.

import { formatMoney } from '../kit/money'

const count = new Intl.NumberFormat()

/** "18,240". */
export function formatCount(n: number) {
  return count.format(n)
}

/** Minor-unit digits of a currency: 2 for USD, 0 for JPY. */
export function minorDigits(currency: string) {
  return (
    new Intl.NumberFormat(undefined, { style: 'currency', currency }).resolvedOptions()
      .maximumFractionDigits ?? 2
  )
}

/** Minor units from what a member typed in major units ("600" is 60000 cents), or null. */
export function parseMajor(text: string, currency: string) {
  const n = Number.parseFloat(text)
  if (!Number.isFinite(n) || n < 0) return null
  return Math.round(n * 10 ** minorDigits(currency))
}

/** "$20" for a chart axis: whole units only. */
export function formatWholeMoney(minor: number, currency: string) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: 0,
  }).format(minor / 10 ** minorDigits(currency))
}

/** "89.7%" for a part of a whole; "0%" when the whole is empty. */
export function formatShare(part: number, whole: number) {
  if (whole <= 0) return '0%'
  return `${Math.round((part / whole) * 1000) / 10}%`
}

/** "+9.2%" from tenths of a percent. */
export function formatChange(permille: number) {
  const sign = permille > 0 ? '+' : permille < 0 ? '−' : ''
  return `${sign}${Math.abs(permille) / 10}%`
}

/** "$589" for a whole amount, "$493.70" otherwise: how the design prints projections. */
export function formatRoughMoney(minor: number, currency: string) {
  return minor % 10 ** minorDigits(currency) === 0
    ? formatWholeMoney(minor, currency)
    : formatMoney(minor, currency)
}
