import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, Outlet, useNavigate } from '@tanstack/react-router'
import { AppShell } from '#/components/layout/app-shell'
import { useLogout } from '#/features/auth'
import { rememberWorkspace, trialDaysLeft, workspaceQueryOptions } from '#/features/workspaces'

// The signed-in app frame (sidebar and top bar) around every workspace page.
export const Route = createFileRoute('/_app/w/$workspaceId/_shell')({
  loader: async ({ context, params }) => {
    await context.queryClient.ensureQueryData(
      workspaceQueryOptions(context.me.user.id, params.workspaceId),
    )
    rememberWorkspace(params.workspaceId)
  },
  component: ShellLayout,
})

function ShellLayout() {
  const { me } = Route.useRouteContext()
  const { workspaceId } = Route.useParams()
  const navigate = useNavigate()
  const logout = useLogout()
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(me.user.id, workspaceId))
  const name =
    me.profile.display_name ??
    [me.profile.first_name, me.profile.last_name].filter(Boolean).join(' ')
  const daysLeft = workspace.activated_at ? null : trialDaysLeft(workspace.trial_ends_at)

  return (
    <AppShell
      workspace={{ id: workspace.id, name: workspace.name, trialDaysLeft: daysLeft }}
      user={{ name, email: me.user.email, avatarUrl: me.profile.avatar_url }}
      onLogout={() => logout.mutate({}, { onSettled: () => navigate({ to: '/login' }) })}
    >
      <Outlet />
    </AppShell>
  )
}
