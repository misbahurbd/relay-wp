import { formatMoney } from './money'

/**
 * "3 monthly payments of $85.00" or "5 monthly payments: 4 × $85.00, then $79.00". The amounts
 * come from the offer as they are; the last one absorbs rounding.
 */
export function describeInstallments(amounts: number[], currency: string) {
  if (amounts.length === 0) return ''
  const runs: { amount: number; times: number }[] = []
  for (const amount of amounts) {
    const last = runs.at(-1)
    if (last?.amount === amount) last.times++
    else runs.push({ amount, times: 1 })
  }
  const count = `${amounts.length} monthly payment${amounts.length === 1 ? '' : 's'}`
  if (runs.length === 1) return `${count} of ${formatMoney(runs[0].amount, currency)}`
  const parts = runs.map((r, i) =>
    r.times === 1
      ? `${i > 0 ? 'then ' : ''}${formatMoney(r.amount, currency)}`
      : `${i > 0 ? 'then ' : ''}${r.times} × ${formatMoney(r.amount, currency)}`,
  )
  return `${count}: ${parts.join(', ')}`
}
