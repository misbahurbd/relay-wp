import { Link } from '@tanstack/react-router'
import { MegaphoneIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Skeleton } from '#/components/ui/skeleton'
import { percent } from '../../schemas/broadcasts.schema'
import type { BroadcastSummary } from '../../types'
import { formatCount, formatWhen } from '../kit/format'
import { StatusBadge } from '../kit/status-badge'

const COLUMNS = 6

export function BroadcastTable({
  items,
  loading,
  empty,
}: {
  items: BroadcastSummary[]
  loading: boolean
  empty: ReactNode
}) {
  return (
    <div className="-mx-5 overflow-x-auto px-5 sm:-mx-6 sm:px-6">
      <table className="w-full min-w-[820px] border-collapse text-sm">
        <thead>
          <tr className="text-left text-xs text-muted-foreground">
            <th scope="col" className="py-2 pr-2 font-semibold">
              Broadcast
            </th>
            <th scope="col" className="p-2 font-semibold">
              Status
            </th>
            <th scope="col" className="p-2 text-right font-semibold">
              Recipients
            </th>
            <th scope="col" className="p-2 text-right font-semibold">
              Delivered
            </th>
            <th scope="col" className="p-2 text-right font-semibold">
              Read
            </th>
            <th scope="col" className="py-2 pl-2 font-semibold">
              When
            </th>
          </tr>
        </thead>
        <tbody>
          {loading &&
            [0, 1, 2, 3].map((i) => (
              <tr key={i} className="border-t">
                <td colSpan={COLUMNS} className="py-3">
                  <Skeleton className="h-11 w-full" />
                </td>
              </tr>
            ))}
          {items.map((b) => (
            <Row key={b.id} broadcast={b} />
          ))}
          {!loading && items.length === 0 && (
            <tr className="border-t">
              <td colSpan={COLUMNS} className="py-12 text-center">
                {empty}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function Row({ broadcast: b }: { broadcast: BroadcastSummary }) {
  const started = b.status !== 'draft' && b.status !== 'scheduled'
  const when =
    b.status === 'scheduled'
      ? `Scheduled for ${formatWhen(b.scheduled_at)}`
      : started
        ? formatWhen(b.started_at)
        : 'Not sent yet'
  return (
    <tr className="border-t align-top hover:bg-muted/60">
      <td className="max-w-[360px] py-3 pr-2">
        <div className="flex items-start gap-3">
          <span className="flex size-9 flex-none items-center justify-center rounded-lg bg-brand-soft text-brand-text">
            <MegaphoneIcon className="size-[18px]" aria-hidden="true" />
          </span>
          <div className="flex min-w-0 flex-col">
            <Link
              to="/broadcasts/$broadcastId"
              params={{ broadcastId: b.id }}
              className="truncate font-bold text-foreground no-underline hover:underline"
            >
              {b.name}
            </Link>
            <span className="truncate text-[13px] text-muted-foreground">
              {b.template?.name ?? 'No template chosen'}
            </span>
          </div>
        </div>
      </td>
      <td className="p-2 py-3">
        <StatusBadge status={b.status} />
      </td>
      <td className="p-2 py-3 text-right tabular-nums">
        {started ? formatCount(b.recipients_total) : '—'}
      </td>
      <td className="p-2 py-3 text-right tabular-nums">
        {started ? `${percent(b.delivered, b.recipients_total)}%` : '—'}
      </td>
      <td className="p-2 py-3 text-right tabular-nums">
        {started ? `${percent(b.read, b.recipients_total)}%` : '—'}
      </td>
      <td className="py-3 pl-2 whitespace-nowrap text-muted-foreground">{when}</td>
    </tr>
  )
}
