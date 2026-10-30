import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { type ReactNode, useState } from 'react'
import { Button } from '#/components/ui/button'
import { overviewQueryOptions } from '../../api/billing.queries'
import type { Price } from '../../types'
import { ErrorState } from '../kit/states'
import { Catalogue } from './catalogue'
import { ChangePlanDialog } from './change-plan-dialog'

/** A price per month, so monthly and yearly prices compare. */
function monthly(p: Price) {
  return p.interval === 'year' ? p.amount_minor / 12 : p.amount_minor
}

/**
 * The catalog for a workspace that may already pay: its plan is marked current, a live
 * subscription switches in place, and anything else goes through checkout.
 */
export function PlanChoices({
  workspaceId,
  canPay,
  header,
}: {
  workspaceId: string
  canPay: boolean
  header?: (toggle: ReactNode) => ReactNode
}) {
  const overview = useQuery(overviewQueryOptions(workspaceId))
  const [changeTo, setChangeTo] = useState<string>()
  const subscription = overview.data?.subscription
  const live = subscription && ['trialing', 'active', 'past_due'].includes(subscription.status)
  const currentPriceId = live ? subscription.plan_price_id : overview.data?.order?.plan_price_id
  const currentPrice = live ? subscription.price : null

  return (
    <>
      {overview.isError && (
        <ErrorState
          title="Couldn’t load your current plan"
          error={overview.error}
          onRetry={() => void overview.refetch()}
        />
      )}
      <Catalogue
        currentPriceId={currentPriceId}
        header={header}
        action={({ plan, price, selection, current }) => {
          if (current) {
            return (
              <Button variant="outline" disabled className="disabled:opacity-100">
                Your current plan
              </Button>
            )
          }
          if (!canPay) return null
          if (live && price.billing_type === 'recurring') {
            const samePlan = currentPrice?.plan_id === price.plan_id
            const up = !currentPrice || monthly(price) > monthly(currentPrice)
            const label = samePlan
              ? `Switch to ${price.interval === 'year' ? 'yearly' : 'monthly'}`
              : up
                ? `Upgrade to ${plan.name}`
                : 'Downgrade'
            return (
              <Button
                variant={up && !samePlan ? 'default' : 'outline'}
                onClick={() => setChangeTo(price.id)}
              >
                {label}
              </Button>
            )
          }
          return (
            <Button asChild variant={plan.recommended ? 'default' : 'outline'}>
              <Link
                to="/billing/checkout"
                search={{ price: selection.priceId, offer: selection.offerId }}
              >
                {price.billing_type === 'one_time' ? 'Buy lifetime deal' : `Choose ${plan.name}`}
              </Link>
            </Button>
          )
        }}
      />
      {changeTo && (
        <ChangePlanDialog
          workspaceId={workspaceId}
          priceId={changeTo}
          onClose={() => setChangeTo(undefined)}
        />
      )}
    </>
  )
}
