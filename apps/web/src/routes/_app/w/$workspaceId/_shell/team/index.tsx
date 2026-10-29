import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { TeamPage } from '#/features/team'

const search = z.object({
  tab: z.enum(['members', 'teams', 'roles']).optional().catch(undefined),
})

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/team/')({
  validateSearch: search,
  head: () => ({ meta: [{ title: 'Team · Relay' }] }),
  component: Page,
})

function Page() {
  const { me } = Route.useRouteContext()
  const { workspaceId } = Route.useParams()
  const { tab = 'members' } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <TeamPage
      workspaceId={workspaceId}
      meId={me.user.id}
      tab={tab}
      onTab={(t) => navigate({ search: { tab: t === 'members' ? undefined : t }, replace: true })}
    />
  )
}
