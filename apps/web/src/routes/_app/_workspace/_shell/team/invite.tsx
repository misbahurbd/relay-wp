import { createFileRoute } from '@tanstack/react-router'
import { InviteWizard } from '#/features/team'

export const Route = createFileRoute('/_app/_workspace/_shell/team/invite')({
  head: () => ({ meta: [{ title: 'Invite teammates · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  return <InviteWizard workspaceId={workspaceId} />
}
