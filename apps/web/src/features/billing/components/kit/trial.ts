import type { Overview, Plan, Price, Provider } from '../../types'

/**
 * Whether the workspace has used its one trial, as far as the overview shows: a subscription that
 * started, or a trial grant. The API decides at checkout; this only keeps the cards from
 * promising a trial it will refuse.
 */
export function hadTrial({ plan, subscription }: Overview) {
  return plan.grant?.source === 'trial' || (!!subscription && subscription.status !== 'incomplete')
}

/**
 * Whether checking out this price starts a free trial. Only a card-required trial can: the
 * provider must run it at checkout (Creem can't) and the workspace must not have had one. A
 * card-less trial is granted once, to a new workspace, and never by buying a plan.
 */
export function checkoutStartsTrial(plan: Plan, price: Price, providers: Provider[]) {
  return (
    plan.trial_days > 0 &&
    plan.trial_requires_card &&
    providers.some((p) => p.checkout_trials && price.providers.includes(p.name))
  )
}
