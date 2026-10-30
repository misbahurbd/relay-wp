import type { CardDto } from '@relay/api-client'

const BRANDS: Record<string, string> = {
  amex: 'American Express',
  diners: 'Diners Club',
  discover: 'Discover',
  eftpos_au: 'eftpos',
  jcb: 'JCB',
  link: 'Link',
  mastercard: 'Mastercard',
  unionpay: 'UnionPay',
  visa: 'Visa',
}

/** "Visa" from the provider's "visa"; an unknown brand is shown as the provider sent it. */
export function cardBrand(brand: string) {
  return BRANDS[brand] ?? (brand ? brand[0].toUpperCase() + brand.slice(1) : 'Card')
}

/** "Visa •••• 4242". */
export function cardLabel(card: CardDto) {
  return `${cardBrand(card.brand)} •••• ${card.last4}`
}

/** "08/28". */
export function cardExpiry(card: CardDto) {
  return `${String(card.exp_month).padStart(2, '0')}/${String(card.exp_year % 100).padStart(2, '0')}`
}
