import { CheckIcon } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import { Badge } from '#/components/ui/badge'
import { Label } from '#/components/ui/label'
import { RadioGroup, RadioGroupItem } from '#/components/ui/radio-group'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import { cn } from '#/lib/utils'
import type { Cadence, Catalog, Plan, Price, Selection } from '../../types'
import { describeInstallments } from '../kit/installments'
import { formatMoney, limitLines, perInterval, yearlySaving } from '../kit/money'

const PAY_ONCE = 'once'

export function priceFor(plan: Plan, cadence: Cadence): Price | undefined {
  return plan.prices.find((p) =>
    cadence === 'lifetime' ? p.billing_type === 'one_time' : p.interval === cadence,
  )
}

/** The cadences at least one plan sells, in toggle order. */
export function cadencesOf(catalog: Catalog): Cadence[] {
  const all: Cadence[] = ['month', 'year', 'lifetime']
  return all.filter((c) => catalog.plans.some((plan) => priceFor(plan, c)))
}

export type PlanAction = (input: {
  plan: Plan
  price: Price
  selection: Selection
  current: boolean
}) => ReactNode

type Props = {
  catalog: Catalog
  cadence: Cadence
  onCadence: (cadence: Cadence) => void
  /** The price this workspace pays now, marked "Current plan". */
  currentPriceId?: string
  action: PlanAction
}

export function PricingTable({ catalog, cadence, onCadence, currentPriceId, action }: Props) {
  const cadences = cadencesOf(catalog)
  const plans = catalog.plans.filter((plan) => priceFor(plan, cadence))
  const bestSaving = Math.max(
    0,
    ...catalog.plans.map((p) => yearlySaving(priceFor(p, 'month'), priceFor(p, 'year'))),
  )

  return (
    <div className="flex flex-col gap-6">
      {cadences.length > 1 && (
        <ToggleGroup
          type="single"
          variant="outline"
          value={cadence}
          // Radix lets a pressed item be unpressed; keep one cadence chosen.
          onValueChange={(v) => v && onCadence(v as Cadence)}
          aria-label="Billing period"
          className="self-center"
        >
          {cadences.map((c) => (
            <ToggleGroupItem key={c} value={c} className="h-11 px-4 font-bold">
              {c === 'month' ? 'Monthly' : c === 'year' ? 'Yearly' : 'Lifetime'}
              {c === 'year' && bestSaving > 0 && (
                <span className="text-xs font-bold text-brand-text">Save up to {bestSaving}%</span>
              )}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      )}

      <ul className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {plans.map((plan) => {
          const price = priceFor(plan, cadence)
          if (!price) return null
          return (
            <li key={plan.id} className="flex">
              <PlanCard
                plan={plan}
                price={price}
                saving={cadence === 'year' ? yearlySaving(priceFor(plan, 'month'), price) : 0}
                current={price.id === currentPriceId}
                action={action}
              />
            </li>
          )
        })}
      </ul>
    </div>
  )
}

function PlanCard({
  plan,
  price,
  saving,
  current,
  action,
}: {
  plan: Plan
  price: Price
  saving: number
  current: boolean
  action: PlanAction
}) {
  const offers = price.installment_offers
  const [offerId, setOfferId] = useState(PAY_ONCE)
  const offer = offers.find((o) => o.id === offerId)
  const headingId = useId()
  const selection: Selection = { priceId: price.id, offerId: offer?.id }

  return (
    <article
      aria-labelledby={headingId}
      className={cn(
        'flex w-full flex-col gap-5 rounded-xl border bg-card p-5 shadow-card sm:p-6',
        current && 'border-primary',
      )}
    >
      <header className="flex flex-col gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <h3 id={headingId} className="text-lg font-extrabold tracking-tight">
            {plan.name}
          </h3>
          {current && <Badge variant="success">Current plan</Badge>}
          {saving > 0 && <Badge variant="info">Save {saving}%</Badge>}
        </div>
        <p className="flex flex-wrap items-baseline gap-x-1.5">
          <span className="text-[28px] font-extrabold tracking-[-0.02em]">
            {formatMoney(price.amount_minor, price.currency)}
          </span>
          <span className="text-[13px] text-muted-foreground">{perInterval(price)}</span>
        </p>
        {price.billing_type === 'recurring' && plan.trial_days > 0 && (
          <p className="text-[13px] text-muted-foreground">
            {plan.trial_days}-day free trial
            {plan.trial_requires_card ? ', card required' : ', no card needed'}
          </p>
        )}
        {price.billing_type === 'one_time' && (
          <p className="text-[13px] text-muted-foreground">Pay once, keep the plan for life.</p>
        )}
      </header>

      <ul className="flex flex-col gap-2 text-sm">
        {limitLines(plan.limits).map((line) => (
          <li key={line} className="flex items-center gap-2">
            <CheckIcon className="size-4 flex-none text-brand-text" aria-hidden="true" />
            {line}
          </li>
        ))}
      </ul>

      {offers.length > 0 && (
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-2 text-[13px] font-bold">How you pay</legend>
          <RadioGroup value={offerId} onValueChange={setOfferId} className="gap-2">
            <OfferOption id={`${headingId}-once`} value={PAY_ONCE} selected={offerId === PAY_ONCE}>
              <span className="font-bold">Pay once</span>
              <span className="text-[13px] text-muted-foreground">
                {formatMoney(price.amount_minor, price.currency)} today
              </span>
            </OfferOption>
            {offers.map((o) => {
              const markup = o.total_minor - price.amount_minor
              return (
                <OfferOption
                  key={o.id}
                  id={`${headingId}-${o.id}`}
                  value={o.id}
                  selected={offerId === o.id}
                >
                  <span className="font-bold">
                    {describeInstallments(o.amounts_minor, price.currency)}
                  </span>
                  <span className="text-[13px] text-muted-foreground">
                    {formatMoney(o.total_minor, price.currency)} in total
                    {markup > 0 &&
                      ` (${formatMoney(markup, price.currency)} more than paying once)`}
                  </span>
                </OfferOption>
              )
            })}
          </RadioGroup>
        </fieldset>
      )}

      <div className="mt-auto flex flex-col gap-2">
        {action({ plan, price, selection, current })}
      </div>
    </article>
  )
}

function OfferOption({
  id,
  value,
  selected,
  children,
}: {
  id: string
  value: string
  selected: boolean
  children: ReactNode
}) {
  return (
    <Label
      htmlFor={id}
      className={cn(
        'flex cursor-pointer items-start gap-3 rounded-lg border border-input p-3 font-normal leading-snug',
        selected && 'border-primary bg-muted',
      )}
    >
      <RadioGroupItem id={id} value={value} className="mt-0.5" />
      <span className="flex flex-col gap-0.5">{children}</span>
    </Label>
  )
}
