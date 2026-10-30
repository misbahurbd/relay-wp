import { Clock3Icon } from 'lucide-react'
import { cn } from '#/lib/utils'
import type { WindowState } from '../kit/window'

const TONE = {
  open: { bar: 'bg-brand-soft text-brand-text', fill: 'bg-brand-text' },
  urgent: { bar: 'bg-destructive-soft text-destructive', fill: 'bg-destructive' },
  awaiting: { bar: 'bg-warning-soft text-warning', fill: 'bg-warning' },
  closed: { bar: 'bg-neutral-soft text-neutral', fill: 'bg-neutral' },
}

/** The window countdown (F5); shown only on a channel with a window (window_hours > 0). */
export function WindowBar({ state }: { state: WindowState }) {
  if (!state.limited) return null
  const tone = TONE[state.tone]
  return (
    <div
      className={cn(
        'flex flex-wrap items-center gap-x-3.5 gap-y-2 px-3 py-2.5 text-[13px] sm:px-[18px]',
        tone.bar,
      )}
    >
      <Clock3Icon className="size-4" aria-hidden="true" />
      <span className="font-bold">{state.title}</span>
      <span className="flex-[1_1_200px] text-muted-foreground">{state.hint}</span>
      <div
        role="progressbar"
        aria-label={`Time left in the ${state.label}`}
        aria-valuemin={0}
        aria-valuemax={100}
        aria-valuenow={Math.round(state.pct)}
        className="h-1.5 w-[120px] rounded-full bg-card"
      >
        <div className={cn('h-1.5 rounded-full', tone.fill)} style={{ width: `${state.pct}%` }} />
      </div>
    </div>
  )
}
