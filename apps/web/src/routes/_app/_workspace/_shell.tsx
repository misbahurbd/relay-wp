import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet, useNavigate, useRouter } from '@tanstack/react-router'
import { AppShell } from '#/components/layout/app-shell'
import { useLogout } from '#/features/auth'
import { useAccessVersionWatcher } from '#/features/team'
import {
  selectWorkspace,
  trialDaysLeft,
  workspaceQueryOptions,
  workspacesQueryOptions,
} from '#/features/workspaces'

// The signed-in app frame (sidebar and top bar) around every workspace page.
export const Route = createFileRoute('/_app/_workspace/_shell')({
  loader: async ({ context }) => {
    await context.queryClient.ensureQueryData(
      workspaceQueryOptions(context.me.user.id, context.workspaceId),
    )
  },
  component: ShellLayout,
})

function ShellLayout() {
  const { me, workspaceId } = Route.useRouteContext()
  const navigate = useNavigate()
  const router = useRouter()
  const logout = useLogout()
  useAccessVersionWatcher()
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(me.user.id, workspaceId))
  const name =
    me.profile.display_name ??
    [me.profile.first_name, me.profile.last_name].filter(Boolean).join(' ')
  const { data: mine = [] } = useQuery(workspacesQueryOptions(me.user.id))
  const roleName = mine.find((w) => w.id === workspaceId)?.role_name
  const daysLeft = workspace.activated_at ? null : trialDaysLeft(workspace.trial_ends_at)

  // Pages belong to one workspace, so switching starts again from its dashboard (or setup).
  const switchWorkspace = async (id: string) => {
    if (id === workspaceId) return
    selectWorkspace(id)
    await navigate({ to: '/' })
    await router.invalidate()
  }

  return (
    <AppShell
      workspace={{ id: workspace.id, name: workspace.name, trialDaysLeft: daysLeft }}
      workspaces={mine.map((w) => ({ id: w.id, name: w.name, roleName: w.role_name }))}
      onSwitchWorkspace={switchWorkspace}
      user={{ name, email: me.user.email, avatarUrl: me.profile.avatar_url, roleName }}
      onLogout={() => logout.mutate({}, { onSettled: () => navigate({ to: '/login' }) })}
    >
      <Outlet />
    </AppShell>
  )
}
