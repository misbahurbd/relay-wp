import { env } from '#/lib/env'

/**
 * Whether to show the billing sections that run on sample data (Meta costs, wallet, Spending).
 * VITE_BILLING_PREVIEW=false hides them everywhere.
 */
export function useBillingPreview() {
  return env.VITE_BILLING_PREVIEW
}
