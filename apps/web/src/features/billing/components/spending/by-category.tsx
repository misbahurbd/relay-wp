import { useId } from 'react'
import type { Spending } from '../../types'
import { formatMicros, formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { formatCount, formatShare } from './amounts'
import { CATEGORY, CategoryLabel } from './category-label'

const TH = 'px-3 py-2 font-semibold'
const TD = 'px-3 py-2.5'

/** Meta charges per message category, with each one's share of the Meta total. */
export function ByCategory({ spending: s }: { spending: Spending }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, s.currency)
  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader id={`${id}-title`} title="Spend by message category" />
      <div className="overflow-x-auto rounded-lg border">
        <table className="w-full min-w-[600px] text-sm">
          <caption className="sr-only">Meta charges by message category</caption>
          <thead className="bg-muted text-left text-xs text-muted-foreground">
            <tr>
              <th scope="col" className={TH}>
                Category
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Avg. rate / message
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Messages
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Total
              </th>
              <th scope="col" className={`${TH} w-[28%]`}>
                Share of Meta spend
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {s.categories.map((c) => {
              const share = formatShare(c.cost_minor, s.meta_minor)
              return (
                <tr key={c.category}>
                  <th scope="row" className={`${TD} text-left font-semibold`}>
                    <CategoryLabel category={c.category} />
                  </th>
                  <td className={`${TD} text-right tabular-nums`}>
                    {c.avg_micros > 0 ? formatMicros(c.avg_micros, s.currency) : 'Free'}
                  </td>
                  <td className={`${TD} text-right tabular-nums`}>{formatCount(c.messages)}</td>
                  <td className={`${TD} text-right font-bold tabular-nums`}>
                    {money(c.cost_minor)}
                  </td>
                  <td className={TD}>
                    <div className="flex items-center gap-2">
                      <div aria-hidden="true" className="h-2 flex-1 rounded-full bg-muted">
                        <div
                          className={`h-2 rounded-full ${CATEGORY[c.category].dot}`}
                          style={{
                            width: `${s.meta_minor > 0 ? (c.cost_minor / s.meta_minor) * 100 : 0}%`,
                          }}
                        />
                      </div>
                      <span className="w-12 text-right text-xs font-semibold tabular-nums">
                        {share}
                      </span>
                    </div>
                  </td>
                </tr>
              )
            })}
          </tbody>
          <tfoot className="border-t font-bold">
            <tr>
              <th scope="row" className={`${TD} text-left`}>
                Total
              </th>
              <td className={`${TD} text-right tabular-nums`}>
                {formatMicros(s.avg_micros, s.currency)}
              </td>
              <td className={`${TD} text-right tabular-nums`}>
                {formatCount(s.paid_messages + s.free_messages)}
              </td>
              <td className={`${TD} text-right tabular-nums`}>{money(s.meta_minor)}</td>
              <td className={TD} />
            </tr>
          </tfoot>
        </table>
      </div>
    </Section>
  )
}
