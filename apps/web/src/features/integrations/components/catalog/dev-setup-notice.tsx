import { TriangleAlertIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { env } from '#/lib/env'
import type { Overview } from '../../types'

const Code = ({ children }: { children: ReactNode }) => (
  <code className="rounded bg-background/60 px-1 py-px font-mono text-[12px]">{children}</code>
)

/**
 * Development only: what this server still needs before an app can connect in one click.
 * Production builds never render it; there, cards fall back to credentials or say it's unavailable.
 */
export function DevSetupNotice({ overview }: { overview: Overview }) {
  if (!import.meta.env.DEV) return null
  const api = env.VITE_API_URL.replace(/\/$/, '')
  const missing: { app: string; fix: ReactNode }[] = []
  if (!overview.shopify_oauth) {
    missing.push({
      app: 'Shopify',
      fix: (
        <>
          create an app in the Shopify Partner dashboard and set <Code>SHOPIFY_CLIENT_ID</Code> and{' '}
          <Code>SHOPIFY_CLIENT_SECRET</Code> in <Code>.env</Code>. Its redirect URL is{' '}
          <Code>{api}/v1/integrations/shopify/callback</Code>. An unlisted app is fine; the public
          App Store needs extra work Shopify requires.
        </>
      ),
    })
  }
  if (!overview.woocommerce_oauth) {
    missing.push({
      app: 'WooCommerce',
      fix: (
        <>
          no app to register, but <Code>API_BASE_URL</Code> must be a public https address: the
          store only sends its keys to an https URL it can reach.
        </>
      ),
    })
  }
  if (!overview.google_available) {
    missing.push({
      app: 'Google Sheets',
      fix: (
        <>
          set <Code>GOOGLE_CLIENT_ID</Code> and <Code>GOOGLE_CLIENT_SECRET</Code> (the same app as
          Google sign-in), with redirect URL <Code>{api}/v1/integrations/google/callback</Code>.
        </>
      ),
    })
  }
  if (missing.length === 0) return null

  return (
    <Alert variant="warning">
      <TriangleAlertIcon aria-hidden="true" />
      <AlertTitle className="font-bold">
        Development: one-click connect isn't set up for {missing.map((m) => m.app).join(', ')}
      </AlertTitle>
      <AlertDescription>
        <p>
          Until then those apps connect with pasted credentials. Restart the API after editing .env.
        </p>
        <ul className="mt-1 flex list-disc flex-col gap-1 pl-4">
          {missing.map((m) => (
            <li key={m.app}>
              <span className="font-bold">{m.app}:</span> {m.fix}
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  )
}
