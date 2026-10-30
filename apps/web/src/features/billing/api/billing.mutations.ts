import {
  cancelPendingSubscriptionChange,
  cancelSubscription,
  changeSubscription,
  completeTestCheckout,
  createCheckoutSession,
  createPortalSession,
  type ProviderDto,
  payInstallment,
  previewDiscount,
  resumeOrder,
  resumeSubscription,
  updateBillingProfile,
} from '@relay/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { BillingProfileInput, DiscountPreview, StartedCheckout } from '../types'
import { billingKeys, invalidateBilling } from './billing.keys'

/** Every billing change can move the plan, grant or invoices, so it refreshes all billing data. */
function useBillingMutation<TInput, TOutput>(
  workspaceId: string,
  fn: (input: TInput) => Promise<TOutput>,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => invalidateBilling(queryClient, workspaceId),
  })
}

// Create endpoints take an Idempotency-Key: a retry with the same key returns the same checkout.

export type CheckoutInput = {
  idempotencyKey: string
  priceId: string
  offerId?: string
  discountCode?: string
  provider?: ProviderDto['name']
  returnUrl: string
}

export function useCreateCheckout(workspaceId: string) {
  return useMutation({
    mutationFn: async (input: CheckoutInput): Promise<StartedCheckout> => {
      const { data } = await createCheckoutSession({
        headers: { ...inWorkspace(workspaceId), 'Idempotency-Key': input.idempotencyKey },
        body: {
          plan_price_id: input.priceId,
          installment_offer_id: input.offerId,
          discount_code: input.discountCode,
          provider: input.provider,
          return_url: input.returnUrl,
        },
        throwOnError: true,
      })
      return data
    },
  })
}

export function usePayInstallment(workspaceId: string) {
  return useMutation({
    mutationFn: async (input: {
      idempotencyKey: string
      orderId: string
      installmentId: string
    }): Promise<StartedCheckout> => {
      const { data } = await payInstallment({
        headers: { ...inWorkspace(workspaceId), 'Idempotency-Key': input.idempotencyKey },
        path: { orderId: input.orderId, installmentId: input.installmentId },
        throwOnError: true,
      })
      return data
    },
  })
}

/** Pays a defaulted lifetime deal's remaining balance, which restores its grant. */
export function useResumeOrder(workspaceId: string) {
  return useMutation({
    mutationFn: async (input: {
      idempotencyKey: string
      orderId: string
    }): Promise<StartedCheckout> => {
      const { data } = await resumeOrder({
        headers: { ...inWorkspace(workspaceId), 'Idempotency-Key': input.idempotencyKey },
        path: { orderId: input.orderId },
        throwOnError: true,
      })
      return data
    },
  })
}

export function usePreviewDiscount(workspaceId: string) {
  return useMutation({
    mutationFn: async (input: {
      code: string
      priceId: string
      offerId?: string
    }): Promise<DiscountPreview> => {
      const { data } = await previewDiscount({
        headers: inWorkspace(workspaceId),
        body: {
          code: input.code,
          plan_price_id: input.priceId,
          installment_offer_id: input.offerId,
        },
        throwOnError: true,
      })
      return { ...data, installments_minor: data.installments_minor ?? [] }
    },
  })
}

// ---------- subscription ----------

export function useChangeSubscription(workspaceId: string) {
  return useBillingMutation(workspaceId, async (priceId: string) => {
    const { data } = await changeSubscription({
      headers: inWorkspace(workspaceId),
      body: { plan_price_id: priceId },
      throwOnError: true,
    })
    return data
  })
}

/** Pays a checkout on the development test gateway, as its provider would. */
export function useCompleteTestCheckout(workspaceId: string) {
  return useBillingMutation(workspaceId, async (sessionId: string) => {
    const { data } = await completeTestCheckout({
      headers: inWorkspace(workspaceId),
      path: { sessionId },
      throwOnError: true,
    })
    return data
  })
}

/** Drops a downgrade that is waiting for the period end. */
export function useCancelPendingChange(workspaceId: string) {
  return useBillingMutation(workspaceId, async () => {
    const { data } = await cancelPendingSubscriptionChange({
      headers: inWorkspace(workspaceId),
      throwOnError: true,
    })
    return data
  })
}

export function useCancelSubscription(workspaceId: string) {
  return useBillingMutation(workspaceId, async (atPeriodEnd: boolean) => {
    const { data } = await cancelSubscription({
      headers: inWorkspace(workspaceId),
      body: { at_period_end: atPeriodEnd },
      throwOnError: true,
    })
    return data
  })
}

/** Undoes a cancel-at-period-end before the period ends. */
export function useResumeSubscription(workspaceId: string) {
  return useBillingMutation(workspaceId, async () => {
    const { data } = await resumeSubscription({
      headers: inWorkspace(workspaceId),
      throwOnError: true,
    })
    return data
  })
}

// ---------- profile ----------

export function useUpdateProfile(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: BillingProfileInput) => {
      const { data } = await updateBillingProfile({
        headers: inWorkspace(workspaceId),
        body,
        throwOnError: true,
      })
      return data
    },
    onSuccess: (data) => queryClient.setQueryData(billingKeys.profile(workspaceId), data),
  })
}

/** Opens the payment provider's page for changing cards, in this tab. */
export function useOpenCardPortal(workspaceId: string) {
  return useMutation({
    mutationFn: async () => {
      const { data } = await createPortalSession({
        headers: inWorkspace(workspaceId),
        throwOnError: true,
      })
      return data.url
    },
    onSuccess: (url) => window.location.assign(url),
  })
}
