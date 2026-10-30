import type {
  ChangeQuoteOutputBody,
  CheckoutSessionDto,
  CurrentPlanDto,
  DiscountPreviewOutputBody,
  InstallmentDto,
  InvoiceDto,
  LimitsDto,
  OfferProviderDto,
  OrderDto,
  ProfileOutputBody,
  ProviderDto,
  ScheduledPaymentDto,
  StartedCheckoutOutputBody,
  SubscriptionDto,
  UpdateProfileInputBody1,
} from '@relay/api-client'

// The API sends null for empty lists; the queries turn those into [] so components never check.

export type ProviderName = ProviderDto['name']
export type Provider = ProviderDto & { currencies: string[] }
export type Limits = LimitsDto

export type InstallmentOffer = {
  id: string
  count: number
  interval: 'month'
  grace_days: number
  total_minor: number
  amounts_minor: number[]
  providers: OfferProviderDto[]
}

export type Price = {
  id: string
  plan_id: string
  plan_code: string
  amount_minor: number
  currency: string
  billing_type: 'recurring' | 'one_time'
  /** Null for one-time (lifetime) prices. */
  interval: 'month' | 'year' | null
  providers: ProviderName[]
  installment_offers: InstallmentOffer[]
}

export type Plan = {
  id: string
  code: string
  name: string
  version: number
  limits: Limits
  trial_days: number
  trial_requires_card: boolean
  prices: Price[]
}

export type Catalog = { plans: Plan[]; providers: Provider[] }

/** The pricing toggle: monthly and yearly subscriptions, or a one-time lifetime deal. */
export type Cadence = 'month' | 'year' | 'lifetime'

export type Installment = InstallmentDto
export type Order = Omit<OrderDto, 'installments' | 'price'> & {
  installments: Installment[]
  price: Price | null
}
export type Subscription = Omit<SubscriptionDto, 'price'> & { price: Price | null }
export type CurrentPlan = CurrentPlanDto

export type Overview = {
  plan: CurrentPlan
  subscription: Subscription | null
  order: Order | null
}

export type Invoice = InvoiceDto
export type BillingProfile = ProfileOutputBody
export type BillingProfileInput = UpdateProfileInputBody1

export type StartedCheckout = StartedCheckoutOutputBody
export type CheckoutSession = CheckoutSessionDto
export type DiscountPreview = Omit<DiscountPreviewOutputBody, 'installments_minor'> & {
  installments_minor: number[]
}
export type ScheduledPayment = ScheduledPaymentDto
export type OrderQuote = {
  plan_price_id: string
  installment_offer_id: string
  currency: string
  total_minor: number
  discount_minor: number
  credit_minor: number
  due_now_minor: number
  replaces_subscription_id: string
  upgrades_order_id: string
  schedule: ScheduledPayment[]
}
export type ChangeQuote = ChangeQuoteOutputBody

/** What the member chose on the pricing page; the checkout page reads it from its URL. */
export type Selection = { priceId: string; offerId?: string }
