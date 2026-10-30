import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'

const count = new Intl.NumberFormat()

/** Share of a limit at which a meter warns. */
export const ALMOST_FULL = 0.8

/** Whether used is close enough to limit to warn. limit -1 is unlimited. */
export function almostFull(used: number, limit: number) {
  return limit > 0 && used / limit >= ALMOST_FULL
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
  const unlimited = limit < 0
  const share = unlimited || limit === 0 ? 0 : Math.min(used / limit, 1)
  const pct = Math.round(share * 100)
  const hot = almostFull(used, limit)

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex items-center justify-between gap-2 text-[13px]">
        <span className="flex min-w-0 items-center gap-2 font-bold">
          {label}
          {tag}
        </span>
        <span className="text-muted-foreground tabular-nums">
          <span className={cn('font-bold', hot ? 'text-warning' : 'text-foreground')}>
            {count.format(used)}
          </span>{' '}
          / {unlimited ? 'Unlimited' : count.format(limit)}
        </span>
      </div>
      <div aria-hidden="true" className="h-2 rounded-sm bg-secondary">
        {!unlimited && (
          <div
            className={cn('h-2 rounded-sm', hot ? 'bg-warning-bar' : 'bg-brand')}
            style={{ width: `${pct}%` }}
          />
        )}
      </div>
      <span className={cn('text-xs', hot ? 'text-warning' : 'text-muted-foreground')}>
        {unlimited
          ? 'No limit on your plan'
          : hot
            ? `${pct}% used. Upgrade to avoid interruptions.`
            : `${pct}% used`}
      </span>
    </div>
  )
}
