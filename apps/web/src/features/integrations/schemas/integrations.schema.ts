import { z } from 'zod'
import type { AppId, Category, KeyApp, StoreProvider } from '../types'

export type Tone = 'green' | 'blue' | 'amber' | 'red' | 'purple' | 'gray'

export const TONE: Record<Tone, string> = {
  green: 'bg-brand-soft text-brand-text',
  blue: 'bg-info-soft text-info',
  amber: 'bg-warning-soft text-warning',
  red: 'bg-destructive-soft text-destructive',
  purple: 'bg-purple-soft text-purple',
  gray: 'bg-neutral-soft text-neutral',
}

export type AppDef = {
  id: AppId
  name: string
  /** Two or three letters on the tile, standing in for the app's logo. */
  mono: string
  tone: Tone
  category: Category
  description: string
}

export const CATEGORIES: { id: Category | 'all'; label: string }[] = [
  { id: 'all', label: 'All' },
  { id: 'ecom', label: 'E-commerce' },
  { id: 'auto', label: 'Automation' },
  { id: 'dev', label: 'Developer' },
]

export const CATEGORY_LABEL: Record<Category, string> = {
  ecom: 'E-commerce',
  auto: 'Automation',
  dev: 'Developer',
}

export const APPS: AppDef[] = [
  {
    id: 'shopify',
    name: 'Shopify',
    mono: 'Sh',
    tone: 'green',
    category: 'ecom',
    description: 'Sync customers and orders. Trigger order, shipping and abandoned cart messages.',
  },
  {
    id: 'woocommerce',
    name: 'WooCommerce',
    mono: 'Wc',
    tone: 'purple',
    category: 'ecom',
    description: 'Connect your WordPress store to send order updates and recover carts.',
  },
  {
    id: 'google_sheets',
    name: 'Google Sheets',
    mono: 'GS',
    tone: 'green',
    category: 'auto',
    description: 'Add new contacts and form leads to a sheet, or import contacts from one.',
  },
  {
    id: 'zapier',
    name: 'Zapier',
    mono: 'Za',
    tone: 'amber',
    category: 'auto',
    description: 'Link Relay to 6,000+ apps with no-code Zaps.',
  },
  {
    id: 'make',
    name: 'Make',
    mono: 'Mk',
    tone: 'purple',
    category: 'auto',
    description: 'Build visual scenarios that send messages or update contacts.',
  },
  {
    id: 'n8n',
    name: 'n8n',
    mono: 'n8',
    tone: 'red',
    category: 'auto',
    description: 'Self-hosted workflows with full control over your data.',
  },
  {
    id: 'webhooks',
    name: 'Webhooks',
    mono: 'WH',
    tone: 'blue',
    category: 'dev',
    description: 'Receive real-time events for messages, contacts and broadcasts.',
  },
  {
    id: 'api_keys',
    name: 'API keys',
    mono: 'API',
    tone: 'gray',
    category: 'dev',
    description: 'Send messages and manage contacts from your own software.',
  },
]

export const APP_BY_ID = Object.fromEntries(APPS.map((a) => [a.id, a])) as Record<AppId, AppDef>

export const KEY_APPS: KeyApp[] = ['zapier', 'make', 'n8n']
export const STORE_PROVIDERS: StoreProvider[] = ['shopify', 'woocommerce']

/** How to use a key in each no-code app. */
export const KEY_APP_STEPS: Record<KeyApp, string[]> = {
  zapier: [
    'In Zapier, add a "Webhooks by Zapier" or HTTP step.',
    'Send requests to the Relay API with the header Authorization: Bearer <your key>.',
    'Add X-Workspace-ID with your workspace ID to every request.',
  ],
  make: [
    'In Make, add an HTTP "Make a request" module.',
    'Set the header Authorization: Bearer <your key>.',
    'Add X-Workspace-ID with your workspace ID to every request.',
  ],
  n8n: [
    'In n8n, add an HTTP Request node with "Header Auth" credentials.',
    'Name the header Authorization and set the value to Bearer <your key>.',
    'Add X-Workspace-ID with your workspace ID to every request.',
  ],
}

/** Messages for ?integration_error=<code> after a provider sends the person back. */
export const CALLBACK_ERRORS: Record<string, string> = {
  invalid_oauth_state:
    'That connection attempt expired or was started in another browser. Start again.',
  shopify_exchange_failed: "Shopify didn't confirm the connection. Try again.",
  store_unreachable: "We couldn't reach the store. Check the address and try again.",
  credentials_rejected: "The store didn't accept Relay's access. Try again.",
  google_exchange_failed: "Google didn't confirm the connection. Try again.",
  google_no_offline_access:
    "Google didn't grant offline access. Remove Relay from your Google account's connections and try again.",
}

export const keySchema = z.object({
  name: z.string().trim().min(1, 'Name the key').max(80, 'Use 80 characters or fewer'),
})

const httpsUrl = z
  .string()
  .trim()
  .min(1, 'Enter the URL')
  .refine((v) => {
    try {
      const u = new URL(v)
      return u.protocol === 'https:' || (import.meta.env.DEV && u.protocol === 'http:')
    } catch {
      return false
    }
  }, 'Use a full https:// URL')

export const endpointSchema = z.object({
  url: httpsUrl,
  description: z.string().trim().max(200, 'Use 200 characters or fewer'),
  events: z.array(z.string()).min(1, 'Pick at least one event'),
  enabled: z.boolean(),
})

const shopDomain = z
  .string()
  .trim()
  .min(1, 'Enter your store address')
  .regex(
    /^(https?:\/\/)?[a-z0-9][a-z0-9-]*(\.myshopify\.com)?\/?/i,
    'Use your myshopify.com address',
  )

export const shopifySchema = z.object({
  shop_domain: shopDomain,
  access_token: z.string().trim().min(1, 'Paste the Admin API access token'),
})

/** One-click connect asks only where the store is. */
export const storeAddressSchema = { shopify: shopDomain, woocommerce: httpsUrl }

/** What a store connection lets Relay do, shown before the owner approves it. */
export const STORE_ACCESS: Record<StoreProvider, string[]> = {
  shopify: ['Read customers', 'Read orders'],
  woocommerce: ['Read customers, orders and products (read-only REST keys)'],
}

export const wooSchema = z.object({
  store_url: httpsUrl,
  consumer_key: z.string().trim().min(1, 'Paste the consumer key'),
  consumer_secret: z.string().trim().min(1, 'Paste the consumer secret'),
})
