import { createFileRoute } from '@tanstack/react-router'
import { PlanPicker } from '#/features/billing'
import { useAccess } from '#/features/team'

export const Route = createFileRoute('/_app/_workspace/_shell/billing/plans')({
  head: () => ({ meta: [{ title: 'Plans · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { can } = useAccess(workspaceId)
  return <PlanPicker workspaceId={workspaceId} canPay={can('billing.pay')} />
}
