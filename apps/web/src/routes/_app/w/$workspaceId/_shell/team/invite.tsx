import { createFileRoute } from '@tanstack/react-router'
import { InviteWizard } from '#/features/team'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/team/invite')({
  head: () => ({ meta: [{ title: 'Invite teammates · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useParams()
  return <InviteWizard workspaceId={workspaceId} />
}
