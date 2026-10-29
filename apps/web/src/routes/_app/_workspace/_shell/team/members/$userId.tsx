import { createFileRoute } from '@tanstack/react-router'
import { MemberPage } from '#/features/team'

export const Route = createFileRoute('/_app/_workspace/_shell/team/members/$userId')({
  head: () => ({ meta: [{ title: 'Member · Relay' }] }),
  component: Page,
})

function Page() {
  const { me, workspaceId } = Route.useRouteContext()
  const { userId } = Route.useParams()
  return <MemberPage workspaceId={workspaceId} userId={userId} meId={me.user.id} />
}
