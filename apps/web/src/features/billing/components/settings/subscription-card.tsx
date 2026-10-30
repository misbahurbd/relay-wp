import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Label } from '#/components/ui/label'
import { RadioGroup, RadioGroupItem } from '#/components/ui/radio-group'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import {
  useCancelPendingChange,
  useCancelSubscription,
  useResumeSubscription,
} from '../../api/billing.mutations'
import { plansQueryOptions } from '../../api/billing.queries'
import { SUBSCRIPTION_STATUS } from '../../schemas/billing.schema'
import type { Subscription } from '../../types'
import { formatDate } from '../kit/format'
import { Modal } from '../kit/modal'
import { formatMoney, perInterval } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { NoPayPermission } from '../kit/states'
import { StatusBadge } from '../kit/status-badge'

export function SubscriptionCard({
  workspaceId,
  subscription: sub,
  canPay,
}: {
  workspaceId: string
  subscription: Subscription
  canPay: boolean
}) {
  const plans = useQuery(plansQueryOptions())
  const [canceling, setCanceling] = useState(false)
  const resume = useResumeSubscription(workspaceId)
  const dropPending = useCancelPendingChange(workspaceId)
  const live = ['trialing', 'active', 'past_due'].includes(sub.status)
  const planName = (priceId: string) =>
    plans.data?.plans.find((p) => p.prices.some((pr) => pr.id === priceId))?.name
  const error = resume.error ?? dropPending.error

  return (
    <Section aria-labelledby="sub-heading">
      <SectionHeader
        id="sub-heading"
        title="Subscription"
        description={
          sub.price
            ? `${formatMoney(sub.price.amount_minor, sub.price.currency)} ${perInterval(sub.price)}`
            : undefined
        }
        action={<StatusBadge status={SUBSCRIPTION_STATUS[sub.status]} />}
      />
      <p className="text-sm">
        {sub.status === 'canceled'
          ? `Canceled${sub.canceled_at ? ` on ${formatDate(sub.canceled_at)}` : ''}.`
          : sub.cancel_at_period_end && sub.current_period_end
            ? `Ends on ${formatDate(sub.current_period_end)}. You keep the plan until then.`
            : sub.status === 'trialing' && sub.current_period_end
              ? `Trial ends ${formatDate(sub.current_period_end)}; the first charge is that day.`
              : sub.current_period_end
                ? `Renews on ${formatDate(sub.current_period_end)}.`
                : 'Waiting for the first payment.'}
      </p>
      {sub.discount_ends_at && (
        <p className="text-[13px] text-muted-foreground">
          Your discount applies until {formatDate(sub.discount_ends_at)}.
        </p>
      )}

      {sub.status === 'past_due' && (
        <Alert variant="danger">
          <AlertDescription>
            The last renewal failed. Update your card with the payment provider
            {sub.grace_ends_at && ` before ${formatDate(sub.grace_ends_at)}`} to keep your plan.
          </AlertDescription>
        </Alert>
      )}

      {sub.pending_plan_price_id && sub.pending_change_at && (
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

      {error && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(error)}</AlertDescription>
        </Alert>
      )}

      {live &&
        (canPay ? (
          <div className="flex flex-wrap gap-2">
            {sub.cancel_at_period_end ? (
              <Button
                disabled={resume.isPending}
                onClick={() =>
                  resume.mutate(undefined, {
                    onSuccess: () => toast.success('Subscription resumed'),
                  })
                }
              >
                {resume.isPending ? 'Resuming…' : 'Resume subscription'}
              </Button>
            ) : (
              <Button variant="outline" onClick={() => setCanceling(true)}>
                Cancel subscription
              </Button>
            )}
          </div>
        ) : (
          <NoPayPermission />
        ))}

      {canceling && (
        <CancelDialog
          workspaceId={workspaceId}
          periodEnd={sub.current_period_end}
          onClose={() => setCanceling(false)}
        />
      )}
    </Section>
  )
}

function CancelDialog({
  workspaceId,
  periodEnd,
  onClose,
}: {
  workspaceId: string
  periodEnd: string | null
  onClose: () => void
}) {
  const cancel = useCancelSubscription(workspaceId)
  const [when, setWhen] = useState<'end' | 'now'>('end')
  const options = [
    {
      value: 'end' as const,
      title: 'At the end of the period',
      body: periodEnd
        ? `Keep the plan until ${formatDate(periodEnd)}, then move to the free plan.`
        : 'Keep the plan until the period ends, then move to the free plan.',
    },
    {
      value: 'now' as const,
      title: 'Right away',
      body: 'Move to the free plan now. Unused time isn’t refunded.',
    },
  ]

  return (
    <Modal
      onClose={onClose}
      title="Cancel subscription?"
      description="Your workspace moves to the free plan’s limits when the subscription ends. You can subscribe again at any time."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Keep subscription
          </Button>
          <Button
            variant="destructive"
            disabled={cancel.isPending}
            onClick={() =>
              cancel.mutate(when === 'end', {
                onSuccess: () => {
                  toast.success(when === 'end' ? 'Subscription will end' : 'Subscription canceled')
                  onClose()
                },
              })
            }
          >
            {cancel.isPending ? 'Canceling…' : 'Cancel subscription'}
          </Button>
        </>
      }
    >
      <fieldset>
        <legend className="mb-2 text-sm font-medium">When should it end?</legend>
        <RadioGroup
          value={when}
          onValueChange={(v) => setWhen(v as 'end' | 'now')}
          className="gap-2"
        >
          {options.map((o) => (
            <Label
              key={o.value}
              htmlFor={`cancel-${o.value}`}
              className={cn(
                'flex cursor-pointer items-start gap-3 rounded-lg border border-input p-3 font-normal leading-snug',
                when === o.value && 'border-primary bg-muted',
              )}
            >
              <RadioGroupItem id={`cancel-${o.value}`} value={o.value} className="mt-0.5" />
              <span className="flex flex-col gap-0.5">
                <span className="font-bold">{o.title}</span>
                <span className="text-[13px] text-muted-foreground">{o.body}</span>
              </span>
            </Label>
          ))}
        </RadioGroup>
      </fieldset>
      {cancel.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(cancel.error)}</AlertDescription>
        </Alert>
      )}
    </Modal>
  )
}
