import { createFileRoute, Link } from '@tanstack/react-router'
import { z } from 'zod'
import { AuthLayout, ResetPasswordForm, useOneTimeParam } from '#/features/auth'

export const Route = createFileRoute('/_auth/reset-password')({
  validateSearch: z.object({ token: z.string().optional() }),
  head: () => ({ meta: [{ title: 'Choose a new password · Relay' }] }),
  component: ResetPasswordPage,
})

function ResetPasswordPage() {
  const token = useOneTimeParam('token', Route.useSearch().token)
  return (
    <AuthLayout title="Choose a new password" panel="login">
      {token ? (
        <ResetPasswordForm token={token} />
      ) : (
        <p className="text-muted-foreground">
          This page needs the link from your email.{' '}
          <Link to="/forgot-password" className="font-bold">
            Get a new link
          </Link>
        </p>
      )}
    </AuthLayout>
  )
}
