import { useQuery } from '@tanstack/react-query'
import { Skeleton } from '#/components/ui/skeleton'
import { segmentPreviewQueryOptions } from '../../api/contacts.segments'
import { rulesComplete } from '../../schemas/segments.schema'
import type { SegmentRules } from '../../types'
import { ContactAvatar } from '../kit/avatar'
import { formatNumber } from '../kit/format'
import { OptInPill } from '../kit/opt-in-pill'

/** The live count and a sample of who the rules match, as they are edited. */
export function WhoMatches({ workspaceId, rules }: { workspaceId: string; rules: SegmentRules }) {
  const ready = rulesComplete(rules)
  const q = useQuery({ ...segmentPreviewQueryOptions(workspaceId, rules), enabled: ready })
  const c = q.data?.count
  const share = (n: number) => (c && c.matched > 0 ? `${Math.round((n / c.matched) * 100)}%` : '0%')
  return (
    <aside
      aria-label="Who matches"
      className="flex flex-col gap-4 rounded-2xl border bg-card p-5 shadow-card xl:sticky xl:top-5"
    >
      <h2 className="text-base font-extrabold">Who matches</h2>
      <div className="flex flex-col gap-1 rounded-[14px] bg-brand-soft p-4" aria-live="polite">
        <span className="text-xs font-bold text-brand-text">Contacts in this segment</span>
        {!ready ? (
          <span className="text-sm text-muted-foreground">
            Finish the conditions to see who matches.
          </span>
        ) : c ? (
          <span className="text-[32px] leading-tight font-extrabold tabular-nums">
            {formatNumber(c.matched)}
          </span>
        ) : (
          <Skeleton className="h-9 w-24" />
        )}
      </div>
      {c && ready && (
        <div className="flex flex-col gap-2 text-sm">
          <span className="font-bold">Reachable on WhatsApp</span>
          <div
            className="flex h-2 overflow-hidden rounded-full bg-muted"
            role="img"
            aria-label={`${share(c.opted_in)} opted in`}
          >
            <span className="bg-primary" style={{ width: share(c.opted_in) }} />
            <span className="bg-warning" style={{ width: share(c.pending) }} />
            <span className="bg-destructive" style={{ width: share(c.opted_out) }} />
          </div>
          <span className="flex justify-between">
            <span>Opted in, gets broadcasts</span>
            <b className="tabular-nums">{formatNumber(c.opted_in)}</b>
          </span>
          <span className="flex justify-between text-muted-foreground">
            <span>Not opted in yet, skipped</span>
            <b className="tabular-nums">{formatNumber(c.pending)}</b>
          </span>
          <span className="flex justify-between text-muted-foreground">
            <span>Opted out, skipped</span>
            <b className="tabular-nums">{formatNumber(c.opted_out)}</b>
          </span>
        </div>
      )}
      {q.data && ready && (
        <div className="flex flex-col gap-2">
          <span className="text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase">
            Sample
          </span>
          {q.data.sample.length === 0 ? (
            <p className="text-[13px] text-muted-foreground">
              Nobody matches yet. Try loosening a condition or switching a group to ANY.
            </p>
          ) : (
            q.data.sample.map((m) => (
              <div key={m.id} className="flex items-center gap-2.5">
                <ContactAvatar name={m.name || m.phone} seed={m.id} />
                <span className="flex min-w-0 flex-1 flex-col leading-tight">
                  <span className="truncate font-bold">{m.name || m.phone}</span>
                  <span className="text-xs text-muted-foreground">{m.phone}</span>
                </span>
                <OptInPill value={m.opt_in} />
              </div>
            ))
          )}
          <span className="text-xs text-muted-foreground">
            Contacts who aren’t opted in are left out automatically when you send a broadcast.
          </span>
        </div>
      )}
      {q.isError && <p className="text-[13px] text-destructive">Couldn’t count this segment.</p>}
    </aside>
  )
}
