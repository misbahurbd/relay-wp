import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { CheckoutPage, CheckoutStatus } from '#/features/billing'
import { useAccess } from '#/features/team'

// ?price= (and ?offer= for installments) starts a checkout; ?session= shows how one ended,
// after Stripe's 3-D Secure or a hosted checkout returns here.
const search = z.object({
  price: z.uuid().optional().catch(undefined),
  offer: z.uuid().optional().catch(undefined),
  session: z
    .union([z.uuid(), z.literal('return')])
    .optional()
    .catch(undefined),
})

export const Route = createFileRoute('/_app/_workspace/_shell/billing/checkout')({
  validateSearch: search,
  beforeLoad: ({ search }) => {
    if (!search.price && !search.session) throw redirect({ to: '/billing/plans' })
  },
  head: () => ({ meta: [{ title: 'Checkout · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { price, offer, session } = Route.useSearch()
  const { can } = useAccess(workspaceId)
  if (session) return <CheckoutStatus key={session} workspaceId={workspaceId} sessionId={session} />
  if (!price) return null
  return (
    <CheckoutPage
      key={`${price}:${offer ?? ''}`}
      workspaceId={workspaceId}
      canPay={can('billing.pay')}
      priceId={price}
      offerId={offer}
    />
  )
}
