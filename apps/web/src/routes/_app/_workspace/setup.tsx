import { useSuspenseQuery } from '@tanstack/react-query'
import { createFileRoute, redirect } from '@tanstack/react-router'
import { z } from 'zod'
import { onboardingQueryOptions, SetupScreen } from '#/features/onboarding'
import { workspaceQueryOptions } from '#/features/workspaces'

export const Route = createFileRoute('/_app/_workspace/setup')({
  validateSearch: z.object({
    step: z.enum(['business', 'whatsapp', 'test_message']).optional().catch(undefined),
  }),
  head: () => ({ meta: [{ title: 'Set up your workspace · Relay' }] }),
  loader: async ({ context }) => {
    const userId = context.me.user.id
    const [onboarding] = await Promise.all([
      context.queryClient.fetchQuery(onboardingQueryOptions(userId, context.workspaceId)),
      context.queryClient.ensureQueryData(workspaceQueryOptions(userId, context.workspaceId)),
    ])
    if (onboarding.current_step === 'done') {
      throw redirect({ to: '/' })
    }
  },
  component: SetupPage,
})

function SetupPage() {
  const { me, workspaceId } = Route.useRouteContext()
  const { step } = Route.useSearch()
  const { data: workspace } = useSuspenseQuery(workspaceQueryOptions(me.user.id, workspaceId))
  const { data: onboarding } = useSuspenseQuery(onboardingQueryOptions(me.user.id, workspaceId))
  return (
    <SetupScreen
      userId={me.user.id}
      firstName={me.profile.first_name}
      workspace={workspace}
      onboarding={onboarding}
      initialStep={step}
    />
  )
}
