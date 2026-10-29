import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { TemplateEditor } from '#/features/templates'
import { workspaceQueryOptions } from '#/features/workspaces'

export const Route = createFileRoute('/_app/_workspace/_shell/templates/new')({
  head: () => ({ meta: [{ title: 'Create template · Relay' }] }),
  component: Page,
})

function Page() {
  const { me, workspaceId } = Route.useRouteContext()
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(me.user.id, workspaceId))
  return <TemplateEditor workspaceId={workspaceId} businessName={workspace.name} />
}
