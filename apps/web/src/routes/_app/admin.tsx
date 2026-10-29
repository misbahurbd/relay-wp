import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { AdminPage } from '#/features/admin'

const search = z.object({
  tab: z.enum(['plans', 'discounts', 'workspaces']).optional().catch(undefined),
})

// Staff tools. Any signed-in account reaches the page; the API admits only ADMIN_EMAILS.
export const Route = createFileRoute('/_app/admin')({
  validateSearch: search,
  head: () => ({ meta: [{ title: 'Admin · Relay' }] }),
  component: Page,
})

function Page() {
  const { tab = 'plans' } = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <AdminPage
      tab={tab}
      onTab={(t) => navigate({ search: { tab: t === 'plans' ? undefined : t }, replace: true })}
    />
  )
}
