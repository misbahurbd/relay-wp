import { useQuery, useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import {
  FirstRunDashboard,
  numbersQueryOptions,
  onboardingQueryOptions,
} from '#/features/onboarding'
import { workspaceQueryOptions } from '#/features/workspaces'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/')({
  head: () => ({ meta: [{ title: 'Dashboard · Relay' }] }),
  loader: async ({ context, params }) => {
    const onboarding = await context.queryClient.fetchQuery(
      onboardingQueryOptions(context.me.user.id, params.workspaceId),
    )
    if (!onboarding.completed) throw redirect({ to: '/w/$workspaceId/setup', params })
  },
  component: DashboardPage,
})

function DashboardPage() {
  const { me } = Route.useRouteContext()
  const { workspaceId } = Route.useParams()
  const userId = me.user.id
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(userId, workspaceId))
  const { data: onboarding } = useSuspenseQuery(onboardingQueryOptions(userId, workspaceId))
  const { data: numbers } = useQuery(numbersQueryOptions(userId, workspaceId))
  // The everyday dashboard (activated workspaces) is not built yet; first-run covers both.
  return (
    <FirstRunDashboard
      userId={userId}
      firstName={me.profile.first_name}
      workspace={workspace}
      onboarding={onboarding}
      number={numbers?.find((n) => n.status === 'connected')}
    />
  )
}
