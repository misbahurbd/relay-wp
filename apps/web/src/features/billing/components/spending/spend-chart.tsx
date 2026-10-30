import { useId } from 'react'
import { cn } from '#/lib/utils'
import type { Spending } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { formatWholeMoney } from './amounts'
import { CATEGORY } from './category-label'
import { monthName, shortDay } from './dates'

const PAID = ['marketing', 'utility', 'authentication'] as const

/** Axis labels: every week (every third week for a weekly chart), then the last point. */
function xLabels(dates: string[], daily: boolean) {
  if (dates.length === 0) return []
  const every = daily ? 7 : 3
  const picked = [0, 1, 2, 3].map((i) => i * every).filter((i) => i < dates.length - 1)
  return [...picked, dates.length - 1].map((i) => dates[i])
}

/** Meta charges per day (or week), stacked by category: authentication on top, marketing at the base. */
export function SpendChart({ spending: s, className }: { spending: Spending; className?: string }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, s.currency)
  const daily = s.series.unit === 'day'
  const title = daily ? `Daily spend, ${monthName(s.from)}` : 'Weekly spend, last 3 months'
  const totals = Object.fromEntries(s.categories.map((c) => [c.category, c.cost_minor])) as Record<
    string,
    number
  >
  // Round the tallest bar up to a whole step ($20 a day, $50 a week), as the design's axis does.
  const step = daily ? 2000 : 5000
  const tallest = Math.max(
    0,
    ...s.series.points.map((p) => p.marketing_minor + p.utility_minor + p.authentication_minor),
  )
  const top = Math.max(step, Math.ceil(tallest / step) * step)
  const pct = (minor: number) => `${((minor / top) * 100).toFixed(2)}%`
  const summary = `Stacked bar chart of Meta spend per ${s.series.unit}: ${PAID.map(
    (k) => `${CATEGORY[k].label} ${money(totals[k] ?? 0)}`,
  ).join(', ')}. Marketing makes up most of the spend and peaks on broadcast days.`

  return (
    <Section aria-labelledby={`${id}-title`} className={cn('gap-3.5 sm:py-5', className)}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="flex flex-col gap-0.5">
          <h2 id={`${id}-title`} className="text-[17px] font-bold">
            {title}
          </h2>
          <span className="text-[13px] text-muted-foreground">
            {shortDay(s.from)} – {shortDay(s.to)} · Meta charges by category
          </span>
        </div>
        <div className="flex flex-wrap gap-3.5 text-[13px] font-semibold">
          {PAID.map((k) => (
            <span key={k} className="flex items-center gap-1.5">
              <span className={cn('size-3 rounded-[3px]', CATEGORY[k].bg)} />
              {CATEGORY[k].label} {money(totals[k] ?? 0)}
            </span>
          ))}
        </div>
      </div>
      <div className="flex gap-2.5">
        <div
          aria-hidden="true"
          className="-my-2 flex h-[220px] min-w-10 flex-col justify-between text-right text-xs text-muted-foreground"
        >
          <span>{formatWholeMoney(top, s.currency)}</span>
          <span>{formatWholeMoney(top / 2, s.currency)}</span>
          <span>{formatWholeMoney(0, s.currency)}</span>
        </div>
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div
            role="img"
            aria-label={summary}
            className="relative flex h-[220px] items-end gap-[3px] border-b"
          >
            <div aria-hidden="true" className="absolute inset-x-0 top-0 border-t border-dashed" />
            <div aria-hidden="true" className="absolute inset-x-0 top-1/2 border-t border-dashed" />
            {s.series.points.map((p) => (
              <div
                key={p.date}
                title={`${daily ? shortDay(p.date) : `Week of ${shortDay(p.date)}`}: ${money(
                  p.marketing_minor + p.utility_minor + p.authentication_minor,
                )}`}
                className="relative flex h-full min-w-0 flex-1 flex-col justify-end"
              >
                <div
                  className="rounded-t-[3px] bg-cat-authentication"
                  style={{ height: pct(p.authentication_minor) }}
                />
                <div className="bg-cat-utility" style={{ height: pct(p.utility_minor) }} />
                <div className="bg-cat-marketing" style={{ height: pct(p.marketing_minor) }} />
              </div>
            ))}
          </div>
          <div aria-hidden="true" className="flex justify-between text-xs text-muted-foreground">
            {xLabels(
              s.series.points.map((p) => p.date),
              daily,
            ).map((d) => (
              <span key={d}>{shortDay(d)}</span>
            ))}
          </div>
        </div>
      </div>
      <span className="text-xs text-muted-foreground">
        Service conversations (customer-started replies) are free and not shown. Tall bars are
        broadcast days.
      </span>
    </Section>
  )
}
