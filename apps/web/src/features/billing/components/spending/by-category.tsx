import { useId } from 'react'
import { cn } from '#/lib/utils'
import type { Spending } from '../../types'
import { formatMicros, formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { formatCount, formatShare } from './amounts'
import { CATEGORY } from './category-label'

/** Meta charges per message category, with each one's share of the Meta total. */
export function ByCategory({ spending: s, className }: { spending: Spending; className?: string }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, s.currency)
  return (
    <Section aria-labelledby={`${id}-title`} className={cn('gap-3 sm:py-5', className)}>
      <h2 id={`${id}-title`} className="text-[17px] font-bold">
        Spend by message category
      </h2>
      <div className="relative -mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
        <table className="w-full min-w-[600px] border-collapse text-sm">
          <thead>
            <tr className="text-left text-xs text-muted-foreground">
              <th scope="col" className="py-2 pr-2 font-semibold">
                Category
              </th>
              <th scope="col" className="p-2 text-right font-semibold">
                Avg. rate / message
              </th>
              <th scope="col" className="p-2 text-right font-semibold">
                Messages
              </th>
              <th scope="col" className="p-2 text-right font-semibold">
                Total
              </th>
              <th scope="col" className="w-[180px] py-2 pl-2 font-semibold">
                Share of Meta spend
              </th>
            </tr>
          </thead>
          <tbody>
            {s.categories.map((c) => {
              const share = formatShare(c.cost_minor, s.meta_minor)
              return (
                <tr key={c.category} className="border-t hover:bg-muted">
                  <th scope="row" className="py-3 pr-2 text-left">
                    <span className="inline-flex items-center gap-2 font-bold">
                      <span className={cn('size-2.5 rounded-[3px]', CATEGORY[c.category].bg)} />
                      {CATEGORY[c.category].label}
                    </span>
                  </th>
                  <td className="px-2 py-3 text-right text-muted-foreground tabular-nums">
                    {c.avg_micros > 0 ? formatMicros(c.avg_micros, s.currency) : 'Free'}
                  </td>
                  <td className="px-2 py-3 text-right tabular-nums">{formatCount(c.messages)}</td>
                  <td className="px-2 py-3 text-right font-bold tabular-nums">
                    {money(c.cost_minor)}
                  </td>
                  <td className="py-3 pl-2">
                    <div className="flex items-center gap-2">
                      <div aria-hidden="true" className="h-2 flex-1 rounded-sm bg-secondary">
                        <div
                          className={cn('h-2 rounded-sm', CATEGORY[c.category].bg)}
                          style={{ width: share }}
                        />
                      </div>
                      <span className="min-w-10 text-right text-xs font-bold tabular-nums">
                        {share}
                      </span>
                    </div>
                  </td>
                </tr>
              )
            })}
            <tr className="border-t-2">
              <th scope="row" className="py-3 pr-2 text-left font-extrabold">
                Total
              </th>
              <td className="px-2 py-3 text-right text-muted-foreground tabular-nums">
                {formatMicros(s.avg_micros, s.currency)}
              </td>
              <td className="px-2 py-3 text-right font-bold tabular-nums">
                {formatCount(s.paid_messages + s.free_messages)}
              </td>
              <td className="px-2 py-3 text-right font-extrabold tabular-nums">
                {money(s.meta_minor)}
              </td>
              <td className="py-3 pl-2" />
            </tr>
          </tbody>
        </table>
      </div>
    </Section>
  )
}
