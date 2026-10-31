import { DownloadIcon } from 'lucide-react'
import { useId } from 'react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'
import type { WalletEntry } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { shortDay } from './dates'

/** Wallet movements, newest first. */
export function WalletLedger({
  entries,
  currency,
  className,
}: {
  entries: WalletEntry[]
  currency: string
  className?: string
}) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, currency)
  return (
    <Section aria-labelledby={`${id}-title`} className={cn('gap-3 sm:py-5', className)}>
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <h2 id={`${id}-title`} className="text-[17px] font-bold">
          Wallet transactions
        </h2>
        <Button
          variant="unstyled"
          type="button"
          disabled
          title="Coming soon"
          className="inline-flex h-10 items-center gap-1.5 rounded-[10px] border bg-card px-3 text-[13px] font-bold"
        >
          <DownloadIcon className="size-[18px]" aria-hidden="true" />
          Export CSV
          <span className="sr-only">(coming soon)</span>
        </Button>
      </div>
      <div className="relative -mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
        <table className="w-full min-w-[620px] border-collapse text-sm">
          <thead>
            <tr className="text-left text-xs text-muted-foreground">
              <th scope="col" className="py-2 pr-2 font-semibold">
                Date
              </th>
              <th scope="col" className="p-2 font-semibold">
                Description
              </th>
              <th scope="col" className="p-2 text-right font-semibold">
                Amount
              </th>
              <th scope="col" className="py-2 pl-2 text-right font-semibold">
                Balance after
              </th>
            </tr>
          </thead>
          <tbody>
            {entries.map((e) => {
              const credit = e.amount_minor > 0
              return (
                <tr key={e.id} className="border-t hover:bg-muted">
                  <td className="py-[11px] pr-2 whitespace-nowrap text-muted-foreground">
                    {shortDay(e.date)}
                  </td>
                  <td className="px-2 py-[11px]">
                    <div className="flex flex-col leading-[1.3]">
                      <span className="font-semibold">{e.description}</span>
                      <span className="text-xs text-muted-foreground">{e.detail}</span>
                    </div>
                  </td>
                  <td
                    className={cn(
                      'px-2 py-[11px] text-right font-bold whitespace-nowrap tabular-nums',
                      credit && 'text-brand-text',
                    )}
                  >
                    {credit ? '+' : '−'}
                    {money(Math.abs(e.amount_minor))}
                  </td>
                  <td className="py-[11px] pl-2 text-right whitespace-nowrap tabular-nums">
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
