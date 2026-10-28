import { revalidateLogic, useForm } from '@tanstack/react-form'
import { Link, useNavigate } from '@tanstack/react-router'
import { useRef } from 'react'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { errorMessage, isApiError } from '#/lib/api'
import { rememberSignupPassword, useLogin } from '../api/auth'
import { loginSchema } from '../schemas/auth'
import { PasswordInput } from './password-input'
import { SocialButtons } from './social-buttons'

const OAUTH_ERRORS: Record<string, string> = {
  email_in_use_sign_in_first:
    'An account with that email was just created. Log in with your password, then connect the provider from Settings.',
  email_not_verified:
    'Confirm your email address first: open the link we emailed you, or ask for a new one on the verify page.',
  oauth_cancelled: 'Sign-in was cancelled. Try again when you’re ready.',
  invalid_oauth_state:
    'That sign-in took too long or was opened in another browser. Start it again from this page.',
  provider_not_available: 'That sign-in method isn’t available right now. Use your email instead.',
}

type Props = {
  email?: string
  /** Error code from a social sign-in redirect. */
  oauthError?: string
  /** Where to go after signing in; the home route decides when absent. */
  redirect?: string
}

export function LoginForm({ email = '', oauthError, redirect }: Props) {
  const navigate = useNavigate()
  const login = useLogin()
  const formRef = useRef<HTMLFormElement>(null)

  const form = useForm({
    defaultValues: { email, password: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      const body = { email: value.email.trim(), password: value.password, client: 'web' as const }
      const ok = await login.mutateAsync({ body }).then(
        () => true,
        (error) => {
          if (isApiError(error) && error.code === 'email_not_verified') {
            rememberSignupPassword(body.email, body.password)
            navigate({ to: '/verify-email', search: { email: body.email } })
          }
          return false
        },
      )
      if (ok) await navigate({ to: redirect ?? '/' })
    },
  })

  const loginError =
    login.isError && !(isApiError(login.error) && login.error.code === 'email_not_verified')
      ? errorMessage(login.error)
      : null
  const banner =
    loginError ??
    (oauthError ? (OAUTH_ERRORS[oauthError] ?? 'Sign-in didn’t work. Please try again.') : null)

  return (
    <div className="flex flex-col gap-5">
      {banner && (
        <Alert variant="danger" aria-live="polite">
          <AlertDescription>{banner}</AlertDescription>
        </Alert>
      )}
      <SocialButtons action="log in" redirect={redirect} />
      <form
        ref={formRef}
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          form.handleSubmit()
        }}
      >
        <form.Field name="email" validators={{ onDynamic: loginSchema.shape.email }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField id="email" label="Work email" error={error}>
                <Input
                  id="email"
                  type="email"
                  autoComplete="email"
                  inputMode="email"
                  placeholder="you@yourbusiness.com"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('email', { error })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="password" validators={{ onDynamic: loginSchema.shape.password }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField
                id="password"
                label="Password"
                error={error}
                labelAside={
                  <Link
                    to="/forgot-password"
                    search={{ email: form.state.values.email || undefined }}
                    className="text-sm font-bold"
                  >
                    Forgot password?
                  </Link>
                }
              >
                <PasswordInput
                  id="password"
                  autoComplete="current-password"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('password', { error })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Subscribe selector={(s) => s.isSubmitting}>
          {(submitting) => (
            <Button type="submit" size="lg" className="w-full" disabled={submitting}>
              {submitting ? 'Logging in…' : 'Log in'}
            </Button>
          )}
        </form.Subscribe>
        <p className="rounded-lg bg-muted px-3.5 py-2.5 text-sm text-muted-foreground">
          Didn’t finish setting up? We’ll take you back to the step you left, with everything you
          entered saved.
        </p>
      </form>
      <p className="text-center text-sm text-muted-foreground">
        New to Relay?{' '}
        <Link to="/signup" className="font-bold">
          Create an account
        </Link>
      </p>
    </div>
  )
}
