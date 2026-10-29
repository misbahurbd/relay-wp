import { createFileRoute } from '@tanstack/react-router'
import {
  ContactsPage,
  contactsListQueryOptions,
  contactsSearchSchema,
  contactsSummaryQueryOptions,
} from '#/features/contacts'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/contacts/')({
  validateSearch: contactsSearchSchema,
  loaderDeps: ({ search }) => search,
  // Prefetch without throwing: the page shows its own error state (e.g. no contacts.view).
  loader: ({ context, params, deps: { cursor, ...filter } }) =>
    Promise.all([
      context.queryClient.prefetchQuery(
        contactsListQueryOptions(params.workspaceId, filter, cursor),
      ),
      context.queryClient.prefetchQuery(contactsSummaryQueryOptions(params.workspaceId)),
    ]),
  head: () => ({ meta: [{ title: 'Contacts · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useParams()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <ContactsPage
      workspaceId={workspaceId}
      search={search}
      onSearchChange={(next) => navigate({ search: next, replace: true })}
    />
  )
}
