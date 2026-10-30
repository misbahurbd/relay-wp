import { useId } from 'react'
import type { Spending } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { formatShare } from './amounts'

/** Meta charges per WhatsApp number, with a bar for each one's share. */
export function ByNumber({ spending: s }: { spending: Spending }) {
  const id = useId()
  const total = s.numbers.reduce((a, n) => a + n.cost_minor, 0)
  return (
    <Section aria-labelledby={`${id}-title`} className="gap-2.5 sm:py-5">
      <h2 id={`${id}-title`} className="text-[17px] font-bold">
        By WhatsApp number
      </h2>
      {s.numbers.map((n) => (
        <div key={n.phone} className="flex flex-col gap-1.5 border-t py-2.5">
          <div className="flex justify-between gap-2.5">
            <span className="flex flex-col leading-[1.3]">
              <span className="font-bold">{n.name}</span>
              <span className="text-xs text-muted-foreground">{n.phone}</span>
            </span>
            <span className="font-extrabold tabular-nums">
              {formatMoney(n.cost_minor, s.currency)}
            </span>
          </div>
          <div aria-hidden="true" className="h-1.5 rounded-[3px] bg-secondary">
            <div
              className="h-1.5 rounded-[3px] bg-brand"
              style={{ width: formatShare(n.cost_minor, total) }}
            />
          </div>
        </div>
      ))}
    </Section>
  )
}
