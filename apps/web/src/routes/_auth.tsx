import { createFileRoute, Outlet, redirect } from '@tanstack/react-router'
import { fetchMe, safeRedirect } from '#/features/auth'

// Signed-out pages. A verified, signed-in user is sent on: to ?redirect= (as after
// a social sign-in that started from a deep link), or home.
export const Route = createFileRoute('/_auth')({
  ssr: false,
  beforeLoad: async ({ context, location }) => {
    const me = await fetchMe(context.queryClient)
    if (me?.user.email_verified) {
      throw redirect({
        to: safeRedirect((location.search as { redirect?: unknown }).redirect) ?? '/',
      })
    }
  },
  component: Outlet,
})
