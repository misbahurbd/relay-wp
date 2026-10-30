import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { BillingPage } from '#/features/billing'
import { useAccess } from '#/features/team'

const search = z.object({
  tab: z.enum(['overview', 'invoices', 'details']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/_workspace/_shell/billing/')({
  validateSearch: search,
  head: () => ({ meta: [{ title: 'Billing · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { tab = 'overview' } = Route.useSearch()
  const navigate = Route.useNavigate()
  const { access, can } = useAccess(workspaceId)
  return (
    <BillingPage
      workspaceId={workspaceId}
      // Until access loads, show the page: the API refuses what the member can't see anyway.
      canView={!access || can('billing.view') || can('billing.pay')}
      canPay={can('billing.pay')}
      tab={tab}
      onTab={(t) => navigate({ search: { tab: t === 'overview' ? undefined : t }, replace: true })}
    />
  )
}
