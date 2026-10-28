import { revalidateLogic, useForm } from '@tanstack/react-form'
import { Link, useNavigate } from '@tanstack/react-router'
import { useRef } from 'react'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Checkbox } from '#/components/ui/checkbox'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { errorMessage } from '#/lib/api'
import { rememberSignupPassword, useSignup } from '../api/auth'
import { signupSchema, splitName } from '../schemas/auth'
import { PasswordInput, PasswordRules } from './password-input'
import { SocialButtons } from './social-buttons'

export function SignupForm() {
  const navigate = useNavigate()
  const signup = useSignup()
  const formRef = useRef<HTMLFormElement>(null)

  const form = useForm({
    defaultValues: { fullName: '', email: '', password: '', acceptTerms: false as boolean },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      const email = value.email.trim()
      const ok = await signup
        .mutateAsync({
          body: {
            ...splitName(value.fullName),
            email,
            password: value.password,
            accept_terms: value.acceptTerms,
            timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          },
        })
        .then(
          () => true,
          () => false, // shown by the alert above the form
        )
      if (ok) {
        rememberSignupPassword(email, value.password)
        await navigate({ to: '/verify-email', search: { email } })
      }
    },
  })

  return (
    <div className="flex flex-col gap-5">
      <SocialButtons action="sign up" />
      <form
        ref={formRef}
        noValidate
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          form.handleSubmit()
        }}
      >
        {signup.isError && (
          <Alert variant="danger" aria-live="polite">
            <AlertDescription>{errorMessage(signup.error)}</AlertDescription>
          </Alert>
        )}
        <form.Field name="fullName" validators={{ onDynamic: signupSchema.shape.fullName }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField id="fullName" label="Full name" error={error}>
                <Input
                  id="fullName"
                  autoComplete="name"
                  placeholder="e.g. Shakil Ahmed"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('fullName', { error })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="email" validators={{ onDynamic: signupSchema.shape.email }}>
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
        <form.Field name="password" validators={{ onDynamic: signupSchema.shape.password }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <FormField
                id="password"
                label="Password"
                error={error}
                description={<PasswordRules id="password-rules" value={field.state.value} />}
              >
                <PasswordInput
                  id="password"
                  autoComplete="new-password"
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('password', { error, hint: true })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="acceptTerms" validators={{ onDynamic: signupSchema.shape.acceptTerms }}>
          {(field) => {
            const error = fieldError(field)
            return (
              <div className="flex flex-col gap-1.5">
                <Label className="items-start gap-2.5 leading-snug font-normal">
                  <Checkbox
                    className="mt-0.5"
                    checked={field.state.value}
                    onCheckedChange={(v) => field.handleChange(v === true)}
                    onBlur={field.handleBlur}
                    aria-invalid={!!error}
                    aria-describedby={describedBy('acceptTerms', { error })}
                  />
                  <span>
                    I agree to the <a href="/terms">Terms of Service</a> and{' '}
                    <a href="/privacy">Privacy Policy</a>, and to the{' '}
                    <a href="https://business.whatsapp.com/policy" target="_blank" rel="noreferrer">
                      WhatsApp Business messaging policy
                    </a>
                    .
                  </span>
                </Label>
                {error && (
                  <p id="acceptTerms-error" role="alert" className="text-sm text-destructive">
                    {error}
                  </p>
                )}
              </div>
            )
          }}
        </form.Field>
        <form.Subscribe selector={(s) => s.isSubmitting}>
          {(submitting) => (
            <Button type="submit" size="lg" className="w-full" disabled={submitting}>
              {submitting ? 'Creating your account…' : 'Create account'}
            </Button>
          )}
        </form.Subscribe>
      </form>
      <p className="text-center text-sm text-muted-foreground">
        Already have an account?{' '}
        <Link to="/login" className="font-bold">
          Log in
        </Link>
      </p>
    </div>
  )
}
