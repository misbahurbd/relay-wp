import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import {
  AuthLayout,
  ResendVerification,
  useOneTimeParam,
  VerifyEmailCode,
  VerifyEmailLink,
} from '#/features/auth'

export const Route = createFileRoute('/_auth/verify-email')({
  // Only a real address is shown back ("we sent a code to …"), so a link can't put words on the page.
  validateSearch: z.object({
    email: z.email().optional().catch(undefined),
    token: z.string().optional(),
  }),
  head: () => ({ meta: [{ title: 'Verify your email · Relay' }] }),
  component: VerifyEmailPage,
})

function VerifyEmailPage() {
  const { email, token: fromUrl } = Route.useSearch()
  const token = useOneTimeParam('token', fromUrl)
  if (token) {
    return (
      <AuthLayout title="Verify your email" panel="none">
        <VerifyEmailLink token={token} />
      </AuthLayout>
    )
  }
  // Renders its own layout: the heading changes once the code is accepted.
  if (email) return <VerifyEmailCode email={email} />
  return (
    <AuthLayout
      title="Verify your email"
      description="Enter the email you signed up with and we’ll send a new code."
      panel="none"
    >
      <ResendVerification />
    </AuthLayout>
  )
}
