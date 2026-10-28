import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { fetchMe } from '#/features/auth'

// Everything behind sign-in. Child routes get the signed-in user as context.me.
export const Route = createFileRoute('/_app')({
  ssr: false,
  beforeLoad: async ({ context, location }) => {
    const me = await fetchMe(context.queryClient)
    if (!me) throw redirect({ to: '/login', search: { redirect: location.href } })
    if (!me.user.email_verified) {
      throw redirect({ to: '/verify-email', search: { email: me.user.email } })
    }
    return { me }
  },
  component: Outlet,
})
