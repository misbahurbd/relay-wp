import { createFileRoute } from '@tanstack/react-router'
import { AutomationsPage } from '#/features/automations'
import { useAccess } from '#/features/team'

export const Route = createFileRoute('/_app/_workspace/_shell/automations/')({
  head: () => ({ meta: [{ title: 'Automations · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { can } = useAccess(workspaceId)
  return (
    <AutomationsPage
      workspaceId={workspaceId}
      canEdit={can('automations.edit')}
      canPublish={can('automations.publish')}
    />
  )
}
