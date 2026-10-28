import { createFileRoute } from '@tanstack/react-router'
import { AuthLayout, SignupForm } from '#/features/auth'

export const Route = createFileRoute('/_auth/signup')({
  head: () => ({ meta: [{ title: 'Create your account · Relay' }] }),
  component: SignupPage,
})

function SignupPage() {
  return (
    <AuthLayout
      title="Create your account"
      description="Start your 14-day free trial. No credit card needed."
    >
      <SignupForm />
    </AuthLayout>
  )
}
