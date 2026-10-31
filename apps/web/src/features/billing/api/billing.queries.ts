import {
  type CatalogOutputBody,
  getBillingOverview,
  getBillingProfile,
  getBillingUsage,
  getCheckoutSession,
  getInvoice,
  getPaymentMethod,
  getSeats,
  listBillingPlans,
  listInvoices,
  listWhatsappNumbers,
  type OrderDto,
  type PriceDto,
  previewSubscriptionChange,
  quoteOrder,
  type SubscriptionDto,
} from '@relay/api-client'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  BillingProfile,
  Catalog,
  ChangeQuote,
  CheckoutSession,
  Order,
  OrderQuote,
  Overview,
  PaymentMethod,
  Price,
  Subscription,
} from '../types'
import { billingKeys } from './billing.keys'

const list = <T>(v: T[] | null | undefined): T[] => v ?? []

export function toPrice(p: PriceDto): Price {
  return {
    ...p,
    interval: p.billing_type === 'one_time' ? null : p.interval,
    providers: list(p.providers) as Price['providers'],
    installment_offers: list(p.installment_offers).map((o) => ({
      ...o,
      amounts_minor: list(o.amounts_minor),
      providers: list(o.providers),
    })),
  }
}

function toCatalog(data: CatalogOutputBody): Catalog {
  return {
    plans: list(data.plans).map((plan) => ({
      ...plan,
      features: list(plan.features),
      prices: list(plan.prices).map(toPrice),
    })),
    providers: list(data.providers).map((p) => ({ ...p, currencies: list(p.currencies) })),
  }
}

function toOrder(o: OrderDto): Order {
  return {
    ...o,
    installments: list(o.installments).sort((a, b) => a.seq - b.seq),
    price: o.price ? toPrice(o.price) : null,
  }
}

function toSubscription(s: SubscriptionDto): Subscription {
  return { ...s, price: s.price ? toPrice(s.price) : null }
}

/** The public plan catalog and the payment providers this server has enabled. */
export const plansQueryOptions = () =>
  queryOptions({
    queryKey: billingKeys.plans(),
    queryFn: async ({ signal }) => {
      const { data } = await listBillingPlans({ signal, throwOnError: true })
      return toCatalog(data)
    },
    staleTime: 5 * 60_000,
  })

export const overviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.overview(workspaceId),
    queryFn: async ({ signal }): Promise<Overview> => {
      const { data } = await getBillingOverview({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return {
        plan: data.plan,
        subscription: data.subscription ? toSubscription(data.subscription) : null,
        order: data.order ? toOrder(data.order) : null,
      }
    },
  })

export const invoicesQueryOptions = (workspaceId: string) =>
  infiniteQueryOptions({
    queryKey: billingKeys.invoices(workspaceId),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listInvoices({
        headers: inWorkspace(workspaceId),
        query: { cursor: pageParam },
        signal,
        throwOnError: true,
      })
      return { invoices: list(data.invoices), next: data.next_cursor ?? undefined }
    },
    getNextPageParam: (last) => last.next,
  })

export const invoiceQueryOptions = (workspaceId: string, invoiceId: string) =>
  queryOptions({
    queryKey: billingKeys.invoice(workspaceId, invoiceId),
    queryFn: async ({ signal }) => {
      const { data } = await getInvoice({
        headers: inWorkspace(workspaceId),
        path: { invoiceId },
        signal,
        throwOnError: true,
      })
      return data
    },
  })

/** The billing details, or null before anyone has saved them. */
export const profileQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.profile(workspaceId),
    queryFn: async ({ signal }): Promise<BillingProfile | null> => {
      const { data, error, response } = await getBillingProfile({
        headers: inWorkspace(workspaceId),
        signal,
      })
      if (response?.status === 404) return null
      if (error || !data) throw error
      return data
    },
  })

/** What a lifetime deal costs this workspace: discount, credit from what it replaces, schedule. */
export const quoteQueryOptions = (
  workspaceId: string,
  priceId: string,
  offerId: string | undefined,
  code: string | undefined,
) =>
  queryOptions({
    queryKey: billingKeys.quote(workspaceId, priceId, offerId ?? '', code ?? ''),
    queryFn: async ({ signal }): Promise<OrderQuote> => {
      const { data } = await quoteOrder({
        headers: inWorkspace(workspaceId),
        body: { plan_price_id: priceId, installment_offer_id: offerId, discount_code: code },
        signal,
        throwOnError: true,
      })
      return { ...data, schedule: list(data.schedule).sort((a, b) => a.seq - b.seq) }
    },
    retry: false,
  })

export const changePreviewQueryOptions = (workspaceId: string, priceId: string) =>
  queryOptions({
    queryKey: billingKeys.changePreview(workspaceId, priceId),
    queryFn: async ({ signal }): Promise<ChangeQuote> => {
      const { data } = await previewSubscriptionChange({
        headers: inWorkspace(workspaceId),
        body: { plan_price_id: priceId },
        signal,
        throwOnError: true,
      })
      return data
    },
    retry: false,
  })

const SETTLED: CheckoutSession['status'][] = ['completed', 'failed', 'expired']

/** A checkout's state. Polls while it is pending: the provider's webhook completes it. */
export const checkoutSessionQueryOptions = (workspaceId: string, sessionId: string) =>
  queryOptions({
    queryKey: billingKeys.checkout(workspaceId, sessionId),
    queryFn: async ({ signal }): Promise<CheckoutSession> => {
      const { data } = await getCheckoutSession({
        headers: inWorkspace(workspaceId),
        path: { sessionId },
        signal,
        throwOnError: true,
      })
      return data
    },
    refetchInterval: (query) =>
      query.state.status === 'error' ||
      (query.state.data && SETTLED.includes(query.state.data.status))
        ? false
        : 2000,
  })

/** The card the payment provider charges next, read from the provider on each request. */
export const paymentMethodQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.paymentMethod(workspaceId),
    queryFn: async ({ signal }): Promise<PaymentMethod> => {
      const { data } = await getPaymentMethod({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return {
        provider: data.provider ?? null,
        card: data.card ?? null,
        card_details: data.card_details,
        can_manage: data.can_manage,
      }
    },
  })

/** Seats used (members plus pending invites) against the plan's cap, for the usage meters. */
export const seatsUsageQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.seats(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await getSeats({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return { used: data.used + data.reserved, limit: data.limit }
    },
  })

/** WhatsApp numbers that count toward the plan's cap: every number not disconnected. */
export const numbersUsageQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.numbers(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await listWhatsappNumbers({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return list(data.numbers).filter((n) => n.status !== 'disconnected').length
    },
  })

/** This month's use of each monthly plan resource, keyed by resource, for the usage meters. */
export const monthlyUsageQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.monthly(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await getBillingUsage({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return Object.fromEntries(list(data.resources).map((r) => [r.resource, r]))
    },
  })
