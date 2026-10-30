import {
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
  listBillingResources,
  type ManualGrantInputBody,
  type UpdatePlanInputBody,
} from '@relay/api-client'
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import type {
  AdminCatalog,
  AdminDiscount,
  AdminWorkspace,
  CatalogRowKind,
  Resource,
} from '../types'

/** Every admin query sits under ['admin'], so a change refreshes all of them. */
export const adminKeys = {
  all: () => ['admin'] as const,
  catalog: () => ['admin', 'catalog'] as const,
  discounts: () => ['admin', 'discounts'] as const,
  workspaces: (search: string) => ['admin', 'workspaces', search] as const,
  resources: () => ['admin', 'resources'] as const,
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

export const useUpdatePlan = () =>
  useAdminMutation(({ planId, ...body }: UpdatePlanInputBody & { planId: string }) =>
    adminUpdatePlan({ path: { id: planId }, body, throwOnError: true }),
  )

export const useSetGrantOverrides = () =>
  useAdminMutation((body: GrantOverridesInputBody) =>
    adminSetGrantOverrides({ body, throwOnError: true }),
  )
