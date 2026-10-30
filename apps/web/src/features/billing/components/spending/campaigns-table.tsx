import { MegaphoneIcon } from 'lucide-react'
import { useId } from 'react'
import type { CampaignSource, CampaignSpend, CampaignSpendList } from '../../types'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { formatCount } from './amounts'
import { CategoryLabel } from './category-label'
import { ComingSoonButton } from './coming-soon-button'
import { monthName, shortDay } from './dates'

const SOURCE: Record<CampaignSource, string> = {
  broadcast: 'Broadcast',
  automation: 'Automation',
  api: 'API',
}

const TH = 'px-3 py-2 font-semibold'
const TD = 'px-3 py-2.5'

function kind(c: CampaignSpend) {
  return c.detail ? `${SOURCE[c.source]} · ${c.detail}` : SOURCE[c.source]
}

/** What each broadcast, automation and API flow cost this month. */
export function CampaignsTable({ campaigns }: { campaigns: CampaignSpendList }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, campaigns.currency)
  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader
        id={`${id}-title`}
        title="Where the money went"
        description={`${monthName(campaigns.month)} · broadcasts, automations and API messages`}
        action={
          <ComingSoonButton icon={<MegaphoneIcon aria-hidden="true" />}>
            Plan a broadcast
          </ComingSoonButton>
        }
      />
      <div className="overflow-x-auto rounded-lg border">
        <table className="w-full min-w-[760px] text-sm">
          <caption className="sr-only">Cost of each campaign this month, by cost</caption>
          <thead className="bg-muted text-left text-xs text-muted-foreground">
            <tr>
              <th scope="col" className={TH}>
                Campaign
              </th>
              <th scope="col" className={TH}>
                Date
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Recipients
              </th>
              <th scope="col" className={TH}>
                Category
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Cost
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Cost per reply
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {campaigns.rows.map((c) => (
              <tr key={c.id}>
                <th scope="row" className={`${TD} text-left font-normal`}>
                  <div className="flex flex-col">
                    <span className="font-bold">{c.name}</span>
                    <span className="text-xs text-muted-foreground">{kind(c)}</span>
                  </div>
                </th>
                <td className={`${TD} whitespace-nowrap`}>
                  {c.date ? shortDay(c.date) : 'All month'}
                </td>
                <td className={`${TD} text-right tabular-nums`}>{formatCount(c.recipients)}</td>
                <td className={`${TD} whitespace-nowrap`}>
                  <CategoryLabel
                    category={c.category}
                    label={c.category === 'service' ? 'Service' : undefined}
                  />
                </td>
                <td className={`${TD} text-right font-bold whitespace-nowrap tabular-nums`}>
                  {money(c.cost_minor)}
                  {c.in_progress && (
                    <span className="font-normal text-muted-foreground"> so far</span>
                  )}
                </td>
                <td className={`${TD} text-right tabular-nums`}>
                  {c.cost_per_reply_minor === null ? (
                    <>
                      <span aria-hidden="true">—</span>
                      <span className="sr-only">No replies expected</span>
                    </>
                  ) : (
                    money(c.cost_per_reply_minor)
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Section>
  )
}
