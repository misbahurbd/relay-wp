import { CheckIcon } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import { Badge } from '#/components/ui/badge'
import { Label } from '#/components/ui/label'
import { RadioGroup, RadioGroupItem } from '#/components/ui/radio-group'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import { cn } from '#/lib/utils'
import type { Cadence, Catalog, Plan, Price, Selection } from '../../types'
import { describeInstallments } from '../kit/installments'
import { formatMoney, formatPrice, limitLines, yearlySaving } from '../kit/money'

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
  /** Places the cadence toggle, e.g. beside a section title. Without it the toggle is centred. */
  header?: (toggle: ReactNode) => ReactNode
}

export function PricingTable({
  catalog,
  cadence,
  onCadence,
  currentPriceId,
  action,
  header,
}: Props) {
  const cadences = cadencesOf(catalog)
  const plans = catalog.plans.filter((plan) => priceFor(plan, cadence))
  const bestSaving = Math.max(
    0,
    ...catalog.plans.map((p) => yearlySaving(priceFor(p, 'month'), priceFor(p, 'year'))),
  )

  const toggle = cadences.length > 1 && (
    <ToggleGroup
      type="single"
      value={cadence}
      // Radix lets a pressed item be unpressed; keep one cadence chosen.
      onValueChange={(v) => v && onCadence(v as Cadence)}
      aria-label="Billing period"
      className="rounded-lg bg-muted p-1"
    >
      {cadences.map((c) => (
        <ToggleGroupItem
          key={c}
          value={c}
          className="h-9 gap-1.5 rounded-md border-0 px-3.5 text-[13px] font-bold text-foreground data-[state=on]:bg-card data-[state=on]:shadow-sm"
        >
          {c === 'month' ? 'Monthly' : c === 'year' ? 'Yearly' : 'Lifetime'}
          {c === 'year' && bestSaving > 0 && (
            <span className="rounded-sm bg-brand-soft px-1.5 py-px text-[11px] text-brand-text">
              Save {bestSaving}%
            </span>
          )}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )

  return (
    <div className="flex flex-col gap-4">
      {header ? header(toggle) : toggle && <div className="self-center">{toggle}</div>}
      <ul className="grid grid-cols-[repeat(auto-fit,minmax(260px,1fr))] gap-4">
        {plans.map((plan) => {
          const price = priceFor(plan, cadence)
          if (!price) return null
          return (
            <li key={plan.id} className="flex">
              <PlanCard
                plan={plan}
                price={price}
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
  current,
  action,
}: {
  plan: Plan
  price: Price
  current: boolean
  action: PlanAction
}) {
  const offers = price.installment_offers
  const [offerId, setOfferId] = useState(PAY_ONCE)
  const offer = offers.find((o) => o.id === offerId)
  const headingId = useId()
  const selection: Selection = { priceId: price.id, offerId: offer?.id }
  const features = plan.features.length > 0 ? plan.features : limitLines(plan.limits)
  const yearly = price.interval === 'year'
  // Yearly prices show what they come to each month, as the design does.
  const shown = yearly ? Math.round(price.amount_minor / 12) : price.amount_minor

  return (
    <article
      aria-labelledby={headingId}
      className={cn(
        'flex w-full flex-col gap-3.5 rounded-xl bg-card p-5',
        current ? 'border-2 border-brand' : 'border',
      )}
    >
      <header className="flex min-h-[26px] items-center justify-between gap-2">
        <h3 id={headingId} className="text-lg font-extrabold">
          {plan.name}
        </h3>
        {current ? (
          <Badge variant="success" className="rounded-full px-2.5">
            Current plan
          </Badge>
        ) : (
          plan.recommended && (
            <Badge variant="info" className="rounded-full px-2.5">
              Recommended
            </Badge>
          )
        )}
      </header>
      {plan.description && (
        <p className="min-h-[38px] text-[13px] text-muted-foreground">{plan.description}</p>
      )}

      <div className="flex flex-col gap-0.5">
        <p className="flex flex-wrap items-baseline gap-x-1.5">
          <span className="text-[34px] leading-tight font-extrabold tracking-[-0.02em]">
            {formatPrice(shown, price.currency)}
          </span>
          <span className="text-muted-foreground">
            {price.billing_type === 'one_time' ? 'once' : '/ month'}
          </span>
        </p>
        <p className="text-xs text-muted-foreground">{billedNote(plan, price)}</p>
      </div>

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

      <div className="flex flex-col gap-2">{action({ plan, price, selection, current })}</div>

      <ul className="flex flex-col gap-[9px] text-[13px]">
        {features.map((line) => (
          <li key={line} className="flex items-start gap-2">
            <CheckIcon className="mt-0.5 size-4 flex-none text-brand" aria-hidden="true" />
            <span>{line}</span>
          </li>
        ))}
      </ul>
    </article>
  )
}

/** The small line under a price: how and when it is billed. */
function billedNote(plan: Plan, price: Price) {
  if (price.billing_type === 'one_time') return 'Pay once, keep the plan for life.'
  const trial =
    plan.trial_days > 0
      ? `${plan.trial_days}-day free trial${plan.trial_requires_card ? ', card required' : ''}, then `
      : ''
  const billed =
    price.interval === 'year'
      ? `billed ${formatPrice(price.amount_minor, price.currency)} yearly`
      : 'billed monthly, cancel anytime'
  const note = trial + billed
  return note[0].toUpperCase() + note.slice(1)
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
