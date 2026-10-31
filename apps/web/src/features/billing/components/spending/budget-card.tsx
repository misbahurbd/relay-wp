import { useId, useState } from 'react'
import { Input } from '#/components/ui/input'
import { cn } from '#/lib/utils'
import type { SpendingBudget } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { formatRoughMoney, minorDigits, parseMajor } from './amounts'
import { monthName, shortDay } from './dates'
import { SwitchRow } from './switch-row'

/** Share of the budget at which the card warns. */
const WARN_AT = 0.8

/** This month's budget. The field and switches work, but nothing is saved while this is a preview. */
export function BudgetCard({ budget, className }: { budget: SpendingBudget; className?: string }) {
  const id = useId()
  const { currency } = budget
  const [text, setText] = useState(String(budget.budget_minor / 10 ** minorDigits(currency)))
  const [alertAt80, setAlertAt80] = useState(budget.alert_at_80)
  const [pauseMarketing, setPauseMarketing] = useState(budget.pause_marketing_at_100)

  const money = (minor: number) => formatMoney(minor, currency)
  const limit = parseMajor(text, currency) ?? 0
  const spent = budget.spent_minor
  const share = limit > 0 ? spent / limit : 0
  const state = share >= 1 || limit <= 0 ? 'over' : share >= WARN_AT ? 'close' : 'ok'

  const message =
    state === 'over'
      ? `Over budget by ${money(spent - limit)}`
      : state === 'close'
        ? `Heads up: ${money(limit - spent)} left · projected ${formatRoughMoney(budget.projected_minor, currency)} by ${shortDay(budget.projected_to)}`
        : `${money(limit - spent)} left this month`

  return (
    <Section aria-labelledby={`${id}-title`} className={cn('sm:py-5', className)}>
      <div className="flex flex-col gap-0.5">
        <h2 id={`${id}-title`} className="text-[17px] font-bold">
          Monthly budget
        </h2>
        <span className="text-[13px] text-muted-foreground">
          {monthName(budget.month)} · plan + Meta charges
        </span>
      </div>

      <label className="flex flex-col gap-1.5 text-[13px] font-semibold">
        Budget for this month
        <span className="flex h-11 items-center rounded-[10px] border bg-muted px-3 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring">
          <span className="font-bold text-muted-foreground">$</span>
          <Input
            unstyled
            type="number"
            min={0}
            step={10}
            value={text}
            onChange={(e) => setText(e.target.value)}
            className="min-w-0 flex-1 border-0 bg-transparent pl-1.5 text-base font-bold text-foreground outline-none"
          />
        </span>
      </label>

      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap items-baseline justify-between gap-2">
          <span className="text-[22px] font-extrabold tabular-nums">{money(spent)}</span>
          <span className="text-[13px] text-muted-foreground">
            of {money(limit)} · {Math.round(share * 100)}% used
          </span>
        </div>
        <div aria-hidden="true" className="h-2.5 overflow-hidden rounded-[5px] bg-secondary">
          <div
            className={cn(
              'h-2.5 rounded-[5px]',
              state === 'over'
                ? 'bg-destructive'
                : state === 'close'
                  ? 'bg-warning-bar'
                  : 'bg-brand',
            )}
            style={{ width: `${Math.min(100, share * 100).toFixed(1)}%` }}
          />
        </div>
        <span
          aria-live="polite"
          className={cn(
            'text-[13px] font-bold',
            state === 'over'
              ? 'text-destructive'
              : state === 'close'
                ? 'text-warning'
                : 'text-brand-text',
          )}
        >
          {message}
        </span>
      </div>

      <div className="flex flex-col gap-3 border-t pt-1">
        <SwitchRow
          className="pt-2.5"
          label="Email me at 80% of budget"
          hint={`Sent to ${budget.alert_email} and billing admins`}
          checked={alertAt80}
          onChange={setAlertAt80}
        />
        <SwitchRow
          className="pt-2.5"
          label="Pause marketing broadcasts at 100%"
          hint="Order updates and login codes keep sending"
          checked={pauseMarketing}
          onChange={setPauseMarketing}
        />
      </div>
    </Section>
  )
}
