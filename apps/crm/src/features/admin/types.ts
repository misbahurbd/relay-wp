import type {
  AdminDiscountDto,
  AdminGrantDto,
  AdminOfferDto,
  AdminPlanDto,
  AdminPriceDto,
  AdminProviderPriceDto,
  AdminWorkspaceDto,
  DisputedInvoiceDto,
  ResourceDto,
} from '@relay/api-client'

export type ProviderPrice = AdminProviderPriceDto

export type InstallmentOffer = Omit<AdminOfferDto, 'amounts_minor' | 'provider_prices'> & {
  amounts_minor: number[]
  provider_prices: ProviderPrice[]
}

export type Price = Omit<AdminPriceDto, 'interval' | 'provider_prices' | 'installment_offers'> & {
  interval: 'month' | 'year' | null
  provider_prices: ProviderPrice[]
  installment_offers: InstallmentOffer[]
}

export type Plan = Omit<AdminPlanDto, 'prices'> & { prices: Price[] }

export type AdminCatalog = { plans: Plan[]; providers: string[] }

export type AdminDiscount = AdminDiscountDto

export type AdminWorkspace = Omit<AdminWorkspaceDto, 'grant'> & { grant: AdminGrantDto | null }

export type DisputedInvoice = Omit<DisputedInvoiceDto, 'paid_at'> & { paid_at: string | null }

export type CatalogRowKind = 'plan' | 'price' | 'installment_offer' | 'provider_price'

export type Provider = AdminProviderPriceDto['provider']

export type Resource = ResourceDto

/** Resource key to cap; -1 is unlimited. */
export type Limits = Plan['limits']

/** What the plan editor needs to know about an enabled payment provider. */
export type ProviderCaps = { name: Provider; checkoutTrials: boolean }
