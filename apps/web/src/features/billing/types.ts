import type {
  CardDto,
  ChangeQuoteOutputBody,
  CheckoutSessionDto,
  CurrentPlanDto,
  DiscountPreviewOutputBody,
  InstallmentDto,
  InvoiceDto,
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
// Every resource key (GET /v1/billing/resources) to its cap; -1 is unlimited.
export type Limits = CurrentPlanDto['limits']

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
  /** One line under the name on the pricing table; may be empty. */
  description: string
  /** Bullet points for the pricing table; empty means show the limits instead. */
  features: string[]
  recommended: boolean
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

export type PaymentMethod = {
  provider: ProviderName | null
  card: CardDto | null
  /** False when the provider never shares card details (Creem). */
  card_details: boolean
  can_manage: boolean
}

// ---------- Preview (sample data until the backend exists; api/billing.preview.ts) ----------
// Amounts follow the API's rules: plan money in minor units, Meta's sub-cent rates in micros.

/** One usage meter. limit -1 is unlimited. */
export type UsageRow = { key: string; label: string; used: number; limit: number; preview: boolean }

export type MetaCostCategory = 'marketing' | 'utility' | 'authentication' | 'service'

export type MetaCostRow = {
  category: MetaCostCategory
  messages: number
  /** Average charge per message, in micros; 0 for free messages. */
  avg_micros: number
  cost_minor: number
}

export type MetaCostSummary = {
  currency: string
  rows: MetaCostRow[]
  messages: number
  total_minor: number
}

export type Wallet = {
  currency: string
  balance_minor: number
  auto_recharge: { enabled: boolean; below_minor: number; add_minor: number }
}

// ---------- Spending page preview (components/spending) ----------

export type SpendingPeriod = 'this_month' | 'last_month' | 'last_3_months'

/** Meta charges on one day (or week) by paid category, in minor units. */
export type SpendPoint = {
  /** First day the point covers, YYYY-MM-DD. */
  date: string
  marketing_minor: number
  utility_minor: number
  authentication_minor: number
}

export type SpendByNumber = { name: string; phone: string; cost_minor: number }

export type SpendByCountry = {
  /** ISO 3166 code; null for the "other countries" row. */
  code: string | null
  name: string
  cost_minor: number
}

/** Everything Relay and Meta charged in one period. */
export type Spending = {
  period: SpendingPeriod
  currency: string
  /** First and last day, YYYY-MM-DD. */
  from: string
  to: string
  plan_minor: number
  /** Meta charges by category; service conversations are free. */
  categories: MetaCostRow[]
  meta_minor: number
  paid_messages: number
  free_messages: number
  /** Average Meta charge per paid message, in micros. */
  avg_micros: number
  /** The period's outlook: projected total, closed total, or the monthly average. */
  outlook: {
    kind: 'projected' | 'final' | 'monthly_average'
    amount_minor: number
    note: string
  }
  /** Change in the total against the period before, in tenths of a percent (92 is +9.2%). */
  change: { against: string; permille: number; note: string }
  series: { unit: 'day' | 'week'; points: SpendPoint[] }
  numbers: SpendByNumber[]
  countries: SpendByCountry[]
}

/** This month's budget. Plan plus Meta charges count against it. */
export type SpendingBudget = {
  currency: string
  /** First day of the month, YYYY-MM-DD. */
  month: string
  budget_minor: number
  spent_minor: number
  projected_minor: number
  /** Last day of the projection, YYYY-MM-DD. */
  projected_to: string
  alert_at_80: boolean
  pause_marketing_at_100: boolean
  /** Where the 80% email goes, besides billing admins. */
  alert_email: string
}

export type CampaignSource = 'broadcast' | 'automation' | 'api'

/** What one broadcast, automation or API flow cost this month. */
export type CampaignSpend = {
  id: string
  name: string
  source: CampaignSource
  /** Template, integration or progress note shown under the name. */
  detail: string | null
  /** Send day, YYYY-MM-DD; null when it ran all month. */
  date: string | null
  recipients: number
  category: MetaCostCategory
  cost_minor: number
  /** True while a broadcast is still sending, so the cost is a running total. */
  in_progress: boolean
  /** Null when the messages don't expect replies. */
  cost_per_reply_minor: number | null
}

export type CampaignSpendList = { currency: string; month: string; rows: CampaignSpend[] }

/** One wallet movement. amount_minor is negative for charges. */
export type WalletEntry = {
  id: string
  date: string
  description: string
  detail: string
  amount_minor: number
  balance_after_minor: number
}

/** What the wallet panel shows around the balance. */
export type WalletOutlook = {
  currency: string
  avg_daily_spend_minor: number
  card: { brand: string; last4: string; exp_month: number; exp_year: number }
  /** The next scheduled send the balance doesn't cover, if any. */
  shortfall: { broadcast: string; needed_minor: number } | null
  /** Quick amounts offered in the Top up dialog. */
  top_up_options_minor: number[]
  min_top_up_minor: number
}
