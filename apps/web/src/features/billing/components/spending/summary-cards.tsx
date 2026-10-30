import type { Spending } from '../../types'
import { formatMicros, formatMoney } from '../kit/money'
import { formatChange, formatCount } from './amounts'
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

export function SummaryCards({ spending: s }: { spending: Spending }) {
  const money = (minor: number) => formatMoney(minor, s.currency)
  const top = priciest(s)
  const cards = [
    {
      label: totalLabel(s),
      value: money(s.plan_minor + s.meta_minor),
      note: `Plan ${money(s.plan_minor)} + Meta ${money(s.meta_minor)}`,
    },
    {
      label: 'Meta message charges',
      value: money(s.meta_minor),
      note: `${formatCount(s.paid_messages)} paid messages · ${formatCount(s.free_messages)} free`,
    },
    { label: outlookLabel(s), value: money(s.outlook.amount_minor), note: s.outlook.note },
    {
      label: 'Avg. cost per paid message',
      value: formatMicros(s.avg_micros, s.currency),
      note: top ? `${CATEGORY[top].label} costs most per message` : '',
    },
    {
      label: `vs. ${s.change.against}`,
      value: formatChange(s.change.permille),
      note: s.change.note,
    },
  ]

  return (
    <section aria-label="Spending summary">
      <dl className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-4">
        {cards.map((c) => (
          <div
            key={c.label}
            className="flex min-w-0 flex-col gap-1 rounded-xl border bg-card p-5 shadow-card"
          >
            <dt className="text-[13px] font-semibold text-muted-foreground">{c.label}</dt>
            <dd className="text-2xl font-extrabold tracking-tight tabular-nums">{c.value}</dd>
            {c.note && <dd className="text-xs text-muted-foreground">{c.note}</dd>}
          </div>
        ))}
      </dl>
    </section>
  )
}
