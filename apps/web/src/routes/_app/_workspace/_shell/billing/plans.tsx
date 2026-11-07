import { createFileRoute } from '@tanstack/react-router'
import { PlanPicker } from '#/features/billing'
import { useAccess } from '#/features/team'

export const Route = createFileRoute('/_app/_workspace/_shell/billing/plans')({
  head: () => ({ meta: [{ title: 'Plans · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { access, can } = useAccess(workspaceId)
  return (
    <PlanPicker
      workspaceId={workspaceId}
      // The current plan and change previews need billing.view; until access loads, show the page.
      canView={!access || can('billing.view')}
      canPay={can('billing.pay')}
    />
  )
}
