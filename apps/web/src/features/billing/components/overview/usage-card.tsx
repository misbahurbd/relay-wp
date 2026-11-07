import { useQuery } from '@tanstack/react-query'
import { Badge } from '#/components/ui/badge'
import { Skeleton } from '#/components/ui/skeleton'
import { usagePreviewQueryOptions } from '../../api/billing.preview'
import {
  monthlyUsageQueryOptions,
  numbersUsageQueryOptions,
  seatsUsageQueryOptions,
} from '../../api/billing.queries'
import { useBillingPreview } from '../../hooks/use-billing-preview'
import type { Overview, UsageRow } from '../../types'
import { formatDate } from '../kit/format'
import { type LimitState, limitState } from '../kit/limit-state'
import { Meter } from '../kit/meter'
import { PreviewBadge } from '../kit/preview-badge'
import { Section, SectionHeader } from '../kit/section'

const DAY = 24 * 60 * 60 * 1000

/**
 * How much of each plan limit the workspace uses. Seats, WhatsApp numbers and broadcasts are real;
 * contacts, flows and API calls are a preview until Relay counts them.
 */
export function UsageCard({
  workspaceId,
  overview,
  className,
}: {
  workspaceId: string
  overview: Overview
  className?: string
}) {
  const preview = useBillingPreview()
  const seats = useQuery(seatsUsageQueryOptions(workspaceId))
  const numbers = useQuery(numbersUsageQueryOptions(workspaceId))
  const monthly = useQuery(monthlyUsageQueryOptions(workspaceId))
  const broadcasts = monthly.data?.broadcasts
  const samples = useQuery({ ...usagePreviewQueryOptions(workspaceId), enabled: preview })
  const limits = overview.plan.limits
  const seatLimit = seats.data?.limit ?? limits.members
  const states = [
    seats.data && limitState(seats.data.used, seatLimit),
    numbers.data !== undefined && limitState(numbers.data, limits.whatsapp_numbers),
    broadcasts && limitState(broadcasts.used, broadcasts.limit),
  ].flatMap((s) => (s ? [s] : []))
  const alert = limitsAlert(states)
  const sample = (key: string) => samples.data?.find((r) => r.key === key)

  const seatsRow = (
    <Live label="Team seats" state={seats} used={seats.data?.used} limit={seatLimit} />
  )
  const numbersRow = (
    <Live
      label="WhatsApp numbers"
      state={numbers}
      used={numbers.data}
      limit={limits.whatsapp_numbers}
    />
  )
  const broadcastsRow = (
    <Live
      label="Broadcasts this month"
      state={monthly}
      used={broadcasts?.used}
      limit={broadcasts?.limit ?? limits.broadcasts}
    />
  )

  return (
    <Section aria-labelledby="usage-heading" className={className}>
      <SectionHeader
        id="usage-heading"
        title="Usage limits"
        description={period(overview)}
        action={
          alert && (
            <Badge variant={alert.variant} className="rounded-full px-2.5">
              {alert.label}
            </Badge>
          )
        }
      />
      <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-x-6 gap-y-4">
        {preview ? (
          <>
            <Sample row={sample('contacts')} />
            {seatsRow}
            <Sample row={sample('chatbot_flows')} />
            {broadcastsRow}
            <Sample row={sample('api_calls_per_month')} />
            {numbersRow}
          </>
        ) : (
          <>
            {seatsRow}
            {numbersRow}
            {broadcastsRow}
          </>
        )}
      </div>
    </Section>
  )
}

const ALERT_WORDS = [
  ['over', 'exceeded'],
  ['at', 'reached'],
  ['almost', 'almost reached'],
] as const

/** "1 limit exceeded, 2 almost reached": the limits that need attention, worst first. */
function limitsAlert(states: LimitState[]) {
  const parts = ALERT_WORDS.flatMap(([kind, words]) => {
    const n = states.filter((s) => s.kind === kind).length
    return n > 0 ? [{ n, words }] : []
  })
  if (parts.length === 0) return undefined
  const label = parts
    .map(({ n, words }, i) =>
      i === 0 ? `${n} ${n === 1 ? 'limit' : 'limits'} ${words}` : `${n} ${words}`,
    )
    .join(', ')
  return {
    label,
    variant: states.some((s) => s.kind === 'over') ? ('danger' as const) : ('warning' as const),
  }
}

/** "Oct 14 – Nov 13 · resets in 16 days" for a subscription, else what the card shows. */
function period({ subscription: sub }: Overview) {
  if (!sub?.current_period_start || !sub.current_period_end) {
    return 'What your plan includes and how much of it you use.'
  }
  const days = Math.max(0, Math.ceil((Date.parse(sub.current_period_end) - Date.now()) / DAY))
  const short = (iso: string) =>
    new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
  return `${short(sub.current_period_start)} – ${short(sub.current_period_end)} · ${
    days === 0 ? `resets ${formatDate(sub.current_period_end)}` : `resets in ${days} days`
  }`
}

function Sample({ row }: { row: UsageRow | undefined }) {
  if (!row) return <Skeleton className="h-[62px]" aria-label="Loading usage" />
  return <Meter label={row.label} used={row.used} limit={row.limit} tag={<PreviewBadge />} />
}

function Live({
  label,
  state,
  used,
  limit,
}: {
  label: string
  state: { isPending: boolean; isError: boolean }
  used: number | undefined
  limit: number
}) {
  if (state.isPending) return <Skeleton className="h-[62px]" aria-label={`Loading ${label}`} />
  if (state.isError || used === undefined) {
    return (
      <div className="flex flex-col gap-1">
        <span className="text-[13px] font-bold">{label}</span>
        <span className="text-xs text-muted-foreground">Couldn’t load this right now.</span>
      </div>
    )
  }
  return <Meter label={label} used={used} limit={limit} />
}
