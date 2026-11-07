import {
  adminCloseDispute,
  adminCreateDiscount,
  adminCreateInstallmentOffer,
  adminCreatePlan,
  adminCreatePrice,
  adminCreateProviderPrice,
  adminDeactivateDiscount,
  adminDeactivateInstallmentOffer,
  adminDeactivatePlan,
  adminDeactivatePrice,
  adminDeactivateProviderPrice,
  adminEndGrant,
  adminGetCatalog,
  adminGrantPlan,
  adminListDiscounts,
  adminListDisputes,
  adminListWorkspaces,
  adminRestoreGrant,
  adminSetGrantOverrides,
  adminUpdatePlan,
  type CreateDiscountInputBody,
  type CreateInstallmentOfferInputBody,
  type CreatePlanInputBody,
  type CreatePriceInputBody,
  type CreateProviderPriceInputBody,
  type GrantOverridesInputBody,
  listBillingPlans,
  listBillingResources,
  type ManualGrantInputBody,
  type UpdatePlanInputBody,
} from '@relay/api-client'
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { isApiError } from '#/lib/api'
import type {
  AdminCatalog,
  AdminDiscount,
  AdminWorkspace,
  CatalogRowKind,
  DisputedInvoice,
  ProviderCaps,
  Resource,
} from '../types'
import type { PlanPublishApi } from './plan-publish'

/** Every admin query sits under ['admin'], so a change refreshes all of them. */
export const adminKeys = {
  all: () => ['admin'] as const,
  catalog: () => ['admin', 'catalog'] as const,
  discounts: () => ['admin', 'discounts'] as const,
  workspaces: (search: string) => ['admin', 'workspaces', search] as const,
  disputes: () => ['admin', 'disputes'] as const,
  resources: () => ['admin', 'resources'] as const,
  providerCaps: () => ['billing', 'providers'] as const,
}

const list = <T>(v: T[] | null | undefined): T[] => v ?? []

export function catalogQueryOptions() {
  return queryOptions({
    queryKey: adminKeys.catalog(),
    queryFn: async (): Promise<AdminCatalog> => {
      const { data } = await adminGetCatalog({ throwOnError: true })
      return {
        providers: list(data.providers),
        plans: list(data.plans).map((plan) => ({
          ...plan,
          prices: list(plan.prices).map((p) => ({
            ...p,
            interval: p.billing_type === 'one_time' ? null : p.interval,
            provider_prices: list(p.provider_prices),
            installment_offers: list(p.installment_offers).map((o) => ({
              ...o,
              amounts_minor: list(o.amounts_minor),
              provider_prices: list(o.provider_prices),
            })),
          })),
        })),
      }
    },
    retry: false,
  })
}

export function discountsQueryOptions() {
  return queryOptions({
    queryKey: adminKeys.discounts(),
    queryFn: async (): Promise<AdminDiscount[]> => {
      const { data } = await adminListDiscounts({ throwOnError: true })
      return list(data.discounts)
    },
  })
}

export function workspacesQueryOptions(search: string) {
  return queryOptions({
    queryKey: adminKeys.workspaces(search),
    queryFn: async (): Promise<AdminWorkspace[]> => {
      const { data } = await adminListWorkspaces({ query: { search }, throwOnError: true })
      return list(data.workspaces).map((w) => ({ ...w, grant: w.grant ?? null }))
    },
  })
}

export function disputesQueryOptions() {
  return queryOptions({
    queryKey: adminKeys.disputes(),
    queryFn: async (): Promise<DisputedInvoice[]> => {
      const { data } = await adminListDisputes({ throwOnError: true })
      return list(data.invoices).map((i) => ({ ...i, paid_at: i.paid_at ?? null }))
    },
  })
}

/** What plans can include. It changes only with a deploy. */
export function resourcesQueryOptions() {
  return queryOptions({
    queryKey: adminKeys.resources(),
    queryFn: async (): Promise<Resource[]> => {
      const { data } = await listBillingResources({ throwOnError: true })
      return list(data.resources)
    },
    staleTime: Number.POSITIVE_INFINITY,
  })
}

function useAdminMutation<TInput>(fn: (input: TInput) => Promise<unknown>) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => queryClient.invalidateQueries({ queryKey: adminKeys.all() }),
  })
}

export const useCreatePlan = () =>
  useAdminMutation((body: CreatePlanInputBody) => adminCreatePlan({ body, throwOnError: true }))

export const useCreatePrice = () =>
  useAdminMutation((body: CreatePriceInputBody) => adminCreatePrice({ body, throwOnError: true }))

export const useCreateInstallmentOffer = () =>
  useAdminMutation((body: CreateInstallmentOfferInputBody) =>
    adminCreateInstallmentOffer({ body, throwOnError: true }),
  )

export const useCreateProviderPrice = () =>
  useAdminMutation((body: CreateProviderPriceInputBody) =>
    adminCreateProviderPrice({ body, throwOnError: true }),
  )

const deactivators = {
  plan: adminDeactivatePlan,
  price: adminDeactivatePrice,
  installment_offer: adminDeactivateInstallmentOffer,
  provider_price: adminDeactivateProviderPrice,
} satisfies Record<CatalogRowKind, unknown>

export const useDeactivateCatalogRow = () =>
  useAdminMutation(({ kind, id }: { kind: CatalogRowKind; id: string }) =>
    deactivators[kind]({ path: { id }, throwOnError: true }),
  )

export const useCreateDiscount = () =>
  useAdminMutation((body: CreateDiscountInputBody) =>
    adminCreateDiscount({ body, throwOnError: true }),
  )

export const useDeactivateDiscount = () =>
  useAdminMutation((code: string) =>
    adminDeactivateDiscount({ path: { code }, throwOnError: true }),
  )

export const useGrantPlan = () =>
  useAdminMutation((body: ManualGrantInputBody) => adminGrantPlan({ body, throwOnError: true }))

export const useEndGrant = () =>
  useAdminMutation((workspaceId: string) =>
    adminEndGrant({ body: { workspace_id: workspaceId }, throwOnError: true }),
  )

export const useRestoreGrant = () =>
  useAdminMutation((workspaceId: string) =>
    adminRestoreGrant({ body: { workspace_id: workspaceId }, throwOnError: true }),
  )

export const useCloseDispute = () =>
  useAdminMutation(
    ({
      invoiceId,
      outcome,
      reason,
    }: {
      invoiceId: string
      outcome: 'won' | 'lost'
      reason: string
    }) =>
      adminCloseDispute({
        path: { invoice_id: invoiceId },
        body: { outcome, reason },
        throwOnError: true,
      }),
  )

export const useUpdatePlan = () =>
  useAdminMutation(({ planId, ...body }: UpdatePlanInputBody & { planId: string }) =>
    adminUpdatePlan({ path: { id: planId }, body, throwOnError: true }),
  )

export const useSetGrantOverrides = () =>
  useAdminMutation((body: GrantOverridesInputBody) =>
    adminSetGrantOverrides({ body, throwOnError: true }),
  )

/** What each payment provider can do. Public, and it changes only with a deploy. */
export function providerCapsQueryOptions() {
  return queryOptions({
    queryKey: adminKeys.providerCaps(),
    queryFn: async (): Promise<ProviderCaps[]> => {
      const { data } = await listBillingPlans({ throwOnError: true })
      return list(data.providers).map((p) => ({
        name: p.name,
        checkoutTrials: p.checkout_trials,
      }))
    },
    staleTime: Number.POSITIVE_INFINITY,
    retry: false,
  })
}

/** The calls that put a plan on sale, with no cache refresh between them: see useRefreshAdmin. */
export const planPublishApi: PlanPublishApi = {
  createPlan: async (body) => (await adminCreatePlan({ body, throwOnError: true })).data.id,
  createPrice: async (body) => (await adminCreatePrice({ body, throwOnError: true })).data.id,
  createProviderPrice: async (body) => {
    try {
      return (await adminCreateProviderPrice({ body, throwOnError: true })).data.id
    } catch (error) {
      if (isApiError(error) && error.code === 'catalog_duplicate') return 'existing'
      throw error
    }
  },
}

/** Refreshes every admin query once, after a run of calls. */
export function useRefreshAdmin() {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: adminKeys.all() })
}
