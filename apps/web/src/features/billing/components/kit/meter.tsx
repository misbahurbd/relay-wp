import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'
import { type LimitState, limitState } from './limit-state'

const count = new Intl.NumberFormat()

const TONE: Record<LimitState['kind'], { count: string; note: string; bar: string }> = {
  unlimited: { count: 'text-foreground', note: 'text-muted-foreground', bar: 'bg-brand' },
  ok: { count: 'text-foreground', note: 'text-muted-foreground', bar: 'bg-brand' },
  almost: { count: 'text-warning', note: 'text-warning', bar: 'bg-warning-bar' },
  at: { count: 'text-warning', note: 'text-warning', bar: 'bg-warning-bar' },
  over: { count: 'text-destructive', note: 'text-destructive', bar: 'bg-destructive' },
}

function note(state: LimitState, used: number, limit: number) {
  switch (state.kind) {
    case 'unlimited':
      return 'No limit on your plan'
    case 'ok':
      return `${state.pct}% used`
    case 'almost':
      return `${state.pct}% used. Upgrade to avoid interruptions.`
    case 'at':
      return 'At your limit. Upgrade to add more.'
    case 'over':
      return `${count.format(used)} of ${count.format(limit)} · over your limit`
  }
}

/**
 * One usage row: label, used of limit, a bar, and a warning from 80%. limit -1 is unlimited.
 * The numbers are in the text, so the bar itself is decorative.
 */
export function Meter({
  label,
  used,
  limit,
  tag,
}: {
  label: string
  used: number
  limit: number
  /** Shown after the label, such as a Preview badge. */
  tag?: ReactNode
}) {
  const state = limitState(used, limit)
  const tone = TONE[state.kind]
  const width = state.kind === 'ok' || state.kind === 'almost' ? state.pct : 100

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex items-center justify-between gap-2 text-[13px]">
        <span className="flex min-w-0 items-center gap-2 font-bold">
          {label}
          {tag}
        </span>
        <span className="text-muted-foreground tabular-nums">
          <span className={cn('font-bold', tone.count)}>{count.format(used)}</span> /{' '}
          {state.kind === 'unlimited' ? 'Unlimited' : count.format(limit)}
        </span>
      </div>
      <div aria-hidden="true" className="h-2 rounded-sm bg-secondary">
        {state.kind !== 'unlimited' && (
          <div className={cn('h-2 rounded-sm', tone.bar)} style={{ width: `${width}%` }} />
        )}
      </div>
      <span className={cn('text-xs', tone.note)}>{note(state, used, limit)}</span>
    </div>
  )
}
