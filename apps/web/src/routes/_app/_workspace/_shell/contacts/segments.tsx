import { createFileRoute } from '@tanstack/react-router'
import { SegmentsPage, segmentsQueryOptions, segmentsSearchSchema } from '#/features/contacts'

export const Route = createFileRoute('/_app/_workspace/_shell/contacts/segments')({
  validateSearch: segmentsSearchSchema,
  loader: ({ context }) =>
    context.queryClient.prefetchQuery(segmentsQueryOptions(context.workspaceId)),
  head: () => ({ meta: [{ title: 'Segments · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useRouteContext()
  const { new: startNew } = Route.useSearch()
  return <SegmentsPage workspaceId={workspaceId} startNew={startNew} />
}
