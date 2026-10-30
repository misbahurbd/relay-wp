import { useId } from 'react'
import type { Spending } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { formatShare } from './amounts'

/** Meta charges by the customer's country, which sets the per-message rate. */
export function ByCountry({ spending: s }: { spending: Spending }) {
  const id = useId()
  const total = s.countries.reduce((a, c) => a + c.cost_minor, 0)
  return (
    <Section aria-labelledby={`${id}-title`} className="gap-2.5 sm:py-5">
      <h2 id={`${id}-title`} className="text-[17px] font-bold">
        By customer country
      </h2>
      <table className="w-full border-collapse text-sm">
        <thead>
          <tr className="text-left text-xs text-muted-foreground">
            <th scope="col" className="py-1.5 pr-2 font-semibold">
              Country
            </th>
            <th scope="col" className="px-2 py-1.5 text-right font-semibold">
              Share
            </th>
            <th scope="col" className="py-1.5 pl-2 text-right font-semibold">
              Cost
            </th>
          </tr>
        </thead>
        <tbody>
          {s.countries.map((c) => (
            <tr key={c.code ?? 'other'} className="border-t">
              <th scope="row" className="py-[9px] pr-2 text-left font-semibold">
                {c.name}
              </th>
              <td className="px-2 py-[9px] text-right text-muted-foreground tabular-nums">
                {formatShare(c.cost_minor, total)}
              </td>
              <td className="py-[9px] pl-2 text-right font-bold tabular-nums">
                {formatMoney(c.cost_minor, s.currency)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </Section>
  )
}
