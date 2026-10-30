import { useId, useState } from 'react'
import { cn } from '#/lib/utils'
import type { SpendingBudget } from '../../types'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { minorDigits, parseMajor } from './amounts'
import { monthName, shortDay } from './dates'
import { SwitchRow } from './switch-row'

/** Share of the budget at which the card warns. */
const WARN_AT = 0.8

/** This month's budget. The field and switches work, but nothing is saved while this is a preview. */
export function BudgetCard({ budget }: { budget: SpendingBudget }) {
  const id = useId()
  const { currency } = budget
  const [text, setText] = useState(String(budget.budget_minor / 10 ** minorDigits(currency)))
  const [alertAt80, setAlertAt80] = useState(budget.alert_at_80)
  const [pauseMarketing, setPauseMarketing] = useState(budget.pause_marketing_at_100)

  const money = (minor: number) => formatMoney(minor, currency)
  const limit = parseMajor(text, currency) ?? 0
  const spent = budget.spent_minor
  const share = limit > 0 ? spent / limit : 0
  const state = limit <= 0 ? 'none' : share >= 1 ? 'over' : share >= WARN_AT ? 'close' : 'ok'

  const message =
    state === 'none'
      ? 'Set a budget to get alerts.'
      : state === 'over'
        ? `Over budget by ${money(spent - limit)}`
        : state === 'close'
          ? `Heads up: ${money(limit - spent)} left · projected ${money(budget.projected_minor)} by ${shortDay(budget.projected_to)}`
          : `${money(limit - spent)} left this month`

  return (
    <Section aria-labelledby={`${id}-title`}>
      <SectionHeader
        id={`${id}-title`}
        title="Monthly budget"
        description={`${monthName(budget.month)} · plan + Meta charges`}
      />

      <div className="flex flex-col gap-1.5">
        <label htmlFor={`${id}-amount`} className="text-[13px] font-semibold">
          Budget for this month
        </label>
        <div className="flex h-11 items-center gap-1 rounded-lg border border-input bg-background px-3 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring">
          <span aria-hidden="true" className="font-bold text-muted-foreground">
            $
          </span>
          <input
            id={`${id}-amount`}
            type="number"
            inputMode="decimal"
            min={0}
            step={10}
            value={text}
            onChange={(e) => setText(e.target.value)}
            className="min-w-0 flex-1 bg-transparent font-bold tabular-nums outline-none"
          />
        </div>
      </div>

      <div className="flex flex-col gap-1.5">
        <p className="flex flex-wrap items-baseline gap-x-2">
          <span className="text-xl font-extrabold tabular-nums">{money(spent)}</span>
          <span className="text-[13px] text-muted-foreground">
            {limit > 0 ? `of ${money(limit)} · ${Math.round(share * 100)}% used` : 'spent so far'}
          </span>
        </p>
        <div aria-hidden="true" className="h-2 overflow-hidden rounded-full bg-muted">
          <div
            className={cn(
              'h-full rounded-full',
              state === 'over'
                ? 'bg-destructive'
                : state === 'close'
                  ? 'bg-warning-bar'
                  : 'bg-brand',
            )}
            style={{ width: `${Math.min(100, share * 100).toFixed(1)}%` }}
          />
        </div>
        <p
          aria-live="polite"
          className={cn(
            'text-[13px] font-semibold',
            state === 'over'
              ? 'text-destructive'
              : state === 'close'
                ? 'text-warning'
                : state === 'ok'
                  ? 'text-brand-text'
                  : 'text-muted-foreground',
          )}
        >
          {message}
        </p>
      </div>

      <div className="flex flex-col gap-4 border-t pt-4">
        <SwitchRow
          label="Email me at 80% of budget"
          hint={`Sent to ${budget.alert_email} and billing admins`}
          checked={alertAt80}
          onChange={setAlertAt80}
        />
        <SwitchRow
          label="Pause marketing broadcasts at 100%"
          hint="Order updates and login codes keep sending"
          checked={pauseMarketing}
          onChange={setPauseMarketing}
        />
        <p className="text-xs text-muted-foreground">
          Try the budget and alerts here. They aren’t saved yet.
        </p>
      </div>
    </Section>
  )
}
