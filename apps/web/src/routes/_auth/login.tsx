import { createFileRoute } from '@tanstack/react-router'
import { z } from 'zod'
import { AddEmailForm, AuthLayout, LoginForm, useOneTimeParam } from '#/features/auth'

const search = z.object({
  email: z.email().optional().catch(undefined),
  // Set by the API after a social sign-in.
  error: z.string().optional(),
  complete_token: z.string().optional(),
  // Same-site paths only, so a link can't bounce people to another site after login.
  redirect: z
    .string()
    .refine((v) => /^\/(?![/\\])/.test(v))
    .optional()
    .catch(undefined),
})

export const Route = createFileRoute('/_auth/login')({
  validateSearch: search,
  head: () => ({ meta: [{ title: 'Log in · Relay' }] }),
  component: LoginPage,
})

function LoginPage() {
  const { email, error, complete_token, redirect } = Route.useSearch()
  const completeToken = useOneTimeParam('complete_token', complete_token)
  if (completeToken) {
    return (
      <AuthLayout title="Confirm your email" panel="login">
        <AddEmailForm completeToken={completeToken} email={email} redirect={redirect} />
      </AuthLayout>
    )
  }
  return (
    <AuthLayout
      title="Welcome back"
      description="Log in to reply to your customers and check your broadcasts."
      panel="login"
    >
      <LoginForm email={email} oauthError={error} redirect={redirect} />
    </AuthLayout>
  )
}
