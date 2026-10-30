import { useId } from 'react'
import type { Spending } from '../../types'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { formatShare } from './amounts'

/** Meta charges per WhatsApp number, as a share of the period's Meta total. */
export function ByNumber({ spending: s }: { spending: Spending }) {
  const id = useId()
  const total = s.numbers.reduce((a, n) => a + n.cost_minor, 0)
  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader id={`${id}-title`} title="By WhatsApp number" />
      <ul className="flex flex-col gap-4">
        {s.numbers.map((n) => {
          const share = formatShare(n.cost_minor, total)
          return (
            <li key={n.phone} className="flex flex-col gap-1.5">
              <div className="flex items-baseline justify-between gap-3">
                <span className="flex min-w-0 flex-col">
                  <span className="font-bold">{n.name}</span>
                  <span className="text-xs text-muted-foreground tabular-nums">{n.phone}</span>
                </span>
                <span className="text-right tabular-nums">
                  <span className="font-bold">{formatMoney(n.cost_minor, s.currency)}</span>
                  <span className="block text-xs text-muted-foreground">{share}</span>
                </span>
              </div>
              <div aria-hidden="true" className="h-2 overflow-hidden rounded-full bg-muted">
                <div
                  className="h-full rounded-full bg-brand"
                  style={{ width: `${total > 0 ? (n.cost_minor / total) * 100 : 0}%` }}
                />
              </div>
            </li>
          )
        })}
      </ul>
    </Section>
  )
}
