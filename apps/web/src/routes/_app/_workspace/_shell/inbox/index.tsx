import { createFileRoute } from '@tanstack/react-router'
import { conversationsQueryOptions, InboxPage, inboxSearchSchema } from '#/features/inbox'

export const Route = createFileRoute('/_app/_workspace/_shell/inbox/')({
  validateSearch: inboxSearchSchema,
  loaderDeps: ({ search: { tab, q, sort } }) => ({ tab, q, sort }),
  // Prefetch without throwing: the page shows its own error state.
  loader: ({ context, deps }) =>
    context.queryClient.prefetchInfiniteQuery(
      conversationsQueryOptions(context.workspaceId, {
        tab: deps.tab ?? 'open',
        q: deps.q,
        sort: deps.sort ?? 'newest',
      }),
    ),
  head: () => ({ meta: [{ title: 'Inbox · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId, me } = Route.useRouteContext()
  const search = Route.useSearch()
  const navigate = Route.useNavigate()
  const name =
    me.profile.display_name ??
    ([me.profile.first_name, me.profile.last_name].filter(Boolean).join(' ') || me.user.email)
  return (
    <InboxPage
      workspaceId={workspaceId}
      me={{ id: me.user.id, name }}
      search={search}
      // Opening a chat is a step Back can undo; filter changes replace the entry.
      onSearchChange={(next) => navigate({ search: next, replace: next.c === search.c })}
    />
  )
}
