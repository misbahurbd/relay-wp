import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { InfoIcon, WalletIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'
import { metaCostsPreviewQueryOptions, walletPreviewQueryOptions } from '../../api/billing.preview'
import type { MetaCostCategory, Wallet } from '../../types'
import { formatMicros, formatMoney } from '../kit/money'
import { PreviewBadge } from '../kit/preview-badge'
import { Section } from '../kit/section'

const CATEGORY: Record<MetaCostCategory, { label: string; swatch: string }> = {
  marketing: { label: 'Marketing', swatch: 'bg-brand' },
  utility: { label: 'Utility', swatch: 'bg-chart-2' },
  authentication: { label: 'Authentication', swatch: 'bg-warning-bar' },
  service: { label: 'Service', swatch: 'bg-border' },
}

const count = new Intl.NumberFormat()

/** Preview: this month's Meta charges by category, and the message wallet that pays them. */
export function MetaCostsCard({
  workspaceId,
  className,
}: {
  workspaceId: string
  className?: string
}) {
  const costs = useQuery(metaCostsPreviewQueryOptions(workspaceId))
  const wallet = useQuery(walletPreviewQueryOptions(workspaceId))
  const month = new Date().toLocaleDateString(undefined, { month: 'long', year: 'numeric' })

  return (
    <Section aria-labelledby="meta-heading" className={className}>
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-0.5">
          <h2 id="meta-heading" className="flex flex-wrap items-center gap-2 text-[17px] font-bold">
            Meta message costs <PreviewBadge />
          </h2>
          <p className="text-[13px] text-muted-foreground">{month} · paid from your wallet</p>
        </div>
        <p className="flex flex-wrap items-baseline gap-x-2">
          {costs.data && (
            <>
              <span className="text-[28px] font-extrabold tracking-[-0.02em]">
                {formatMoney(costs.data.total_minor, costs.data.currency)}
              </span>
              <span className="text-[13px] text-muted-foreground">so far</span>
            </>
          )}
          <Link
            to="/billing/spending"
            className="ml-1.5 text-[13px] font-bold text-brand-text underline underline-offset-2"
          >
            See spending details
          </Link>
        </p>
      </div>

      <p className="flex gap-2.5 rounded-lg bg-info-soft px-3.5 py-3 text-[13px]">
        <InfoIcon className="mt-0.5 size-4 flex-none text-info" aria-hidden="true" />
        <span>
          Meta charges a small fee for each template message you send, separate from your Relay
          plan. Replies inside the 24-hour customer service window are free.
        </span>
      </p>

      {!costs.data ? (
        <Skeleton className="h-48" aria-label="Loading Meta message costs" />
      ) : (
        <div className="relative overflow-x-auto">
          <table className="w-full min-w-[440px] text-sm">
            <caption className="sr-only">Meta message costs by category, this month</caption>
            <thead className="text-left text-xs text-muted-foreground">
              <tr>
                <th scope="col" className="py-2 pr-2 font-semibold">
                  Category
                </th>
                <th scope="col" className="p-2 text-right font-semibold">
                  Messages
                </th>
                <th scope="col" className="p-2 text-right font-semibold">
                  Avg. per message
                </th>
                <th scope="col" className="py-2 pl-2 text-right font-semibold">
                  Cost
                </th>
              </tr>
            </thead>
            <tbody>
              {costs.data.rows.map((r) => (
                <tr key={r.category} className="border-t">
                  <th scope="row" className="py-[11px] pr-2 text-left font-semibold">
                    <span className="inline-flex items-center gap-2.5">
                      <span
                        className={cn('size-2.5 rounded-[3px]', CATEGORY[r.category].swatch)}
                        aria-hidden="true"
                      />
                      {CATEGORY[r.category].label}
                    </span>
                  </th>
                  <td className="px-2 py-[11px] text-right tabular-nums">
                    {count.format(r.messages)}
                  </td>
                  <td className="px-2 py-[11px] text-right text-muted-foreground tabular-nums">
                    {r.avg_micros === 0 ? 'Free' : formatMicros(r.avg_micros, costs.data.currency)}
                  </td>
                  <td className="py-[11px] pl-2 text-right font-bold tabular-nums">
                    {formatMoney(r.cost_minor, costs.data.currency)}
                  </td>
                </tr>
              ))}
              <tr className="border-t-2">
                <th scope="row" className="py-[11px] pr-2 text-left font-extrabold">
                  Total
                </th>
                <td className="px-2 py-[11px] text-right font-bold tabular-nums">
                  {count.format(costs.data.messages)}
                </td>
                <td />
                <td className="py-[11px] pl-2 text-right font-extrabold tabular-nums">
                  {formatMoney(costs.data.total_minor, costs.data.currency)}
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}

      {wallet.data && <WalletStrip wallet={wallet.data} />}
    </Section>
  )
}

function WalletStrip({ wallet }: { wallet: Wallet }) {
  const money = (n: number) => formatMoney(n, wallet.currency)
  const { auto_recharge: auto } = wallet
  // A preview: the switch moves, but nothing is saved.
  const [on, setOn] = useState(auto.enabled)
  return (
    <div className="flex flex-wrap items-center gap-3.5 rounded-lg bg-muted px-4 py-3.5">
      <span
        className="flex size-10 flex-none items-center justify-center rounded-[10px] bg-brand-soft text-brand-text"
        aria-hidden="true"
      >
        <WalletIcon className="size-5" />
      </span>
      <span className="flex min-w-[140px] flex-1 flex-col leading-tight">
        <span className="text-xs font-semibold text-muted-foreground">Wallet balance</span>
        <span className="text-lg font-extrabold tabular-nums">{money(wallet.balance_minor)}</span>
      </span>
      <div className="flex items-center gap-2.5">
        <Button
          variant="unstyled"
          type="button"
          role="switch"
          aria-checked={on}
          aria-label="Auto-recharge"
          onClick={() => setOn(!on)}
          className={cn(
            'flex h-7 w-12 flex-none rounded-full p-[3px] transition-colors',
            on ? 'justify-end bg-primary' : 'justify-start bg-border',
          )}
        >
          <span className="size-[22px] rounded-full bg-card shadow-sm" />
        </Button>
        <span className="flex flex-col text-[13px] leading-tight">
          <span className="font-bold">Auto-recharge</span>
          <span className="text-xs text-muted-foreground">
            {on ? `Add ${money(auto.add_minor)} when below ${money(auto.below_minor)}` : 'Off'}
          </span>
        </span>
      </div>
      <Button disabled title="Coming soon">
        Top up
        <span className="sr-only">(coming soon)</span>
      </Button>
    </div>
  )
}
