import { revalidateLogic, useForm } from '@tanstack/react-form'
import { Link, useNavigate } from '@tanstack/react-router'
import { CircleCheckIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { InputOTP, InputOTPGroup, InputOTPSlot } from '#/components/ui/input-otp'
import { errorMessage, isApiError } from '#/lib/api'
import {
  useForgotPassword,
  useOauthComplete,
  useOauthCompleteVerify,
  useResetPassword,
} from '../../api/auth.api'
import { emailSchema, resetPasswordSchema } from '../../schemas/auth.schema'
import { PasswordInput, PasswordRules } from '../kit/password-input'

export function ForgotPasswordForm({ email = '' }: { email?: string }) {
  const forgot = useForgotPassword()
  const formRef = useRef<HTMLFormElement>(null)
  const form = useForm({
    defaultValues: { email },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: ({ value }) =>
      forgot.mutateAsync({ body: { email: value.email.trim() } }).catch(() => undefined),
  })

  if (forgot.isSuccess) {
    return (
      <div className="flex flex-col gap-5" role="status">
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>Check your email</AlertTitle>
          <AlertDescription>
            If {form.state.values.email.trim()} has an account, we’ve sent a link to choose a new
            password. It works for 1 hour; check your spam folder if you don’t see it.
          </AlertDescription>
        </Alert>
        <Button variant="outline" asChild>
          <Link to="/login">Back to log in</Link>
        </Button>
      </div>
    )
  }

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        form.handleSubmit()
      }}
    >
      {forgot.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(forgot.error)}</AlertDescription>
        </Alert>
      )}
      <form.Field name="email" validators={{ onDynamic: emailSchema.shape.email }}>
        {(field) => {
          const error = fieldError(field)
          return (
            <FormField id="email" label="Work email" error={error}>
              <Input
                id="email"
                type="email"
                autoComplete="email"
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
      <form.Subscribe selector={(s) => s.isSubmitting}>
        {(submitting) => (
          <Button type="submit" size="lg" disabled={submitting}>
            {submitting ? 'Sending…' : 'Send reset link'}
          </Button>
        )}
      </form.Subscribe>
      <Button variant="ghost" asChild>
        <Link to="/login">Back to log in</Link>
      </Button>
    </form>
  )
}

export function ResetPasswordForm({ token }: { token: string }) {
  const reset = useResetPassword()
  const navigate = useNavigate()
  const formRef = useRef<HTMLFormElement>(null)
  const form = useForm({
    defaultValues: { password: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: ({ value }) =>
      reset.mutateAsync({ body: { token, new_password: value.password } }).catch(() => undefined),
  })

  if (reset.isSuccess) {
    return (
      <div className="flex flex-col gap-5" role="status">
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>Password changed</AlertTitle>
          <AlertDescription>
            We signed you out everywhere. Log in with your new password.
          </AlertDescription>
        </Alert>
        <Button size="lg" onClick={() => navigate({ to: '/login' })}>
          Log in
        </Button>
      </div>
    )
  }

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        form.handleSubmit()
      }}
    >
      {reset.isError && (
        <Alert variant="danger">
          <AlertDescription>
            <p>
              {errorMessage(reset.error)}{' '}
              <Link to="/forgot-password" className="font-bold">
                Get a new link
              </Link>
            </p>
          </AlertDescription>
        </Alert>
      )}
      <form.Field name="password" validators={{ onDynamic: resetPasswordSchema.shape.password }}>
        {(field) => {
          const error = fieldError(field)
          return (
            <FormField
              id="password"
              label="New password"
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
      <form.Subscribe selector={(s) => s.isSubmitting}>
        {(submitting) => (
          <Button type="submit" size="lg" disabled={submitting}>
            {submitting ? 'Saving…' : 'Set new password'}
          </Button>
        )}
      </form.Subscribe>
    </form>
  )
}

/**
 * A social sign-in whose provider doesn't vouch for the email (Facebook) proves it here:
 * the person confirms an address (prefilled with the provider's), then types the code
 * we email to it. Only then does the account exist.
 */
export function AddEmailForm({
  completeToken,
  email: suggested = '',
  redirect,
}: {
  completeToken: string
  email?: string
  redirect?: string
}) {
  const complete = useOauthComplete()
  const [sentTo, setSentTo] = useState<string | null>(null)
  const formRef = useRef<HTMLFormElement>(null)
  const form = useForm({
    defaultValues: { email: suggested },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      const email = value.email.trim()
      await complete
        .mutateAsync({ body: { token: completeToken, email } })
        .then(() => setSentTo(email))
        .catch(() => undefined)
    },
  })

  if (sentTo) {
    return (
      <AddEmailCode
        completeToken={completeToken}
        email={sentTo}
        redirect={redirect}
        onChangeEmail={() => {
          complete.reset()
          setSentTo(null)
        }}
      />
    )
  }

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        form.handleSubmit()
      }}
    >
      <p className="text-[15px] text-muted-foreground">
        Confirm the email address you use for work. We’ll send it a code, so only its owner can
        finish signing in.
      </p>
      {complete.isError && (
        <Alert variant="danger">
          <AlertDescription>
            <p>
              {errorMessage(complete.error)}{' '}
              <Link to="/login" className="font-bold">
                Start again
              </Link>
            </p>
          </AlertDescription>
        </Alert>
      )}
      <form.Field name="email" validators={{ onDynamic: emailSchema.shape.email }}>
        {(field) => {
          const error = fieldError(field)
          return (
            <FormField id="email" label="Work email" error={error}>
              <Input
                id="email"
                type="email"
                autoComplete="email"
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
      <form.Subscribe selector={(s) => s.isSubmitting}>
        {(submitting) => (
          <Button type="submit" size="lg" disabled={submitting}>
            {submitting ? 'Sending…' : 'Send code'}
          </Button>
        )}
      </form.Subscribe>
    </form>
  )
}

function AddEmailCode({
  completeToken,
  email,
  redirect,
  onChangeEmail,
}: {
  completeToken: string
  email: string
  redirect?: string
  onChangeEmail: () => void
}) {
  const verify = useOauthCompleteVerify()
  const resend = useOauthComplete()
  const navigate = useNavigate()
  const [code, setCode] = useState('')

  const submit = (value: string) => {
    if (value.length !== 6) return
    verify.mutate(
      { body: { token: completeToken, code: value } },
      { onSuccess: () => navigate({ to: redirect ?? '/' }) },
    )
  }
  const wrong = verify.isError && code.length === 6
  const flowOver =
    isApiError(verify.error) &&
    ['too_many_attempts', 'invalid_oauth_state'].includes(verify.error.code)

  return (
    <form
      className="flex flex-col gap-3"
      onSubmit={(e) => {
        e.preventDefault()
        submit(code)
      }}
    >
      <p className="text-[15px] text-muted-foreground">
        We sent a 6-digit code to <strong className="text-foreground">{email}</strong>.{' '}
        <button type="button" className="font-bold text-foreground" onClick={onChangeEmail}>
          Change email
        </button>
      </p>
      <label htmlFor="add-email-code" className="font-bold">
        6-digit code
      </label>
      <InputOTP
        id="add-email-code"
        maxLength={6}
        value={code}
        onChange={(v) => {
          setCode(v)
          if (verify.isError && !flowOver) verify.reset()
        }}
        onComplete={submit}
        aria-invalid={wrong}
        aria-describedby={wrong ? 'add-email-code-error' : 'add-email-code-hint'}
        disabled={flowOver}
        autoFocus
      >
        <InputOTPGroup>
          {[0, 1, 2, 3, 4, 5].map((i) => (
            <InputOTPSlot key={i} index={i} aria-invalid={wrong} />
          ))}
        </InputOTPGroup>
      </InputOTP>
      {wrong ? (
        <p id="add-email-code-error" role="alert" className="text-sm text-destructive">
          {errorMessage(verify.error)}{' '}
          {flowOver && (
            <Link to="/login" className="font-bold">
              Start again
            </Link>
          )}
        </p>
      ) : (
        <p id="add-email-code-hint" className="text-sm text-muted-foreground">
          The code expires in 10 minutes.
        </p>
      )}
      <Button type="submit" size="lg" disabled={code.length < 6 || verify.isPending || flowOver}>
        {verify.isPending ? 'Checking…' : 'Continue'}
      </Button>
      <p className="text-sm" aria-live="polite">
        {resend.isSuccess ? (
          <span className="text-brand-text">New code sent. Only the latest code works.</span>
        ) : resend.isError ? (
          <span className="text-destructive">{errorMessage(resend.error)}</span>
        ) : (
          <button
            type="button"
            className="font-bold text-brand-text underline-offset-4 hover:underline"
            disabled={resend.isPending}
            onClick={() => resend.mutate({ body: { token: completeToken, email } })}
          >
            Send a new code
          </button>
        )}
      </p>
    </form>
  )
}
