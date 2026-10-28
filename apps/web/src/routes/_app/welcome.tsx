import { createFileRoute, redirect } from '@tanstack/react-router'
import { WelcomeScreen } from '#/features/onboarding'
import { resolveHomePath } from '#/features/workspaces'

export const Route = createFileRoute('/_app/welcome')({
  head: () => ({ meta: [{ title: 'Welcome · Relay' }] }),
  // One workspace per person for now: anyone who has one goes back to it.
  beforeLoad: async ({ context }) => {
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
