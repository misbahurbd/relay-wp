import { createFileRoute } from '@tanstack/react-router'
import { BroadcastEditor, newBroadcastSearchSchema } from '#/features/broadcasts'

export const Route = createFileRoute('/_app/_workspace/_shell/broadcasts/new')({
  validateSearch: newBroadcastSearchSchema,
  head: () => ({ meta: [{ title: 'New broadcast · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { tag, segment, template } = Route.useSearch()
  return (
    <BroadcastEditor
      workspaceId={workspaceId}
      tagIds={tag ? [tag] : undefined}
      segmentIds={segment ? [segment] : undefined}
      templateId={template}
    />
  )
}
