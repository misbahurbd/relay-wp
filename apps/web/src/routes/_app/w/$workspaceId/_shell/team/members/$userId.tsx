import { createFileRoute } from '@tanstack/react-router'
import { MemberPage } from '#/features/team'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/team/members/$userId')({
  head: () => ({ meta: [{ title: 'Member · Relay' }] }),
  component: Page,
})

function Page() {
  const { me } = Route.useRouteContext()
  const { workspaceId, userId } = Route.useParams()
  return <MemberPage workspaceId={workspaceId} userId={userId} meId={me.user.id} />
}
