import { useId } from 'react'
import { Bar, BarChart, CartesianGrid, XAxis, YAxis } from 'recharts'
import {
  type ChartConfig,
  ChartContainer,
  ChartTooltip,
  ChartTooltipContent,
} from '#/components/ui/chart'
import type { Spending } from '../../types'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { formatWholeMoney } from './amounts'
import { CATEGORY, CategoryLabel } from './category-label'
import { monthName, shortDay } from './dates'

const PAID = ['marketing', 'utility', 'authentication'] as const

const config = {
  marketing: { label: CATEGORY.marketing.label, color: CATEGORY.marketing.color },
  utility: { label: CATEGORY.utility.label, color: CATEGORY.utility.color },
  authentication: { label: CATEGORY.authentication.label, color: CATEGORY.authentication.color },
} satisfies ChartConfig

/** Meta charges per day (or week), stacked by category. */
export function SpendChart({ spending: s }: { spending: Spending }) {
  const id = useId()
  const money = (minor: number) => formatMoney(minor, s.currency)
  const daily = s.series.unit === 'day'
  const title = daily ? `Daily spend, ${monthName(s.from)}` : 'Weekly spend, last 3 months'
  const totals = Object.fromEntries(s.categories.map((c) => [c.category, c.cost_minor])) as Record<
    string,
    number
  >
  const data = s.series.points.map((p) => ({
    date: p.date,
    marketing: p.marketing_minor,
    utility: p.utility_minor,
    authentication: p.authentication_minor,
  }))
  const summary = `Stacked bar chart of Meta spend per ${s.series.unit}: ${PAID.map(
    (k) => `${CATEGORY[k].label} ${money(totals[k] ?? 0)}`,
  ).join(', ')}. Marketing makes up most of the spend and peaks on broadcast days.`

  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader
        id={`${id}-title`}
        title={title}
        description={`${shortDay(s.from)} – ${shortDay(s.to)} · Meta charges by category`}
        action={
          <ul className="flex flex-wrap gap-x-4 gap-y-1 text-[13px] font-semibold">
            {PAID.map((k) => (
              <li key={k}>
                <CategoryLabel
                  category={k}
                  label={`${CATEGORY[k].label} ${money(totals[k] ?? 0)}`}
                />
              </li>
            ))}
          </ul>
        }
      />
      <figure className="m-0 flex flex-col gap-2">
        <ChartContainer
          config={config}
          role="img"
          aria-label={summary}
          className="aspect-auto h-[240px] w-full"
        >
          <BarChart
            data={data}
            margin={{ top: 8, right: 0, left: 0, bottom: 0 }}
            // The figure is one labelled image; the numbers are in the category table below.
            accessibilityLayer={false}
          >
            <CartesianGrid vertical={false} />
            <XAxis
              dataKey="date"
              tickLine={false}
              axisLine={false}
              tickMargin={8}
              minTickGap={24}
              tickFormatter={(v: string) => shortDay(v)}
            />
            <YAxis
              tickLine={false}
              axisLine={false}
              width={52}
              tickFormatter={(v: number) => formatWholeMoney(v, s.currency)}
            />
            <ChartTooltip
              cursor={false}
              content={
                <ChartTooltipContent
                  labelFormatter={(v) =>
                    daily ? shortDay(String(v)) : `Week of ${shortDay(String(v))}`
                  }
                  formatter={(value, name) => (
                    <div className="flex w-full items-center justify-between gap-3">
                      <CategoryLabel
                        category={name as (typeof PAID)[number]}
                        className="text-muted-foreground"
                      />
                      <span className="font-medium tabular-nums text-foreground">
                        {money(Number(value))}
                      </span>
                    </div>
                  )}
                />
              }
            />
            {PAID.map((k, i) => (
              <Bar
                key={k}
                dataKey={k}
                stackId="spend"
                fill={`var(--color-${k})`}
                radius={i === PAID.length - 1 ? [4, 4, 0, 0] : 0}
                isAnimationActive={false}
              />
            ))}
          </BarChart>
        </ChartContainer>
        <figcaption className="text-xs text-muted-foreground">
          Service conversations (customer-started replies) are free and not shown. Tall bars are
          broadcast days.
        </figcaption>
      </figure>
    </Section>
  )
}
