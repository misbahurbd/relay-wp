import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { IntegrationsPage } from '#/features/integrations'
import { useAccess } from '#/features/team'

const search = z.object({
  // Set by the Google Sheets and Shopify callbacks when they send the person back.
  connected: z.string().optional().catch(undefined),
  integration_error: z.string().optional().catch(undefined),
  // WooCommerce's return: ?provider=woocommerce&success=1|0 (its user_id is dropped).
  provider: z.string().optional().catch(undefined),
  success: z.string().optional().catch(undefined),
})

export const Route = createFileRoute('/_app/_workspace/_shell/integrations/')({
  validateSearch: search,
  head: () => ({ meta: [{ title: 'Integrations · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { connected, integration_error, provider, success } = Route.useSearch()
  const navigate = Route.useNavigate()
  const { can } = useAccess(workspaceId)
  return (
    <IntegrationsPage
      workspaceId={workspaceId}
      canManage={can('integrations.manage')}
      canKeys={can('integrations.api_keys')}
      callback={{ connected, error: integration_error, provider, success }}
      onCallbackSeen={() => navigate({ search: {}, replace: true })}
    />
  )
}
