import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { resolveWorkspace, selectWorkspace } from '#/features/workspaces'

// Every page that works in one workspace. The workspace isn't in the URL: it's the one this tab
// picked in the workspace switcher. Child routes get it as context.workspaceId.
export const Route = createFileRoute('/_app/_workspace')({
  beforeLoad: async ({ context, location }) => {
    // Email links name their workspace once (?workspace=…): switch to it, then drop it from the URL.
    const search = new URLSearchParams(location.searchStr)
    const requested = search.get('workspace')
    if (requested) {
      selectWorkspace(requested)
      search.delete('workspace')
      const rest = search.toString()
      throw redirect({ href: location.pathname + (rest ? `?${rest}` : ''), replace: true })
    }
    const ws = await resolveWorkspace(context.queryClient, context.me.user.id)
    if (!ws) throw redirect({ to: '/welcome' })
    selectWorkspace(ws.id)
    return { workspaceId: ws.id }
  },
  component: Outlet,
})
