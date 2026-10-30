import type {
  ApiKeyDto,
  ConnectionDto,
  DeliveryDto,
  EndpointDto,
  IntegrationsOverviewOutputBody,
  WebhookEventDto,
} from '@relay/api-client'

export type Provider = ConnectionDto['provider']
export type StoreProvider = Extract<Provider, 'shopify' | 'woocommerce'>
export type KeyApp = 'zapier' | 'make' | 'n8n'
/** Every card on the page. */
export type AppId = Provider | KeyApp | 'webhooks' | 'api_keys'
export type Category = 'ecom' | 'auto' | 'dev'

export type Connection = ConnectionDto
export type ApiKey = ApiKeyDto
export type Delivery = DeliveryDto
export type WebhookEvent = WebhookEventDto
export type Endpoint = Omit<EndpointDto, 'events'> & { events: string[] }

/** The overview with its lists never null. */
export type Overview = Omit<IntegrationsOverviewOutputBody, 'apps' | 'connections'> & {
  apps: NonNullable<IntegrationsOverviewOutputBody['apps']>
  connections: Connection[]
}

export type NewApiKey = { apiKey: ApiKey; key: string }

export type EndpointInput = {
  url: string
  description: string
  events: string[]
  enabled: boolean
}

export type StoreCredentials = {
  shop_domain?: string
  access_token?: string
  store_url?: string
  consumer_key?: string
  consumer_secret?: string
}

/** One card's state, worked out from the overview. */
export type AppStatus = {
  connected: boolean
  /** The connection exists but its last check failed. */
  error?: string
  /** One line under the description: "greenleaf.myshopify.com · checked 5 min ago". */
  meta?: string
  /** False when this server can't connect the app at all. */
  available: boolean
  /** Connects by signing in at the provider, rather than with pasted credentials. */
  oneClick?: boolean
}
