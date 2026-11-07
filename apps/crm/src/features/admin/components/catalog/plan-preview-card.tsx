import { CheckIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { useState } from 'react'
import { Badge } from '#/components/ui/badge'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import type { Cadence } from '../../api/plan-publish'
import { formatPrice, yearlySaving } from '../kit/money'
import type { PlanSummary } from '../kit/plan-highlights'
import { CADENCES, CURRENCY, minorOf, type PlanDraft } from './plan-draft'

/** The customer's pricing card as it will read, from the same fields staff are filling in. */
export function PlanPreviewCard({ draft, summary }: { draft: PlanDraft; summary: PlanSummary }) {
  const cadences = CADENCES.filter((c) => minorOf(draft.prices[c.id]))
  const [picked, setPicked] = useState<Cadence>('month')
  const cadence = cadences.find((c) => c.id === picked)?.id ?? cadences[0]?.id
  const amount = cadence ? minorOf(draft.prices[cadence]) : undefined
  const month = minorOf(draft.prices.month)
  const year = minorOf(draft.prices.year)
  const saving = month && year ? yearlySaving(month, year) : 0
  const days = Number(draft.trialDays) || 0

  const trial =
    days > 0 ? `${days}-day free trial${draft.requiresCard ? ', card required' : ''}, then ` : ''
  let note = ''
  if (cadence === 'lifetime') note = 'Pay once, keep the plan for life.'
  else if (cadence === 'year' && amount)
    note = `${trial}billed ${formatPrice(amount, CURRENCY)} yearly`
  else if (cadence === 'month') note = `${trial}billed monthly, cancel anytime`
  note = note ? note[0].toUpperCase() + note.slice(1) : ''

  return (
    <div className="flex flex-col gap-3">
      {cadences.length > 1 && (
        <ToggleGroup
          type="single"
          value={cadence}
          onValueChange={(v) => v && setPicked(v as Cadence)}
          aria-label="Preview billing period"
          className="self-start rounded-lg bg-secondary p-1"
        >
          {cadences.map((c) => (
            <ToggleGroupItem
              key={c.id}
              value={c.id}
              className="h-8 gap-1.5 rounded-md border-0 px-3 text-[13px] text-foreground data-[state=on]:bg-card data-[state=on]:text-foreground data-[state=on]:shadow-sm"
            >
              {c.label}
              {c.id === 'year' && saving > 0 && (
                <span className="rounded-sm bg-brand-soft px-1.5 py-px text-[11px] text-brand-text">
                  Save {saving}%
                </span>
              )}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      )}
      <article
        aria-label="Pricing card preview"
        className="flex w-full flex-col gap-3.5 rounded-xl border bg-card p-5 shadow-card"
      >
        <header className="flex min-h-[26px] items-center justify-between gap-2">
          <h3
            className={`min-w-0 truncate text-lg font-extrabold ${draft.name.trim() ? '' : 'text-muted-foreground'}`}
          >
            {draft.name.trim() || 'Plan name'}
          </h3>
          {draft.recommended && (
            <Badge variant="info" className="rounded-full px-2.5">
              Recommended
            </Badge>
          )}
        </header>
        {draft.description.trim() && (
          <p className="text-[13px] text-muted-foreground">{draft.description.trim()}</p>
        )}
        <div className="flex flex-col gap-0.5">
          {amount && cadence ? (
            <>
              <p className="flex flex-wrap items-baseline gap-x-1.5">
                <span className="text-[34px] leading-tight font-extrabold tracking-[-0.02em]">
                  {formatPrice(cadence === 'year' ? Math.round(amount / 12) : amount, CURRENCY)}
                </span>
                <span className="text-muted-foreground">
                  {cadence === 'lifetime' ? 'once' : '/ month'}
                </span>
              </p>
              <p className="text-xs text-muted-foreground">{note}</p>
            </>
          ) : (
            <p className="py-1 text-sm text-muted-foreground">
              No price yet. Add one on the previous step.
            </p>
          )}
        </div>
        {summary.highlights.length + summary.extras.length > 0 || summary.heading ? (
          <div className="flex flex-col gap-2.5">
            {summary.heading && <p className="text-[13px] font-bold">{summary.heading}</p>}
            <ul className="flex flex-col gap-[9px] text-[13px]">
              {summary.highlights.map((line) => (
                <Feature key={`h:${line}`}>{line}</Feature>
              ))}
              {summary.extras.map((line, i) => (
                // biome-ignore lint/suspicious/noArrayIndexKey: lines can repeat while staff type
                <Feature key={`x${i}:${line}`}>{line}</Feature>
              ))}
            </ul>
          </div>
        ) : (
          <p className="text-[13px] text-muted-foreground">Nothing included yet.</p>
        )}
      </article>
    </div>
  )
}

function Feature({ children }: { children: ReactNode }) {
  return (
    <li className="flex items-start gap-2">
      <CheckIcon className="mt-0.5 size-4 flex-none text-brand" aria-hidden="true" />
      <span className="min-w-0 break-words">{children}</span>
    </li>
  )
}
