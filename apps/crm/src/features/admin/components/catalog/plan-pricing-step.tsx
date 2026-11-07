import { Alert, AlertDescription } from '#/components/ui/alert'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { Switch } from '#/components/ui/switch'
import type { Cadence } from '../../api/plan-publish'
import type { Provider, ProviderCaps } from '../../types'
import { formatPrice, yearlySaving } from '../kit/money'
import { CADENCES, CURRENCY, type Errors, minorOf, type PlanDraft } from './plan-draft'
import { PlanTrialFields } from './plan-trial-fields'

const HINTS: Record<Cadence, string> = {
  month: 'Charged every month.',
  year: 'Charged once a year.',
  lifetime: 'One payment, the plan for life. Leave empty if you do not sell a lifetime deal.',
}

function Section({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: React.ReactNode
}) {
  return (
    <section className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-sm font-extrabold">{title}</h3>
        {description && <p className="text-xs text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  )
}

export function PlanPricingStep({
  draft,
  errors,
  enabled,
  sellWith,
  caps,
  created,
  trialLocked,
  onChange,
}: {
  draft: PlanDraft
  errors: Errors
  enabled: string[]
  /** Providers the prices can be put on sale with. */
  sellWith: Provider[]
  caps: ProviderCaps[] | undefined
  /** Prices a retry already created: they cannot change. */
  created: Cadence[]
  trialLocked: boolean
  onChange: (patch: Partial<PlanDraft>) => void
}) {
  const month = minorOf(draft.prices.month)
  const year = minorOf(draft.prices.year)
  const saving = month && year ? yearlySaving(month, year) : 0
  const noRecurring = !draft.prices.month.trim() && !draft.prices.year.trim()

  return (
    <div className="mx-auto flex w-full max-w-[640px] flex-col gap-7">
      <Section
        title="Prices"
        description={`In ${CURRENCY}. Fill the ones you sell; the rest stay off. Prices never change once sold.`}
      >
        <div className="grid gap-4 sm:grid-cols-2">
          {CADENCES.map((c) => {
            const id = `plan-price-${c.id}`
            const note =
              c.id === 'year' && year
                ? `${formatPrice(Math.round(year / 12), CURRENCY)} a month${saving > 0 ? ` · Save ${saving}% vs monthly` : ''}`
                : HINTS[c.id]
            const error = errors[c.id]
            return (
              <div
                key={c.id}
                className={`flex min-w-0 flex-col gap-1.5 ${c.id === 'lifetime' ? 'sm:col-span-2' : ''}`}
              >
                <Label htmlFor={id}>
                  {c.id === 'lifetime' ? 'Lifetime price (optional)' : `${c.label} price`}
                </Label>
                <div className="relative">
                  <span
                    aria-hidden="true"
                    className="pointer-events-none absolute inset-y-0 left-3.5 grid place-content-center text-sm text-muted-foreground"
                  >
                    $
                  </span>
                  <Input
                    id={id}
                    inputMode="decimal"
                    autoComplete="off"
                    placeholder={c.id === 'month' ? '29' : c.id === 'year' ? '290' : '199'}
                    value={draft.prices[c.id]}
                    disabled={created.includes(c.id)}
                    onChange={(e) =>
                      onChange({ prices: { ...draft.prices, [c.id]: e.target.value } })
                    }
                    aria-invalid={error ? true : undefined}
                    aria-describedby={`${id}-note`}
                    className="pl-7"
                  />
                </div>
                <p
                  id={`${id}-note`}
                  className={`text-xs ${error ? 'text-destructive' : saving > 0 && c.id === 'year' ? 'font-bold text-brand-text' : 'text-muted-foreground'}`}
                >
                  {created.includes(c.id) ? 'Already added.' : (error ?? note)}
                </p>
              </div>
            )
          })}
        </div>
        {noRecurring && (
          <Alert variant="info">
            <AlertDescription>
              <p>
                Add a monthly or a yearly price so people can subscribe. You can also add prices
                later from the plan card.
              </p>
            </AlertDescription>
          </Alert>
        )}
      </Section>

      <Section title="Selling">
        <div className="flex items-start justify-between gap-4 rounded-lg border bg-card p-4">
          <div className="flex min-w-0 flex-col gap-0.5">
            <Label htmlFor="plan-sell">
              {sellWith.length > 0
                ? `Put these prices on sale with ${sellWith.join(', ')} now`
                : 'Put these prices on sale now'}
            </Label>
            <p id="plan-sell-note" className="text-xs text-muted-foreground">
              {sellWith.length > 0
                ? 'Relay creates the product and price on each provider. Switch it off to check the prices first and sell them later from the plan card.'
                : enabled.length === 0
                  ? 'No payment provider is enabled on this server (set PAYMENT_PROVIDERS). Prices are saved and can be put on sale later.'
                  : 'None of the providers enabled on this server can create products from here. Prices are saved and can be put on sale later.'}
            </p>
          </div>
          <Switch
            id="plan-sell"
            checked={draft.sell && sellWith.length > 0}
            disabled={sellWith.length === 0}
            onCheckedChange={(sell) => onChange({ sell })}
            aria-describedby="plan-sell-note"
          />
        </div>
      </Section>

      <Section title="Trial">
        <PlanTrialFields
          draft={draft}
          errors={errors}
          enabled={enabled}
          caps={caps}
          disabled={trialLocked}
          onChange={onChange}
        />
      </Section>
    </div>
  )
}
