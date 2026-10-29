import { createFileRoute } from '@tanstack/react-router'
import { PublicPricingPage } from '#/features/billing'

// Public: anyone can compare plans. Choosing one goes to checkout, which asks to sign in first.
export const Route = createFileRoute('/pricing')({
  head: () => ({ meta: [{ title: 'Pricing · Relay' }] }),
  component: PublicPricingPage,
})
