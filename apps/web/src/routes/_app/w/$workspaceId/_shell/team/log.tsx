import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { AccessLog } from '#/features/team'

const search = z.object({ member: z.uuid().optional().catch(undefined) })

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/team/log')({
  validateSearch: search,
  head: () => ({ meta: [{ title: 'Access log · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useParams()
  const { member } = Route.useSearch()
  return <AccessLog key={member ?? ''} workspaceId={workspaceId} member={member} />
}
