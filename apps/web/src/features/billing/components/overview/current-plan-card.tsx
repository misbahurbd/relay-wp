import { useQuery } from '@tanstack/react-query'
import { TriangleAlertIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'
import {
  useCancelPendingChange,
  useOpenCardPortal,
  useResumeSubscription,
} from '../../api/billing.mutations'
import { paymentMethodQueryOptions, plansQueryOptions } from '../../api/billing.queries'
import { GRANT_SOURCE, SUBSCRIPTION_STATUS } from '../../schemas/billing.schema'
import type { Overview, PaymentMethod, Subscription } from '../../types'
import { formatDate, formatMonth } from '../kit/format'
import { formatMoney, formatPrice } from '../kit/money'
import { Section } from '../kit/section'
import { NoPayPermission } from '../kit/states'
import { StatusBadge } from '../kit/status-badge'
import { CancelDialog } from './cancel-dialog'

const LIVE: Subscription['status'][] = ['trialing', 'active', 'past_due']

/** The plan in force now, what it costs, when it next charges, and the actions on it. */
export function CurrentPlanCard({
  workspaceId,
  overview,
  canPay,
  className,
}: {
  workspaceId: string
  overview: Overview
  canPay: boolean
  className?: string
}) {
  const { plan, subscription: sub, order } = overview
  const grant = plan.grant
  const suspended = grant?.status === 'suspended'
  const live = !!sub && LIVE.includes(sub.status)
  const plans = useQuery(plansQueryOptions())
  const card = useQuery({ ...paymentMethodQueryOptions(workspaceId), enabled: live })
  const resume = useResumeSubscription(workspaceId)
  const dropPending = useCancelPendingChange(workspaceId)
  const portal = useOpenCardPortal(workspaceId)
  const [canceling, setCanceling] = useState(false)
  const price = live ? sub.price : order?.price
  const planName = (priceId: string) =>
    plans.data?.plans.find((p) => p.prices.some((pr) => pr.id === priceId))?.name
  const error = resume.error ?? dropPending.error ?? portal.error

  return (
    <Section aria-labelledby="plan-heading" className={className}>
      <div className="flex items-start justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h2
            id="plan-heading"
            className="text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase"
          >
            Current plan
          </h2>
          <p className="text-2xl font-extrabold tracking-[-0.02em]">{plan.plan_name}</p>
        </div>
        {sub && live ? (
          <StatusBadge status={SUBSCRIPTION_STATUS[sub.status]} />
        ) : suspended ? (
          <StatusBadge status={{ label: 'Suspended', tone: 'danger' }} />
        ) : grant ? (
          <Badge variant="neutral">{GRANT_SOURCE[grant.source]}</Badge>
        ) : null}
      </div>

      {price ? (
        <p className="flex flex-wrap items-baseline gap-x-1.5">
          <span className="text-4xl font-extrabold tracking-[-0.02em]">
            {formatPrice(price.amount_minor, price.currency)}
          </span>
          <span className="text-muted-foreground">
            {price.interval === 'month'
              ? '/ month, billed monthly'
              : price.interval === 'year'
                ? '/ year, billed yearly'
                : 'once, for life'}
          </span>
        </p>
      ) : (
        !grant && (
          <p className="text-[13px] text-muted-foreground">
            The free plan. Upgrade for more team members and WhatsApp numbers.
          </p>
        )
      )}

      <Facts overview={overview} card={card.data} />

      {suspended && (
        <Alert variant="warning">
          <TriangleAlertIcon aria-hidden="true" />
          <AlertTitle>Free limits apply for now</AlertTitle>
          <AlertDescription>
            <p>
              An installment was missed
              {grant.suspended_at && `, so access was limited on ${formatDate(grant.suspended_at)}`}
              . Pay it below to get your plan back straight away.
            </p>
          </AlertDescription>
        </Alert>
      )}

      {sub?.status === 'past_due' && (
        <Alert variant="danger">
          <AlertDescription>
            <p>
              The last renewal failed. Update your card
              {sub.grace_ends_at && ` before ${formatDate(sub.grace_ends_at)}`} to keep your plan.
            </p>
            {canPay && card.data?.can_manage && (
              <Button
                variant="outline"
                size="sm"
                className="mt-2"
                disabled={portal.isPending}
                onClick={() => portal.mutate()}
              >
                {portal.isPending ? 'Opening…' : 'Update card'}
              </Button>
            )}
          </AlertDescription>
        </Alert>
      )}

      {sub?.pending_plan_price_id && sub.pending_change_at && (
        <Alert variant="info">
          <AlertDescription>
            <p>
              Switching to {planName(sub.pending_plan_price_id) ?? 'another plan'} on{' '}
              {formatDate(sub.pending_change_at)}.
            </p>
            {canPay && (
              <Button
                variant="outline"
                size="sm"
                className="mt-2"
                disabled={dropPending.isPending}
                onClick={() =>
                  dropPending.mutate(undefined, {
                    onSuccess: () => toast.success('Scheduled change canceled'),
                  })
                }
              >
                {dropPending.isPending ? 'Canceling…' : 'Keep current plan'}
              </Button>
            )}
          </AlertDescription>
        </Alert>
      )}

      {sub?.discount_ends_at && (
        <p className="text-[13px] text-muted-foreground">
          Your discount applies until {formatDate(sub.discount_ends_at)}.
        </p>
      )}

      {error && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(error)}</AlertDescription>
        </Alert>
      )}

      {canPay ? (
        <div className="mt-auto flex flex-wrap gap-2.5">
          <Button asChild>
            <a href="#plans">{grant ? 'Change plan' : 'Upgrade'}</a>
          </Button>
          {live &&
            (sub.cancel_at_period_end ? (
              <Button
                variant="outline"
                disabled={resume.isPending}
                onClick={() =>
                  resume.mutate(undefined, {
                    onSuccess: () => toast.success('Subscription resumed'),
                  })
                }
              >
                {resume.isPending ? 'Resuming…' : 'Resume plan'}
              </Button>
            ) : (
              <Button
                variant="outline"
                className="text-destructive hover:text-destructive"
                onClick={() => setCanceling(true)}
              >
                Cancel plan
              </Button>
            ))}
        </div>
      ) : (
        <NoPayPermission />
      )}

      {canceling && sub && (
        <CancelDialog
          workspaceId={workspaceId}
          periodEnd={sub.current_period_end}
          onClose={() => setCanceling(false)}
        />
      )}
    </Section>
  )
}

/** The dates and charge that apply to this plan, as label/value pairs. */
function Facts({ overview, card }: { overview: Overview; card: PaymentMethod | undefined }) {
  const { plan, subscription: sub } = overview
  const grant = plan.grant
  const live = !!sub && LIVE.includes(sub.status)
  const facts: { label: string; value: string }[] = []

  if (sub && live && sub.current_period_end) {
    facts.push({
      label: sub.cancel_at_period_end
        ? 'Ends on'
        : sub.status === 'trialing'
          ? 'Trial ends'
          : 'Renews on',
      value: formatDate(sub.current_period_end),
    })
    if (!sub.cancel_at_period_end && sub.price) {
      facts.push({
        label: 'Next charge',
        value: `${formatMoney(sub.price.amount_minor, sub.price.currency)} to ${chargeTo(card)}`,
      })
    }
  } else if (sub?.status === 'canceled' && sub.canceled_at) {
    facts.push({ label: 'Canceled on', value: formatDate(sub.canceled_at) })
  } else if (grant?.source === 'trial' && grant.ends_at) {
    facts.push({ label: 'Trial ends', value: formatDate(grant.ends_at) })
  } else if (grant && grant.ends_at === null) {
    facts.push({ label: 'Lifetime access', value: 'No renewals' })
  } else if (grant?.ends_at) {
    facts.push({ label: 'Ends on', value: formatDate(grant.ends_at) })
  }
  if (grant) facts.push({ label: 'Member since', value: formatMonth(grant.starts_at) })

  if (facts.length === 0) return null
  return (
    <dl className="flex flex-col gap-2 rounded-lg bg-muted p-3.5 text-[13px]">
      {facts.map((f) => (
        <div key={f.label} className="flex justify-between gap-3">
          <dt className="text-muted-foreground">{f.label}</dt>
          <dd className="text-right font-bold">{f.value}</dd>
        </div>
      ))}
    </dl>
  )
}

function chargeTo(card: PaymentMethod | undefined) {
  if (card?.card) return `card •••• ${card.card.last4}`
  if (card && !card.card_details) return 'your card on file'
  return 'your saved card'
}
