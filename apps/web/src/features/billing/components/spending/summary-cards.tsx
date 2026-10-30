import type { ReactNode } from 'react'
import type { Spending } from '../../types'
import { formatMicros, formatMoney } from '../kit/money'
import { formatChange, formatCount, formatRoughMoney } from './amounts'
import { CATEGORY } from './category-label'
import { monthEnd, monthName, shortDay } from './dates'

function totalLabel(s: Spending) {
  if (s.period === 'this_month') return 'Total spent this month'
  if (s.period === 'last_month') return `Total spent in ${monthName(s.from)}`
  return `Total spent, ${monthName(s.from, 'short')} – ${monthName(s.to, 'short')}`
}

function outlookLabel(s: Spending) {
  if (s.outlook.kind === 'projected') return `Projected by ${shortDay(monthEnd(s.to))}`
  if (s.outlook.kind === 'final') return `Final for ${monthName(s.from)}`
  return 'Monthly average'
}

/** The paid category with the highest average charge per message. */
function priciest(s: Spending) {
  const paid = s.categories.filter((c) => c.avg_micros > 0)
  if (!paid.length) return null
  return paid.reduce((a, b) => (b.avg_micros > a.avg_micros ? b : a)).category
}

function Card({ label, value, children }: { label: string; value: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-2 rounded-xl border bg-card p-[18px] shadow-card">
      <dt className="text-[13px] font-semibold text-muted-foreground">{label}</dt>
      <dd className="text-[28px] leading-[1.1] font-extrabold tracking-[-0.02em] tabular-nums">
        {value}
      </dd>
      {children}
    </div>
  )
}

export function SummaryCards({ spending: s }: { spending: Spending }) {
  const money = (minor: number) => formatMoney(minor, s.currency)
  const top = priciest(s)
  return (
    <section aria-label="Spending summary">
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-3.5">
        <Card label={totalLabel(s)} value={money(s.plan_minor + s.meta_minor)}>
          <dd className="text-xs text-muted-foreground">
            Plan {money(s.plan_minor)} + Meta {money(s.meta_minor)}
          </dd>
        </Card>
        <Card label="Meta message charges" value={money(s.meta_minor)}>
          <dd className="text-xs text-muted-foreground">
            {formatCount(s.paid_messages)} paid messages · {formatCount(s.free_messages)} free
          </dd>
        </Card>
        <Card label={outlookLabel(s)} value={formatRoughMoney(s.outlook.amount_minor, s.currency)}>
          <dd className="text-xs text-muted-foreground">{s.outlook.note}</dd>
        </Card>
        <Card label="Avg. cost per paid message" value={formatMicros(s.avg_micros, s.currency)}>
          {top && (
            <dd className="text-xs text-muted-foreground">
              {CATEGORY[top].label} costs most per message
            </dd>
          )}
        </Card>
        <Card label={`vs. ${s.change.against}`} value={formatChange(s.change.permille)}>
          <dd className="self-start rounded-full bg-warning-soft px-2 py-0.5 text-xs font-bold text-warning">
            {s.change.note}
          </dd>
        </Card>
      </dl>
    </section>
  )
}
