import { createFileRoute } from '@tanstack/react-router'
import { BroadcastsPage, broadcastSearchSchema } from '#/features/broadcasts'

export const Route = createFileRoute('/_app/_workspace/_shell/broadcasts/')({
  validateSearch: broadcastSearchSchema,
  head: () => ({ meta: [{ title: 'Broadcasts · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const filter = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <BroadcastsPage
      workspaceId={workspaceId}
      filter={filter}
      onFilter={(next) => navigate({ search: next, replace: true })}
    />
  )
}
