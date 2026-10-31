import type { LucideIcon } from 'lucide-react'
import {
  CalendarDaysIcon,
  HashIcon,
  MessageCirclePlusIcon,
  MoonIcon,
  PackageCheckIcon,
  ShoppingCartIcon,
} from 'lucide-react'
import type { Automation, Kind } from '../types'

export const KINDS: Record<
  Kind,
  { title: string; description: string; icon: LucideIcon; iconClass: string }
> = {
  welcome: {
    title: 'Welcome message',
    description: 'Greet new contacts the first time they message you.',
    icon: MessageCirclePlusIcon,
    iconClass: 'bg-brand-soft text-brand-text',
  },
  away: {
    title: 'Away message',
    description: 'Auto-reply outside working hours.',
    icon: MoonIcon,
    iconClass: 'bg-info-soft text-info',
  },
  keyword: {
    title: 'Keyword auto-replies',
    description: 'Answer common questions like price, hours or location.',
    icon: HashIcon,
    iconClass: 'bg-warning-soft text-warning',
  },
  cart: {
    title: 'Abandoned cart reminder',
    description: 'Remind shoppers who left items in their cart.',
    icon: ShoppingCartIcon,
    iconClass: 'bg-destructive-soft text-destructive',
  },
  order: {
    title: 'Order confirmation',
    description: 'Send order details as soon as a store order is placed.',
    icon: PackageCheckIcon,
    iconClass: 'bg-brand-soft text-brand-text',
  },
  drip: {
    title: 'Drip campaign',
    description: 'A welcome series over a contact’s first days.',
    icon: CalendarDaysIcon,
    iconClass: 'bg-neutral-soft text-neutral',
  },
}

export const WELCOME_DELAYS = [
  { value: 0, label: 'Immediately' },
  { value: 1, label: 'After 1 minute' },
  { value: 5, label: 'After 5 minutes' },
]

export const CART_DELAYS = [
  { value: 30, label: '30 minutes after checkout is left' },
  { value: 60, label: '1 hour after checkout is left' },
  { value: 240, label: '4 hours after checkout is left' },
  { value: 1440, label: '24 hours after checkout is left' },
]

export const ORDER_TRIGGERS = [
  { value: 'shopify_order_created', label: 'Shopify · Order created' },
  { value: 'shopify_order_paid', label: 'Shopify · Order paid' },
  { value: 'woocommerce_order_created', label: 'WooCommerce · Order created' },
]

export const ORDER_SOURCES = [
  { value: 'customer_first_name', label: 'Customer first name' },
  { value: 'order_number', label: 'Order number' },
  { value: 'order_total', label: 'Order total' },
  { value: 'tracking_url', label: 'Tracking URL' },
  { value: 'custom', label: 'Custom text…' },
]

export const CART_SOURCES = [
  { value: 'customer_first_name', label: 'Customer first name' },
  { value: 'cart_total', label: 'Cart total' },
  { value: 'checkout_url', label: 'Checkout link' },
  { value: 'discount_code', label: 'Discount code' },
  { value: 'custom', label: 'Custom text…' },
]

export const DAY_NAMES: Record<string, { short: string; name: string }> = {
  sat: { short: 'Sat', name: 'Saturday' },
  sun: { short: 'Sun', name: 'Sunday' },
  mon: { short: 'Mon', name: 'Monday' },
  tue: { short: 'Tue', name: 'Tuesday' },
  wed: { short: 'Wed', name: 'Wednesday' },
  thu: { short: 'Thu', name: 'Thursday' },
  fri: { short: 'Fri', name: 'Friday' },
}

/** The zones the browser knows, for the Away timezone select. */
export function timezones(current: string) {
  const zones = Intl.supportedValuesOf('timeZone')
  return zones.includes(current) ? zones : [current, ...zones]
}

function clock(time: string) {
  const [h = 0, m = 0] = time.split(':').map(Number)
  const suffix = h < 12 ? 'AM' : 'PM'
  const hour = h % 12 || 12
  return m ? `${hour}:${String(m).padStart(2, '0')} ${suffix}` : `${hour} ${suffix}`
}

/** "Auto-reply outside working hours: Sat–Thu, 10 AM–8 PM." from the saved hours. */
function awayDescription(a: Automation) {
  const open = (a.settings.away?.hours ?? []).filter((d) => d.open)
  const first = open[0]
  const last = open[open.length - 1]
  if (!first || !last) return 'Auto-reply at all hours, because every day is closed.'
  const days = (d: string) => DAY_NAMES[d]?.short ?? d
  const range = first === last ? days(first.day) : `${days(first.day)}–${days(last.day)}`
  return `Auto-reply outside working hours: ${range}, ${clock(first.from)}–${clock(first.to)}.`
}

function dripDescription(a: Automation) {
  const steps = a.settings.drip?.steps ?? []
  const lastDay = steps[steps.length - 1]?.day ?? 0
  return `A ${steps.length}-message welcome series over the first ${lastDay} days.`
}

export function description(a: Automation) {
  if (a.kind === 'away') return awayDescription(a)
  if (a.kind === 'drip') return dripDescription(a)
  return KINDS[a.kind].description
}

/** The card's stat line. */
export function stat(a: Automation) {
  switch (a.kind) {
    case 'keyword':
      return `${a.settings.keyword?.rules?.length ?? 0} rules · ${a.sent.toLocaleString()} replies this month`
    case 'cart':
      return `${a.sent.toLocaleString()} sent · 0 carts recovered`
    case 'order':
      return `${a.sent.toLocaleString()} sent`
    case 'drip':
      return `${a.enabled ? 'Active' : 'Paused'} · 0 contacts in sequence`
    default:
      return `Sent ${a.sent.toLocaleString()} times this month`
  }
}

/** How many {{n}} variables a template's text has. */
export function variableCount(text: string) {
  const numbers = [...text.matchAll(/\{\{(\d+)\}\}/g)].map((m) => Number(m[1]))
  return numbers.length ? Math.max(...numbers) : 0
}
