import { createFileRoute } from '@tanstack/react-router'
import { BroadcastDetail } from '#/features/broadcasts'

export const Route = createFileRoute('/_app/_workspace/_shell/broadcasts/$broadcastId')({
  head: () => ({ meta: [{ title: 'Broadcast · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { broadcastId } = Route.useParams()
  return <BroadcastDetail key={broadcastId} workspaceId={workspaceId} broadcastId={broadcastId} />
}
