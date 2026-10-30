import { useId } from 'react'
import type { Spending } from '../../types'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { formatShare } from './amounts'

const TH = 'px-3 py-2 font-semibold'
const TD = 'px-3 py-2.5'

/** Meta charges by the customer's country, which sets the per-message rate. */
export function ByCountry({ spending: s }: { spending: Spending }) {
  const id = useId()
  const total = s.countries.reduce((a, c) => a + c.cost_minor, 0)
  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader id={`${id}-title`} title="By customer country" />
      <div className="overflow-x-auto rounded-lg border">
        <table className="w-full min-w-[320px] text-sm">
          <caption className="sr-only">Meta charges by customer country</caption>
          <thead className="bg-muted text-left text-xs text-muted-foreground">
            <tr>
              <th scope="col" className={TH}>
                Country
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Share
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Cost
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {s.countries.map((c) => (
              <tr key={c.code ?? 'other'}>
                <th scope="row" className={`${TD} text-left font-semibold`}>
                  {c.name}
                </th>
                <td className={`${TD} text-right tabular-nums`}>
                  {formatShare(c.cost_minor, total)}
                </td>
                <td className={`${TD} text-right font-bold tabular-nums`}>
                  {formatMoney(c.cost_minor, s.currency)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Section>
  )
}
