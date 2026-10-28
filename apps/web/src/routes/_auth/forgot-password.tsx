import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { AuthLayout, ForgotPasswordForm } from '#/features/auth'

export const Route = createFileRoute('/_auth/forgot-password')({
  validateSearch: z.object({ email: z.string().optional() }),
  head: () => ({ meta: [{ title: 'Reset your password · Relay' }] }),
  component: ForgotPasswordPage,
})

function ForgotPasswordPage() {
  const { email } = Route.useSearch()
  return (
    <AuthLayout
      title="Reset your password"
      description="Enter the email you signed up with. We’ll send you a link to choose a new password."
      panel="login"
    >
      <ForgotPasswordForm email={email} />
    </AuthLayout>
  )
}
