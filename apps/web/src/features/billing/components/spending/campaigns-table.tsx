import { useId } from 'react'
import { cn } from '#/lib/utils'
import type { CampaignSource, CampaignSpend, CampaignSpendList } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { formatCount } from './amounts'
import { CATEGORY } from './category-label'
import { monthName, shortDay } from './dates'

const SOURCE: Record<CampaignSource, string> = {
  broadcast: 'Broadcast',
  automation: 'Automation',
  api: 'API',
}

function kind(c: CampaignSpend) {
  return c.detail ? `${SOURCE[c.source]} · ${c.detail}` : SOURCE[c.source]
}

/**
 * Broadcast names and "Plan a broadcast" are links in the design. Broadcasts aren't built yet, so
 * they keep the link's look and say so instead of going nowhere.
 */
function SoonLink({ children, className }: { children: string; className?: string }) {
  return (
    <span
      title="Broadcasts are coming soon"
      className={cn('font-bold text-brand-text underline', className)}
    >
      {children}
    </span>
  )
}

/** What each broadcast, automation and API flow cost this month. */
export function CampaignsTable({ campaigns }: { campaigns: CampaignSpendList }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, campaigns.currency)
  return (
    <Section aria-labelledby={`${id}-title`} className="gap-3 sm:py-5">
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <div className="flex flex-col gap-0.5">
          <h2 id={`${id}-title`} className="text-[17px] font-bold">
            Where the money went
          </h2>
          <span className="text-[13px] text-muted-foreground">
            {monthName(campaigns.month)} · broadcasts, automations and API messages
          </span>
        </div>
        <SoonLink className="text-[13px]">Plan a broadcast</SoonLink>
      </div>
      <div className="relative -mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
        <table className="w-full min-w-[760px] border-collapse text-sm">
          <thead>
            <tr className="text-left text-xs text-muted-foreground">
              <th scope="col" className="py-2 pr-2 font-semibold">
                Campaign
              </th>
              <th scope="col" className="p-2 font-semibold">
                Date
              </th>
              <th scope="col" className="p-2 text-right font-semibold">
                Recipients
              </th>
              <th scope="col" className="p-2 font-semibold">
                Category
              </th>
              <th scope="col" className="p-2 text-right font-semibold">
                Cost
              </th>
              <th scope="col" className="py-2 pl-2 text-right font-semibold">
                Cost per reply
              </th>
            </tr>
          </thead>
          <tbody>
            {campaigns.rows.map((c) => (
              <tr key={c.id} className="border-t hover:bg-muted">
                <th scope="row" className="py-3 pr-2 text-left font-normal">
                  <div className="flex flex-col leading-[1.3]">
                    {c.source === 'broadcast' ? (
                      <SoonLink>{c.name}</SoonLink>
                    ) : (
                      <span className="font-bold">{c.name}</span>
                    )}
                    <span className="text-xs text-muted-foreground">{kind(c)}</span>
                  </div>
                </th>
                <td className="px-2 py-3 whitespace-nowrap text-muted-foreground">
                  {c.date ? shortDay(c.date) : 'All month'}
                </td>
                <td className="px-2 py-3 text-right tabular-nums">{formatCount(c.recipients)}</td>
                <td className="px-2 py-3">
                  <span className="inline-flex items-center gap-1.5 text-[13px] font-semibold">
                    <span className={cn('size-2 rounded-[2px]', CATEGORY[c.category].bg)} />
                    {c.category === 'service' ? 'Service' : CATEGORY[c.category].label}
                  </span>
                </td>
                <td className="px-2 py-3 text-right font-bold whitespace-nowrap tabular-nums">
                  {money(c.cost_minor)}
                  {c.in_progress && ' so far'}
                </td>
                <td className="py-3 pl-2 text-right text-muted-foreground tabular-nums">
                  {c.cost_per_reply_minor === null ? '—' : money(c.cost_per_reply_minor)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Section>
  )
}
