import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowRightIcon, DownloadIcon, InfoIcon, SearchIcon } from 'lucide-react'
import { useDeferredValue, useMemo, useState } from 'react'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { NativeSelect } from '#/components/ui/native-select'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import {
  accessLogQueryOptions,
  accessLogSummaryQueryOptions,
  membersQueryOptions,
  permissionsQueryOptions,
} from '../../api/team.queries'
import { LOG_ACTIONS, TONE } from '../../schemas/team.schema'
import type { LogEntry } from '../../types'
import { PersonAvatar } from '../kit/avatar'
import { Btn } from '../kit/btn'
import { deviceLabel, formatShortDate, formatTime } from '../kit/format'
import { logChange } from '../kit/log-format'
import { Breadcrumb } from '../kit/panel'
import { Pill } from '../kit/pill'

const RANGES = [
  { value: '7', label: 'Last 7 days' },
  { value: '30', label: 'Last 30 days' },
  { value: '90', label: 'Last 90 days' },
  { value: '365', label: 'Last 12 months' },
] as const

type Range = (typeof RANGES)[number]['value']

function sinceDays(days: Range) {
  const d = new Date()
  d.setHours(0, 0, 0, 0)
  d.setDate(d.getDate() - Number(days))
  return d.toISOString()
}

export function AccessLog({
  workspaceId,
  member: initialMember,
}: {
  workspaceId: string
  member?: string
}) {
  const [q, setQ] = useState('')
  const [range, setRange] = useState<Range>('30')
  const [member, setMember] = useState(initialMember ?? '')
  const [action, setAction] = useState('')
  const since = useMemo(() => sinceDays(range), [range])
  const log = useInfiniteQuery(accessLogQueryOptions(workspaceId, { member, since }, 200))
  const summary = useQuery(accessLogSummaryQueryOptions(workspaceId))
  const members = useInfiniteQuery(membersQueryOptions(workspaceId))
  const catalog = useQuery(permissionsQueryOptions())
  const label = (k: string) => catalog.data?.find((p) => p.key === k)?.label ?? k
  const query = useDeferredValue(q.trim().toLowerCase())

  const all = log.data?.pages.flatMap((p) => p.entries) ?? []
  const matches = (e: LogEntry) =>
    !query ||
    [e.actor_name, e.subject_name, e.subject_email ?? '', label(e.target), e.ip, e.note]
      .join(' ')
      .toLowerCase()
      .includes(query)
  const searched = all.filter(matches)
  const rows = searched.filter((e) => !action || e.action === action)
  const counts = new Map<string, number>()
  for (const e of searched) counts.set(e.action, (counts.get(e.action) ?? 0) + 1)
  const people = members.data?.pages.flatMap((p) => p.members) ?? []

  function exportCsv() {
    const cell = (v: string) => `"${v.replace(/"/g, '""')}"`
    const lines = [
      [
        'When',
        'Done by',
        'Member',
        'Action',
        'Change',
        'Before',
        'After',
        'Note',
        'IP',
        'Device',
      ].join(','),
      ...rows.map((e) => {
        const c = logChange(e, label)
        return [
          new Date(e.created_at).toISOString(),
          e.actor_name || 'System',
          e.subject_name || e.subject_email || '',
          LOG_ACTIONS[e.action]?.label ?? e.action,
          c.target,
          c.before ?? '',
          c.after ?? '',
          e.note,
          e.ip,
          deviceLabel(e.user_agent),
        ]
          .map((v) => cell(String(v)))
          .join(',')
      }),
    ]
    const url = URL.createObjectURL(new Blob([lines.join('\n')], { type: 'text/csv' }))
    const a = document.createElement('a')
    a.href = url
    a.download = `access-log-${new Date().toISOString().slice(0, 10)}.csv`
    a.click()
    URL.revokeObjectURL(url)
  }

  const s = summary.data
  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
      <Breadcrumb current="Access log" />
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Access log</h1>
          <p className="text-[15px] text-muted-foreground">
            Every change to who can do what. Only people who manage roles can see this page.
          </p>
        </div>
        <Btn variant="outline" className="px-4" onClick={exportCsv} disabled={rows.length === 0}>
          <DownloadIcon className="size-[18px]" aria-hidden="true" />
          Export CSV
        </Btn>
      </div>

      <section
        aria-label="Summary"
        className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-3.5"
      >
        <StatCard label="Events, last 30 days" value={s?.events} />
        <StatCard label="Permission changes" value={s?.permission_changes} />
        <StatCard
          label="Expiring in 7 days"
          value={s?.expiring.length}
          note={s?.expiring
            .map(
              (x) =>
                `${x.name.split(' ')[0]}: ${label(x.permission_key)} (${formatShortDate(x.expires_at)})`,
            )
            .join(' · ')}
        />
        <StatCard
          danger
          label="Security alerts"
          value={s?.alerts.length}
          note={
            s && s.alerts.length > 0
              ? s.alerts
                  .map(
                    (a) =>
                      `${a.failures} failed logins for ${a.name} (last ${formatShortDate(a.last_at)})`,
                  )
                  .join(' · ')
              : s && 'No repeated failed sign-ins'
          }
        />
      </section>

      <section
        aria-label="Access events"
        className="flex flex-col gap-3.5 rounded-xl border bg-card px-5 py-[18px] shadow-card sm:px-[22px]"
      >
        <div className="flex flex-wrap items-center gap-2.5">
          <label className="flex h-10 max-w-[340px] flex-[1_1_240px] items-center gap-2 rounded-[10px] border bg-muted px-3 text-muted-foreground">
            <SearchIcon className="size-[18px] flex-none" aria-hidden="true" />
            <span className="sr-only">Search the log</span>
            <Input
              unstyled
              type="search"
              value={q}
              onChange={(e) => setQ(e.target.value)}
              placeholder="Search name, permission or IP"
              className="min-w-0 flex-1 border-0 bg-transparent text-base text-foreground outline-none md:text-[13px]"
            />
          </label>
          <label className="flex items-center gap-2 text-[13px] font-semibold text-muted-foreground">
            <span className="sr-only">Date range</span>
            <NativeSelect
              unstyled
              value={range}
              onChange={(e) => setRange(e.target.value as Range)}
              className="h-10 rounded-[10px] border bg-card px-2.5 text-[13px] font-semibold text-foreground"
            >
              {RANGES.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </NativeSelect>
          </label>
          <label className="flex items-center gap-2 text-[13px] font-semibold text-muted-foreground">
            Member
            <NativeSelect
              unstyled
              value={member}
              onChange={(e) => setMember(e.target.value)}
              className="h-10 rounded-[10px] border bg-card px-2.5 text-[13px] font-semibold text-foreground"
            >
              <option value="">All members</option>
              {people.map((p) => (
                <option key={p.user_id} value={p.user_id}>
                  {p.name}
                </option>
              ))}
            </NativeSelect>
          </label>
          <span className="ml-auto text-[13px] text-muted-foreground">
            {rows.length} {rows.length === 1 ? 'event' : 'events'}
          </span>
        </div>

        <div role="tablist" aria-label="Action type" className="flex flex-wrap gap-1.5">
          {[
            ['', 'All', searched.length] as const,
            ...[...counts.entries()]
              .sort((a, b) => b[1] - a[1])
              .map(([k, n]) => [k, LOG_ACTIONS[k]?.label ?? k, n] as const),
          ].map(([k, text, n]) => (
            <Button
              variant="unstyled"
              key={k || 'all'}
              type="button"
              role="tab"
              aria-selected={action === k}
              onClick={() => setAction(k)}
              className={cn(
                'inline-flex h-[34px] items-center gap-1.5 rounded-full border px-3 text-[13px] font-bold',
                action === k
                  ? 'border-primary bg-primary text-primary-foreground'
                  : 'bg-card text-foreground hover:bg-muted',
              )}
            >
              {text}
              <span className="text-xs opacity-80">{n}</span>
            </Button>
          ))}
        </div>

        {log.isError && (
          <p className="text-destructive">{errorMessage(log.error, 'Could not load the log.')}</p>
        )}
        <div className="-mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
          <table className="w-full min-w-[1000px] border-collapse text-sm">
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th className="py-2 pr-2 font-semibold">When</th>
                <th className="p-2 font-semibold">Done by</th>
                <th className="p-2 font-semibold">Member</th>
                <th className="p-2 font-semibold">Action</th>
                <th className="p-2 font-semibold">Change</th>
                <th className="py-2 pl-2 font-semibold">IP · device</th>
              </tr>
            </thead>
            <tbody>
              {log.isPending && (
                <tr>
                  <td colSpan={6} className="py-3">
                    <Skeleton className="h-40 w-full" />
                  </td>
                </tr>
              )}
              {rows.map((e) => (
                <LogRow key={e.id} entry={e} label={label} />
              ))}
            </tbody>
          </table>
          {!log.isPending && rows.length === 0 && (
            <p className="my-6 text-center text-muted-foreground">No events match these filters.</p>
          )}
        </div>
        {log.hasNextPage && (
          <Btn
            variant="outline"
            size="md"
            className="self-center"
            disabled={log.isFetchingNextPage}
            onClick={() => log.fetchNextPage()}
          >
            {log.isFetchingNextPage ? 'Loading…' : 'Show older events'}
          </Btn>
        )}
      </section>

      <div className="flex flex-wrap items-center gap-3 rounded-[14px] bg-info-soft px-[18px] py-3.5 text-[13px]">
        <InfoIcon className="size-[18px] flex-none text-info" aria-hidden="true" />
        <span className="flex-[1_1_320px]">
          <strong>Kept for 12 months.</strong> Entries can’t be edited or deleted, including by the
          Owner. Export a CSV any time for your own records.
        </span>
        <Link to="/team" className="font-bold">
          Back to Team
        </Link>
      </div>
    </main>
  )
}

function StatCard({
  label,
  value,
  note,
  danger,
}: {
  label: string
  value?: number
  note?: string
  danger?: boolean
}) {
  return (
    <div
      className={cn(
        'flex flex-col gap-1 rounded-xl border px-[18px] py-4',
        danger && value ? 'bg-destructive-soft' : 'bg-card shadow-card',
      )}
    >
      <span
        className={cn(
          'text-[13px] font-semibold',
          danger && value ? 'text-destructive' : 'text-muted-foreground',
        )}
      >
        {label}
      </span>
      <span className="text-[26px] font-extrabold">{value ?? '—'}</span>
      {note && (
        <span
          className={cn('text-xs', danger && value ? 'text-foreground' : 'text-muted-foreground')}
        >
          {note}
        </span>
      )}
    </div>
  )
}

function LogRow({ entry: e, label }: { entry: LogEntry; label: (k: string) => string }) {
  const meta = LOG_ACTIONS[e.action] ?? { label: e.action, tone: 'gray' as const }
  const change = logChange(e, label)
  const subject = e.subject_name || e.subject_email || '—'
  return (
    <tr className="border-t align-top hover:bg-muted">
      <td className="py-3 pr-2 whitespace-nowrap">
        <div className="flex flex-col leading-[1.3]">
          <span className="font-bold">{formatShortDate(e.created_at)}</span>
          <span className="text-xs text-muted-foreground">{formatTime(e.created_at)}</span>
        </div>
      </td>
      <td className="p-2">
        <div className="flex items-center gap-2">
          <PersonAvatar
            name={e.actor_name || 'System'}
            seed={e.actor_id ?? 'system'}
            tone={e.actor_id ? undefined : 'gray'}
            size={28}
          />
          <span className="font-semibold whitespace-nowrap">{e.actor_name || 'System'}</span>
        </div>
      </td>
      <td className="p-2 font-semibold whitespace-nowrap">{subject}</td>
      <td className="p-2">
        <Pill tone={meta.tone} className="py-[3px]">
          {meta.label}
        </Pill>
      </td>
      <td className="p-2">
        <div className="flex flex-col gap-1.5">
          {change.target && <span className="font-semibold">{change.target}</span>}
          {(change.before || change.after) && (
            <div className="flex flex-wrap items-center gap-1.5">
              {change.before && (
                <>
                  <span className="rounded-md bg-neutral-soft px-2 py-0.5 text-xs font-bold text-neutral line-through">
                    {change.before}
                  </span>
                  <ArrowRightIcon
                    className="size-3.5 text-muted-foreground"
                    aria-label="changed to"
                  />
                </>
              )}
              {change.after && (
                <span
                  className={cn(
                    'rounded-md px-2 py-0.5 text-xs font-bold',
                    TONE[meta.tone].soft,
                    TONE[meta.tone].text,
                  )}
                >
                  {change.after}
                </span>
              )}
            </div>
          )}
          {e.note && <span className="text-xs text-muted-foreground">{e.note}</span>}
        </div>
      </td>
      <td className="py-3 pl-2 whitespace-nowrap">
        <div className="flex flex-col leading-[1.3]">
          <span className="font-mono text-xs">{e.ip || '—'}</span>
          <span className="text-xs text-muted-foreground">{deviceLabel(e.user_agent)}</span>
        </div>
      </td>
    </tr>
  )
}
