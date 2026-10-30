import type { Limits, Price } from '../../types'

const formatters = new Map<string, Intl.NumberFormat>()

function formatter(currency: string) {
  let f = formatters.get(currency)
  if (!f) {
    f = new Intl.NumberFormat(undefined, { style: 'currency', currency })
    formatters.set(currency, f)
  }
  return f
}

/**
 * "$49.00" from 4900 minor units. Amounts stay integers everywhere else; this is the only place
 * they become a decimal, and only to print them.
 */
export function formatMoney(minor: number, currency: string) {
  const f = formatter(currency)
  const digits = f.resolvedOptions().maximumFractionDigits ?? 2
  return f.format(minor / 10 ** digits)
}

/** "$79" for a whole amount, "$79.50" otherwise: how the pricing table shows a price. */
export function formatPrice(minor: number, currency: string) {
  const digits = formatter(currency).resolvedOptions().maximumFractionDigits ?? 2
  if (minor % 10 ** digits !== 0) return formatMoney(minor, currency)
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    maximumFractionDigits: 0,
  }).format(minor / 10 ** digits)
}

/**
 * "$0.0226" from 22600 micros (millionths of the currency unit): Meta's per-message rates go below
 * a cent, so they keep up to four decimals.
 */
export function formatMicros(micros: number, currency: string) {
  return new Intl.NumberFormat(undefined, {
    style: 'currency',
    currency,
    minimumFractionDigits: 2,
    maximumFractionDigits: 4,
  }).format(micros / 1_000_000)
}

/** "/mo", "/yr", or "one time". */
export function perInterval(price: Price) {
  if (price.interval === 'month') return '/month'
  if (price.interval === 'year') return '/year'
  return 'one time'
}

export function limitLabel(n: number, one: string, many: string) {
  if (n < 0) return `Unlimited ${many}`
  return `${n} ${n === 1 ? one : many}`
}

export function limitLines(limits: Limits) {
  return [
    limitLabel(limits.members, 'team member', 'team members'),
    limitLabel(limits.whatsapp_numbers, 'WhatsApp number', 'WhatsApp numbers'),
  ]
}

/** Whole-percent saving of a yearly price against twelve monthly ones, or 0. */
export function yearlySaving(monthly: Price | undefined, yearly: Price | undefined) {
  if (!monthly || !yearly || monthly.currency !== yearly.currency) return 0
  const full = monthly.amount_minor * 12
  if (full <= yearly.amount_minor) return 0
  return Math.floor(((full - yearly.amount_minor) * 100) / full)
}
