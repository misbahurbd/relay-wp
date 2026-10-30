import { createFileRoute } from '@tanstack/react-router'
import { BillingPage } from '#/features/billing'
import { useAccess } from '#/features/team'

export const Route = createFileRoute('/_app/_workspace/_shell/billing/')({
  head: () => ({ meta: [{ title: 'Billing · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { access, can } = useAccess(workspaceId)
  return (
    <BillingPage
      workspaceId={workspaceId}
      // Until access loads, show the page: the API refuses what the member can't see anyway.
      canView={!access || can('billing.view') || can('billing.pay')}
      canPay={can('billing.pay')}
    />
  )
}
