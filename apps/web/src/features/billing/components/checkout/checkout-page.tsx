import { type UseQueryResult, useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronLeftIcon } from 'lucide-react'
import { useId, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Label } from '#/components/ui/label'
import { RadioGroup, RadioGroupItem } from '#/components/ui/radio-group'
import { Separator } from '#/components/ui/separator'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useCreateCheckout } from '../../api/billing.mutations'
import {
  overviewQueryOptions,
  plansQueryOptions,
  quoteQueryOptions,
} from '../../api/billing.queries'
import { useIdempotencyKey } from '../../hooks/use-idempotency-key'
import { PROVIDER_LABEL } from '../../schemas/billing.schema'
import type {
  Catalog,
  DiscountPreview,
  InstallmentOffer,
  OrderQuote,
  Plan,
  Price,
  Provider,
  ProviderName,
  StartedCheckout,
} from '../../types'
import { formatDate } from '../kit/format'
import { formatMoney, limitLines, perInterval } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { ErrorState, NoPayPermission } from '../kit/states'
import { checkoutReturnUrl } from './checkout-return'
import { DiscountField } from './discount-field'
import { PaymentStep } from './payment-step'

type Props = { workspaceId: string; canPay: boolean; priceId: string; offerId?: string }

function find(catalog: Catalog, priceId: string, offerId?: string) {
  for (const plan of catalog.plans) {
    const price = plan.prices.find((p) => p.id === priceId)
    if (price) {
      const offer = offerId ? price.installment_offers.find((o) => o.id === offerId) : undefined
      return { plan, price, offer }
    }
  }
  return undefined
}

/** Providers that sell this price (and this installment offer), as this server has them. */
function providersFor(catalog: Catalog, price: Price, offer?: InstallmentOffer): Provider[] {
  return catalog.providers.filter(
    (p) =>
      price.providers.includes(p.name) &&
      (!offer || offer.providers.some((o) => o.provider === p.name)),
  )
}

export function CheckoutPage({ workspaceId, canPay, priceId, offerId }: Props) {
  const plans = useQuery(plansQueryOptions())

  return (
    <main className="mx-auto flex w-full max-w-[960px] flex-col gap-5 p-4 sm:p-7">
      <nav aria-label="Breadcrumb">
        <Link
          to="/billing/plans"
          className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-brand-text no-underline hover:text-foreground"
        >
          <ChevronLeftIcon className="size-4" aria-hidden="true" />
          Plans
        </Link>
      </nav>
      <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Checkout</h1>
      {plans.isPending && <CheckoutSkeleton />}
      {plans.isError && (
        <ErrorState
          title="Couldn’t load this plan"
          error={plans.error}
          onRetry={() => void plans.refetch()}
        />
      )}
      {plans.data &&
        (() => {
          const found = find(plans.data, priceId, offerId)
          if (!found) {
            return (
              <Section>
                <p className="text-sm">
                  This plan isn’t on sale any more. Pick another one from the plans page.
                </p>
                <Button asChild className="self-start">
                  <Link to="/billing/plans">See plans</Link>
                </Button>
              </Section>
            )
          }
          return (
            <Checkout
              workspaceId={workspaceId}
              canPay={canPay}
              {...found}
              providers={providersFor(plans.data, found.price, found.offer)}
            />
          )
        })()}
    </main>
  )
}

function CheckoutSkeleton() {
  return (
    <div
      className="grid gap-5 md:grid-cols-[1fr_360px]"
      role="status"
      aria-busy="true"
      aria-label="Loading checkout"
    >
      <Skeleton className="h-[320px] rounded-xl" />
      <Skeleton className="h-[260px] rounded-xl" />
    </div>
  )
}

function Checkout({
  workspaceId,
  canPay,
  plan,
  price,
  offer,
  providers,
}: {
  workspaceId: string
  canPay: boolean
  plan: Plan
  price: Price
  offer?: InstallmentOffer
  providers: Provider[]
}) {
  const navigate = useNavigate()
  const overview = useQuery(overviewQueryOptions(workspaceId))
  const [discount, setDiscount] = useState<DiscountPreview>()
  const [provider, setProvider] = useState<ProviderName | undefined>(providers[0]?.name)
  const [started, setStarted] = useState<StartedCheckout>()
  const create = useCreateCheckout(workspaceId)
  const idem = useIdempotencyKey(
    [price.id, offer?.id ?? '', discount?.code ?? '', provider ?? ''].join(':'),
  )
  const lifetime = price.billing_type === 'one_time'
  const quote = useQuery({
    ...quoteQueryOptions(workspaceId, price.id, offer?.id, discount?.code),
    enabled: lifetime,
  })

  const subscription = overview.data?.subscription
  const liveSubscription =
    subscription && ['trialing', 'active', 'past_due'].includes(subscription.status)

  const start = () =>
    create.mutate(
      {
        idempotencyKey: idem.key,
        priceId: price.id,
        offerId: offer?.id,
        discountCode: discount?.code,
        provider,
        returnUrl: checkoutReturnUrl(),
      },
      { onSuccess: setStarted },
    )

  const dueNow = started
    ? started.session.amount_minor
    : lifetime && quote.data
      ? quote.data.due_now_minor
      : discount
        ? (discount.installments_minor[0] ?? discount.total_minor)
        : (offer?.amounts_minor[0] ?? price.amount_minor)

  return (
    <div className="grid items-start gap-5 md:grid-cols-[1fr_360px]">
      <div className="flex flex-col gap-5">
        {liveSubscription && price.billing_type === 'recurring' && (
          <Alert variant="info">
            <AlertDescription>
              <p>
                This workspace already has a subscription.{' '}
                <Link to="/billing/plans" className="font-bold">
                  Switch plans instead
                </Link>{' '}
                to keep one subscription and get prorated pricing.
              </p>
            </AlertDescription>
          </Alert>
        )}

        <Section aria-labelledby="pay-heading">
          <SectionHeader
            id="pay-heading"
            title="Payment"
            description={
              started
                ? undefined
                : 'Your card details go straight to the payment provider; Relay never sees them.'
            }
          />
          {!canPay ? (
            <NoPayPermission>
              Only the owner or members with the “Pay and change plan” permission can buy a plan.
              Ask one of them to complete this checkout.
            </NoPayPermission>
          ) : started ? (
            <PaymentStep
              workspaceId={workspaceId}
              started={started}
              payLabel={`Pay ${formatMoney(started.session.amount_minor, started.session.currency)}`}
              onPaid={(session) => navigate({ to: '/billing/checkout', search: { session } })}
            />
          ) : providers.length === 0 ? (
            <Alert variant="danger">
              <AlertDescription>
                No payment method can sell this plan right now. Please try again later or contact
                support.
              </AlertDescription>
            </Alert>
          ) : (
            <div className="flex flex-col gap-5">
              <DiscountField
                workspaceId={workspaceId}
                priceId={price.id}
                offerId={offer?.id}
                oneTime={lifetime}
                applied={discount}
                onApplied={setDiscount}
              />
              {providers.length > 1 && (
                <ProviderChoice providers={providers} value={provider} onChange={setProvider} />
              )}
              {create.isError && (
                <Alert variant="danger">
                  <AlertDescription>{errorMessage(create.error)}</AlertDescription>
                </Alert>
              )}
              <Button size="lg" onClick={start} disabled={create.isPending}>
                {create.isPending ? 'Starting checkout…' : 'Continue to payment'}
              </Button>
            </div>
          )}
        </Section>
      </div>

      <Summary
        plan={plan}
        price={price}
        offer={offer}
        discount={discount}
        quote={lifetime ? quote : undefined}
        dueNow={dueNow}
      />
    </div>
  )
}

function ProviderChoice({
  providers,
  value,
  onChange,
}: {
  providers: Provider[]
  value?: ProviderName
  onChange: (p: ProviderName) => void
}) {
  const id = useId()
  return (
    <fieldset className="flex flex-col gap-2">
      <legend className="mb-2 text-sm font-medium">Pay with</legend>
      <RadioGroup
        value={value}
        onValueChange={(v) => onChange(v as ProviderName)}
        className="gap-2"
      >
        {providers.map((p) => (
          <Label
            key={p.name}
            htmlFor={`${id}-${p.name}`}
            className={cn(
              'flex cursor-pointer items-start gap-3 rounded-lg border border-input p-3 font-normal leading-snug',
              value === p.name && 'border-primary bg-muted',
            )}
          >
            <RadioGroupItem id={`${id}-${p.name}`} value={p.name} className="mt-0.5" />
            <span className="flex flex-col gap-0.5">
              <span className="font-bold">{PROVIDER_LABEL[p.name]}</span>
              <span className="text-[13px] text-muted-foreground">
                {p.embedded_checkout ? 'Pay on this page' : 'Opens a secure checkout page'}
                {p.merchant_of_record && ' · sales tax included'}
              </span>
            </span>
          </Label>
        ))}
      </RadioGroup>
    </fieldset>
  )
}

function Line({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div className={cn('flex justify-between gap-3 text-sm', strong && 'font-extrabold')}>
      <dt className={strong ? undefined : 'text-muted-foreground'}>{label}</dt>
      <dd className="text-right tabular-nums">{value}</dd>
    </div>
  )
}

function Summary({
  plan,
  price,
  offer,
  discount,
  quote,
  dueNow,
}: {
  plan: Plan
  price: Price
  offer?: InstallmentOffer
  discount?: DiscountPreview
  quote?: UseQueryResult<OrderQuote>
  dueNow: number
}) {
  const money = (n: number) => formatMoney(n, price.currency)
  const q = quote?.data
  const subtotal =
    q?.total_minor ?? discount?.subtotal_minor ?? offer?.total_minor ?? price.amount_minor
  const discountMinor = q?.discount_minor ?? discount?.discount_minor ?? 0
  const credit = q?.credit_minor ?? 0
  const total = subtotal - discountMinor - credit
  // The schedule: the quote's when we have it, else the discounted or listed installments.
  const schedule =
    q?.schedule.map((s) => ({ seq: s.seq, amount: s.amount_minor, due: formatDate(s.due_at) })) ??
    (discount?.installments_minor.length
      ? discount.installments_minor
      : (offer?.amounts_minor ?? [])
    ).map((amount, i) => ({ seq: i + 1, amount, due: i === 0 ? 'Today' : `Month ${i + 1}` }))

  return (
    <Section aria-labelledby="summary-heading" className="md:sticky md:top-4">
      <SectionHeader
        id="summary-heading"
        title="Order summary"
        description={`${plan.name} · ${
          price.billing_type === 'one_time'
            ? offer
              ? `lifetime, ${offer.count} monthly payments`
              : 'lifetime'
            : price.interval === 'year'
              ? 'billed yearly'
              : 'billed monthly'
        }`}
      />
      <ul className="flex flex-col gap-1 text-[13px] text-muted-foreground">
        {limitLines(plan.limits).map((l) => (
          <li key={l}>{l}</li>
        ))}
      </ul>
      <Separator />
      <dl className="flex flex-col gap-2">
        <Line
          label={price.billing_type === 'recurring' ? `Price ${perInterval(price)}` : 'Price'}
          value={money(subtotal)}
        />
        {discountMinor > 0 && (
          <Line
            label={`Discount${discount ? ` (${discount.code})` : ''}`}
            value={`−${money(discountMinor)}`}
          />
        )}
        {credit > 0 && <Line label="Credit from your current plan" value={`−${money(credit)}`} />}
        {(discountMinor > 0 || credit > 0) && <Line label="Total" value={money(total)} />}
        <Separator />
        <Line label="Due today" value={money(dueNow)} strong />
      </dl>
      {quote?.isPending && (
        <Skeleton className="h-4 w-2/3" role="status" aria-label="Working out your credit" />
      )}
      {quote?.isError && (
        <p className="text-[13px] text-muted-foreground">
          We couldn’t work out credit from your current plan yet ({errorMessage(quote.error)}). The
          payment step shows the exact amount.
        </p>
      )}
      {q?.replaces_subscription_id && (
        <p className="text-[13px] text-muted-foreground">
          Your subscription ends when this payment goes through; its unused time is the credit
          above.
        </p>
      )}
      {schedule.length > 1 && (
        <div className="flex flex-col gap-2">
          <h3 className="text-[13px] font-bold">Payment schedule</h3>
          <ol className="flex flex-col gap-1">
            {schedule.map((s) => (
              <li key={s.seq} className="flex justify-between gap-3 text-[13px]">
                <span className="text-muted-foreground">
                  {s.seq}. {s.due}
                </span>
                <span className="tabular-nums">{money(s.amount)}</span>
              </li>
            ))}
          </ol>
          {offer && (
            <p className="text-[13px] text-muted-foreground">
              You get the full plan from the first payment. If a payment is missed, free limits
              apply after {offer.grace_days} days until it is paid.
            </p>
          )}
        </div>
      )}
    </Section>
  )
}
