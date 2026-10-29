import { createFileRoute, redirect } from '@tanstack/react-router'
import { WelcomeScreen } from '#/features/onboarding'
import { pendingInviteToken } from '#/features/team'
import { resolveHomePath } from '#/features/workspaces'

export const Route = createFileRoute('/_app/welcome')({
  head: () => ({ meta: [{ title: 'Welcome · Relay' }] }),
  // One workspace per person for now: anyone who has one goes back to it.
  beforeLoad: async ({ context }) => {
    // Someone who opened an invite joins that workspace instead of making their own.
    if (pendingInviteToken()) throw redirect({ to: '/invite' })
    const home = await resolveHomePath(context.queryClient, context.me.user.id)
    if (home.to !== '/welcome') throw redirect(home)
  },
  component: WelcomePage,
})

function WelcomePage() {
  const { me } = Route.useRouteContext()
  return (
    <WelcomeScreen userId={me.user.id} firstName={me.profile.first_name} email={me.user.email} />
  )
}
