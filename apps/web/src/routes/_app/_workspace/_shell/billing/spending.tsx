import { createFileRoute, redirect } from '@tanstack/react-router'
import { SpendingPage } from '#/features/billing'
import { useAccess } from '#/features/team'
import { env } from '#/lib/env'

export const Route = createFileRoute('/_app/_workspace/_shell/billing/spending')({
  // The page runs on sample data only; the preview kill switch hides it (docs/PHASE-7-BILLING.md).
  beforeLoad: () => {
    if (!env.VITE_BILLING_PREVIEW) throw redirect({ to: '/billing' })
  },
  head: () => ({ meta: [{ title: 'Spending · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { access, can } = useAccess(workspaceId)
  return (
    <SpendingPage
      workspaceId={workspaceId}
      // Until access loads, show the page: it holds sample data only.
      canView={!access || can('billing.view') || can('billing.pay')}
    />
  )
}
