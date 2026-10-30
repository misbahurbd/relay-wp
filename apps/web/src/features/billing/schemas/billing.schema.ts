import { z } from 'zod'
import type { Installment, Invoice, Order, ProviderName, Subscription } from '../types'

export const discountCodeSchema = z
  .string()
  .trim()
  .min(1, 'Enter a discount code')
  .max(64, 'Discount codes are at most 64 characters')

const optional = (max: number, message: string) => z.string().trim().max(max, message)

export const billingProfileSchema = z.object({
  legal_name: z
    .string()
    .trim()
    .min(1, 'Enter the person or company invoices are made out to')
    .max(200, 'Use at most 200 characters'),
  email: z.email('Enter an email address, like billing@example.com'),
  phone: optional(40, 'Use at most 40 characters'),
  address_line1: optional(200, 'Use at most 200 characters'),
  address_line2: optional(200, 'Use at most 200 characters'),
  city: optional(100, 'Use at most 100 characters'),
  state: optional(100, 'Use at most 100 characters'),
  postal_code: optional(20, 'Use at most 20 characters'),
  country: z
    .string()
    .trim()
    .refine((v) => v === '' || /^[A-Za-z]{2}$/.test(v), 'Use the 2-letter country code, like US'),
  tax_id_type: optional(20, 'Use at most 20 characters'),
  tax_id: optional(40, 'Use at most 40 characters'),
})

export type BillingProfileValues = z.infer<typeof billingProfileSchema>

export const PROVIDER_LABEL: Record<ProviderName, string> = {
  stripe: 'Card (Stripe)',
  creem: 'Creem',
  paypal: 'PayPal',
  sslcommerz: 'SSLCommerz',
}

export type Tone = 'success' | 'info' | 'warning' | 'danger' | 'neutral'

export const SUBSCRIPTION_STATUS: Record<Subscription['status'], { label: string; tone: Tone }> = {
  incomplete: { label: 'Awaiting payment', tone: 'warning' },
  trialing: { label: 'Trial', tone: 'info' },
  active: { label: 'Active', tone: 'success' },
  past_due: { label: 'Past due', tone: 'danger' },
  canceled: { label: 'Canceled', tone: 'neutral' },
}

export const ORDER_STATUS: Record<Order['status'], { label: string; tone: Tone }> = {
  pending: { label: 'Awaiting payment', tone: 'warning' },
  in_installments: { label: 'Paying in installments', tone: 'info' },
  paid: { label: 'Paid in full', tone: 'success' },
  defaulted: { label: 'Defaulted', tone: 'danger' },
  upgraded: { label: 'Upgraded', tone: 'neutral' },
  refunded: { label: 'Refunded', tone: 'neutral' },
  canceled: { label: 'Canceled', tone: 'neutral' },
}

export const INSTALLMENT_STATUS: Record<Installment['status'], { label: string; tone: Tone }> = {
  scheduled: { label: 'Scheduled', tone: 'neutral' },
  due: { label: 'Due', tone: 'warning' },
  paid: { label: 'Paid', tone: 'success' },
  overdue: { label: 'Overdue', tone: 'danger' },
  failed: { label: 'Failed', tone: 'danger' },
  waived: { label: 'Waived', tone: 'neutral' },
  canceled: { label: 'Canceled', tone: 'neutral' },
}

/** Installments a member can pay now from the schedule. */
export const PAYABLE: Installment['status'][] = ['due', 'overdue', 'failed']

export const INVOICE_STATUS: Record<Invoice['status'], { label: string; tone: Tone }> = {
  open: { label: 'Open', tone: 'warning' },
  paid: { label: 'Paid', tone: 'success' },
  failed: { label: 'Failed', tone: 'danger' },
  void: { label: 'Void', tone: 'neutral' },
  partially_refunded: { label: 'Partly refunded', tone: 'info' },
  refunded: { label: 'Refunded', tone: 'neutral' },
  disputed: { label: 'Disputed', tone: 'danger' },
}

export const GRANT_SOURCE: Record<'subscription' | 'order' | 'trial' | 'manual', string> = {
  subscription: 'Subscription',
  order: 'Lifetime deal',
  trial: 'Free trial',
  manual: 'Granted by Relay',
}
