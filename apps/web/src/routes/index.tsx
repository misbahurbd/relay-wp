import { createFileRoute, redirect } from '@tanstack/react-router'
import { fetchMe } from '#/features/auth'
import { resolveHomePath } from '#/features/workspaces'

// Sends everyone to the right place: log in, verify, Welcome, Setup or the dashboard.
export const Route = createFileRoute('/')({
  ssr: false,
  beforeLoad: async ({ context }) => {
    const me = await fetchMe(context.queryClient)
    if (!me) throw redirect({ to: '/login' })
    if (!me.user.email_verified) {
      throw redirect({ to: '/verify-email', search: { email: me.user.email } })
    }
    throw redirect(await resolveHomePath(context.queryClient, me.user.id))
  },
})
