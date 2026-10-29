import { createFileRoute } from '@tanstack/react-router'
import { RolesPage } from '#/features/team'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/team/roles')({
  head: () => ({ meta: [{ title: 'Roles and permissions · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useParams()
  return <RolesPage workspaceId={workspaceId} />
}
