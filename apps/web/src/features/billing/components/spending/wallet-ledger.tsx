import { DownloadIcon } from 'lucide-react'
import { useId } from 'react'
import { cn } from '#/lib/utils'
import type { WalletEntry } from '../../types'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { ComingSoonButton } from './coming-soon-button'
import { shortDay } from './dates'

const TH = 'px-3 py-2 font-semibold'
const TD = 'px-3 py-2.5'

/** Wallet movements, newest first. */
export function WalletLedger({ entries, currency }: { entries: WalletEntry[]; currency: string }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, currency)
  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader
        id={`${id}-title`}
        title="Wallet transactions"
        action={
          <ComingSoonButton icon={<DownloadIcon aria-hidden="true" />}>Export CSV</ComingSoonButton>
        }
      />
      <div className="overflow-x-auto rounded-lg border">
        <table className="w-full min-w-[560px] text-sm">
          <caption className="sr-only">Wallet transactions, newest first</caption>
          <thead className="bg-muted text-left text-xs text-muted-foreground">
            <tr>
              <th scope="col" className={TH}>
                Date
              </th>
              <th scope="col" className={TH}>
                Description
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Amount
              </th>
              <th scope="col" className={`${TH} text-right`}>
                Balance after
              </th>
            </tr>
          </thead>
          <tbody className="divide-y">
            {entries.map((e) => {
              const credit = e.amount_minor > 0
              return (
                <tr key={e.id}>
                  <td className={`${TD} whitespace-nowrap`}>{shortDay(e.date)}</td>
                  <td className={TD}>
                    <div className="flex flex-col">
                      <span className="font-bold">{e.description}</span>
                      <span className="text-xs text-muted-foreground">{e.detail}</span>
                    </div>
                  </td>
                  <td
                    className={cn(
                      TD,
                      'text-right font-bold whitespace-nowrap tabular-nums',
                      credit && 'text-brand-text',
                    )}
                  >
                    {credit ? '+' : '−'}
                    {money(Math.abs(e.amount_minor))}
                  </td>
                  <td className={`${TD} text-right whitespace-nowrap tabular-nums`}>
                    {money(e.balance_after_minor)}
                  </td>
                </tr>
              )
            })}
          </tbody>
        </table>
      </div>
    </Section>
  )
}
