import { useQuery } from '@tanstack/react-query'
import { CheckIcon } from 'lucide-react'
import { Badge } from '#/components/ui/badge'
import { Skeleton } from '#/components/ui/skeleton'
import {
  contactsUsageQueryOptions,
  monthlyUsageQueryOptions,
  numbersUsageQueryOptions,
  resourcesQueryOptions,
  seatsUsageQueryOptions,
} from '../../api/billing.queries'
import type { Overview, Resource } from '../../types'
import { type LimitState, limitState } from '../kit/limit-state'
import { Meter } from '../kit/meter'
import { Section, SectionHeader } from '../kit/section'

const DAY = 24 * 60 * 60 * 1000

type Load = { isPending: boolean; isError: boolean }
const LOADED: Load = { isPending: false, isError: false }

/** One meter: what the plan allows of a resource and how much is used, once loaded. */
type Row = { resource: Resource; label: string; load: Load; used?: number; limit: number }

/**
 * How much of each plan limit the workspace uses: a meter for every count and monthly resource, then
 * the features the plan includes. Each count comes from the feature that owns it.
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
  const resources = useQuery(resourcesQueryOptions())
  const seats = useQuery(seatsUsageQueryOptions(workspaceId))
  const numbers = useQuery(numbersUsageQueryOptions(workspaceId))
  const contacts = useQuery(contactsUsageQueryOptions(workspaceId))
  const monthly = useQuery(monthlyUsageQueryOptions(workspaceId))
  const limits = overview.plan.limits

  const counted: Record<string, { load: Load; used?: number; limit?: number }> = {
    members: { load: seats, used: seats.data?.used, limit: seats.data?.limit },
    whatsapp_numbers: { load: numbers, used: numbers.data },
    contacts: { load: contacts, used: contacts.data },
  }

  const catalog = resources.data ?? []
  // Every limit gets a meter, including ones whose feature is still being built (nothing used yet).
  // A limit of 0 (not in the plan) shows only once something is used, as after a downgrade, so
  // going over it is visible.
  const rows: Row[] = catalog
    .filter((r) => r.kind !== 'access')
    .flatMap((resource): Row[] => {
      const limit = limits[resource.key] ?? 0
      if (resource.kind === 'monthly') {
        const m = monthly.data?.resources[resource.key]
        return [
          {
            resource,
            label: `${resource.label} this month`,
            load: monthly,
            used: m?.used,
            limit: m?.limit ?? limit,
          },
        ]
      }
      const c = counted[resource.key] ?? { load: LOADED, used: 0 }
      return [
        { resource, label: resource.label, load: c.load, used: c.used, limit: c.limit ?? limit },
      ]
    })
    .filter((r) => r.limit !== 0 || (r.used ?? 0) > 0)
  const features = catalog.filter((r) => r.kind === 'access' && limits[r.key] === 1)

  const states = rows.flatMap((r) => (r.used === undefined ? [] : [limitState(r.used, r.limit)]))
  const alert = limitsAlert(states)

  return (
    <Section aria-labelledby="usage-heading" className={className}>
      <SectionHeader
        id="usage-heading"
        title="Usage limits"
        description={period(monthly.data)}
        action={
          alert && (
            <Badge variant={alert.variant} className="rounded-full px-2.5">
              {alert.label}
            </Badge>
          )
        }
      />
      <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-x-6 gap-y-4">
        {resources.isPending
          ? [0, 1, 2].map((i) => (
              <Skeleton key={i} className="h-[62px]" aria-label="Loading usage" />
            ))
          : rows.map((row) => <UsageRow key={row.resource.key} row={row} />)}
      </div>
      {features.length > 0 && (
        <div className="mt-5 flex flex-col gap-2 border-t pt-4">
          <span className="text-[13px] font-bold">Included in {overview.plan.plan_name}</span>
          <ul className="flex flex-wrap gap-x-5 gap-y-1.5 text-[13px] text-muted-foreground">
            {features.map((f) => (
              <li key={f.key} className="flex items-center gap-1.5">
                <CheckIcon className="size-3.5 text-brand" aria-hidden="true" />
                {f.label}
              </li>
            ))}
          </ul>
        </div>
      )}
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

/**
 * "Nov 1 – Nov 30 · monthly limits reset in 24 days": monthly limits count a calendar month in UTC,
 * whatever the subscription's billing dates, so the dates are shown in UTC too.
 */
function period(month: { periodStart: string; periodEnd: string } | undefined) {
  if (!month) return 'What your plan includes and how much of it you use.'
  const end = Date.parse(month.periodEnd)
  const short = (ms: number) =>
    new Date(ms).toLocaleDateString(undefined, { month: 'short', day: 'numeric', timeZone: 'UTC' })
  const days = Math.max(0, Math.ceil((end - Date.now()) / DAY))
  return `${short(Date.parse(month.periodStart))} – ${short(end - DAY)} · ${
    days <= 1 ? `monthly limits reset ${short(end)}` : `monthly limits reset in ${days} days`
  }`
}

function UsageRow({ row }: { row: Row }) {
  const { label, load, used, limit } = row
  if (load.isPending) return <Skeleton className="h-[62px]" aria-label={`Loading ${label}`} />
  if (load.isError || used === undefined) {
    return (
      <div className="flex flex-col gap-1">
        <span className="text-[13px] font-bold">{label}</span>
        <span className="text-xs text-muted-foreground">Couldn’t load this right now.</span>
      </div>
    )
  }
  return <Meter label={label} used={used} limit={limit} />
}
