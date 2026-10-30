import type { AppId, AppStatus, KeyApp, Overview } from '../../types'
import { percent, plural, relativeTime } from '../kit/format'

/** What a card shows, worked out from the overview. */
export function appStatus(id: AppId, o: Overview): AppStatus {
  switch (id) {
    case 'shopify':
    case 'woocommerce':
    case 'google_sheets': {
      const available = id !== 'google_sheets' || o.google_available
      const oneClick =
        id === 'google_sheets' || (id === 'shopify' ? o.shopify_oauth : o.woocommerce_oauth)
      const c = o.connections.find((x) => x.provider === id)
      if (!c || c.status === 'disconnected') return { connected: false, available, oneClick }
      const checked = c.last_checked_at ? ` · checked ${relativeTime(c.last_checked_at)}` : ''
      return {
        connected: true,
        available,
        oneClick,
        error: c.status === 'error' ? (c.last_error ?? 'The last check failed') : undefined,
        meta: `${c.account_label}${checked}`,
      }
    }
    case 'zapier':
    case 'make':
    case 'n8n': {
      const a = o.apps.find((x) => x.app === (id satisfies KeyApp))
      if (!a) return { connected: false, available: true }
      const used = a.last_used_at ? `last used ${relativeTime(a.last_used_at)}` : 'not used yet'
      return {
        connected: true,
        available: true,
        meta: `${plural(a.active_keys, 'active key')} · ${used}`,
      }
    }
    case 'webhooks': {
      const w = o.webhooks
      if (w.endpoints === 0) return { connected: false, available: true }
      const health =
        w.week_total > 0
          ? `${percent(w.week_succeeded, w.week_total)} delivered`
          : 'no deliveries yet'
      return {
        connected: true,
        available: true,
        meta: `${plural(w.endpoints, 'endpoint')} · ${health}`,
        error:
          w.enabled < w.endpoints
            ? `${plural(w.endpoints - w.enabled, 'endpoint')} off`
            : undefined,
      }
    }
    case 'api_keys': {
      const k = o.api_keys
      if (k.active === 0) return { connected: false, available: true }
      const used = k.last_used_at ? `last used ${relativeTime(k.last_used_at)}` : 'not used yet'
      return {
        connected: true,
        available: true,
        meta: `${plural(k.active, 'active key')} · ${used}`,
      }
    }
  }
}
