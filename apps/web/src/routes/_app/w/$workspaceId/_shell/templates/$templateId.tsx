import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute } from '@tanstack/react-router'
import { TemplateEditor } from '#/features/templates'
import { workspaceQueryOptions } from '#/features/workspaces'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/templates/$templateId')({
  head: () => ({ meta: [{ title: 'Edit template · Relay' }] }),
  component: Page,
})

function Page() {
  const { me } = Route.useRouteContext()
  const { workspaceId, templateId } = Route.useParams()
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(me.user.id, workspaceId))
  return (
    <TemplateEditor
      key={templateId}
      workspaceId={workspaceId}
      templateId={templateId}
      businessName={workspace.name}
    />
  )
}
