import type { QueryClient } from '@tanstack/react-query'

/** Every billing query for one workspace sits under ['billing', workspaceId], so one invalidate refreshes all. */
export const billingKeys = {
  /** The public catalog: the same for everyone, so it isn't under a workspace. */
  plans: () => ['billing-plans'] as const,
  /** What a plan can include: the same for everyone. */
  resources: () => ['billing-resources'] as const,
  workspace: (workspaceId: string) => ['billing', workspaceId] as const,
  overview: (workspaceId: string) => ['billing', workspaceId, 'overview'] as const,
  invoices: (workspaceId: string) => ['billing', workspaceId, 'invoices'] as const,
  invoice: (workspaceId: string, invoiceId: string) =>
    ['billing', workspaceId, 'invoice', invoiceId] as const,
  profile: (workspaceId: string) => ['billing', workspaceId, 'profile'] as const,
  order: (workspaceId: string, orderId: string) =>
    ['billing', workspaceId, 'order', orderId] as const,
  quote: (workspaceId: string, priceId: string, offerId: string, code: string) =>
    ['billing', workspaceId, 'quote', priceId, offerId, code] as const,
  changePreview: (workspaceId: string, priceId: string) =>
    ['billing', workspaceId, 'change-preview', priceId] as const,
  checkout: (workspaceId: string, sessionId: string) =>
    ['billing', workspaceId, 'checkout', sessionId] as const,
  paymentMethod: (workspaceId: string) => ['billing', workspaceId, 'payment-method'] as const,
  /** Seats, WhatsApp numbers and contacts, read from their own endpoints for the usage meters. */
  seats: (workspaceId: string) => ['billing', workspaceId, 'usage', 'seats'] as const,
  numbers: (workspaceId: string) => ['billing', workspaceId, 'usage', 'numbers'] as const,
  contacts: (workspaceId: string) => ['billing', workspaceId, 'usage', 'contacts'] as const,
  /** This month's use of monthly resources (broadcasts, API calls, AI replies). */
  monthly: (workspaceId: string) => ['billing', workspaceId, 'usage', 'monthly'] as const,
  /** Sample data (api/billing.preview.ts). */
  preview: (workspaceId: string, part: string) =>
    ['billing', workspaceId, 'preview', part] as const,
}

export function invalidateBilling(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: billingKeys.workspace(workspaceId) })
}
