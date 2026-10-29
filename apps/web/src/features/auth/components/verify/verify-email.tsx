import { revalidateLogic, useForm } from '@tanstack/react-form'
import { Link } from '@tanstack/react-router'
import { CircleCheckIcon, MailIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { InputOTP, InputOTPGroup, InputOTPSlot } from '#/components/ui/input-otp'
import { errorMessage, isApiError } from '#/lib/api'
import {
  forgetSignupPassword,
  rememberedSignupPassword,
  useResendVerification,
  useVerifyEmailCode,
  useVerifyEmailLink,
} from '../../api/auth.api'
import { emailSchema } from '../../schemas/auth.schema'
import { AuthLayout } from '../kit/auth-layout'
import { PasswordInput } from '../kit/password-input'

const RESEND_SECONDS = 60

function useCountdown(seconds: number) {
  const [left, setLeft] = useState(seconds)
  useEffect(() => {
    if (left <= 0) return
    const t = setTimeout(() => setLeft((s) => s - 1), 1000)
    return () => clearTimeout(t)
  }, [left])
  return { left, restart: () => setLeft(seconds) }
}

function formatSeconds(s: number) {
  return `${Math.floor(s / 60)}:${String(s % 60).padStart(2, '0')}`
}

/**
 * Code entry after sign-up. The 6 boxes are one native input: paste and SMS/email autofill work.
 * The code goes with the sign-up password, remembered from the page before; after a reload (or a
 * wrong one) the page asks for it.
 */
export function VerifyEmailCode({ email }: { email: string }) {
  const verify = useVerifyEmailCode()
  const resend = useResendVerification()
  const { left, restart } = useCountdown(RESEND_SECONDS)
  const [code, setCode] = useState('')
  const [password, setPassword] = useState(() => rememberedSignupPassword(email) ?? '')
  const [askPassword, setAskPassword] = useState(() => !rememberedSignupPassword(email))
  const passwordRef = useRef<HTMLInputElement>(null)

  if (verify.isSuccess) {
    return (
      <AuthLayout title="Email verified" panel="none">
        <EmailVerified />
      </AuthLayout>
    )
  }

  const submit = (value: string) => {
    if (value.length !== 6) return
    if (!password) {
      setAskPassword(true)
      passwordRef.current?.focus()
      return
    }
    verify.mutate(
      { body: { email, code: value, password } },
      {
        onSuccess: forgetSignupPassword,
        onError: (error) => {
          if (isApiError(error) && ['password_required', 'wrong_password'].includes(error.code)) {
            setAskPassword(true)
          }
        },
      },
    )
  }

  const errorCode = isApiError(verify.error) ? verify.error.code : undefined
  const passwordError = errorCode === 'wrong_password' ? errorMessage(verify.error) : undefined
  const wrong = verify.isError && code.length === 6 && !passwordError

  return (
    <AuthLayout title="Check your inbox" panel="none">
      <div className="flex flex-col gap-6">
        <p className="text-[15px] text-muted-foreground">
          We sent a 6-digit code to <strong className="text-foreground">{email}</strong>.{' '}
          <Link to="/signup" className="font-bold">
            Change email
          </Link>
        </p>
        <form
          className="flex flex-col gap-3"
          onSubmit={(e) => {
            e.preventDefault()
            submit(code)
          }}
        >
          <label htmlFor="code" className="font-bold">
            6-digit code
          </label>
          <InputOTP
            id="code"
            maxLength={6}
            value={code}
            onChange={(v) => {
              setCode(v)
              if (verify.isError) verify.reset()
            }}
            onComplete={submit}
            aria-invalid={wrong}
            aria-describedby={wrong ? 'code-error' : 'code-hint'}
            autoFocus
          >
            <InputOTPGroup>
              {[0, 1, 2, 3, 4, 5].map((i) => (
                <InputOTPSlot key={i} index={i} aria-invalid={wrong} />
              ))}
            </InputOTPGroup>
          </InputOTP>
          {wrong ? (
            <p id="code-error" role="alert" className="text-sm text-destructive">
              {errorMessage(verify.error)}
            </p>
          ) : (
            <p id="code-hint" className="text-sm text-muted-foreground">
              The code expires in 10 minutes. You can also open the link in the email.
            </p>
          )}
          {askPassword && (
            <FormField
              id="code-password"
              label="Password"
              description="The one you chose when you signed up."
              error={passwordError}
            >
              <PasswordInput
                ref={passwordRef}
                id="code-password"
                autoComplete="current-password"
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value)
                  if (verify.isError) verify.reset()
                }}
                aria-invalid={!!passwordError}
                aria-describedby={describedBy('code-password', {
                  error: passwordError,
                  hint: true,
                })}
              />
            </FormField>
          )}
          <Button
            type="submit"
            size="lg"
            disabled={code.length < 6 || (askPassword && !password) || verify.isPending}
          >
            {verify.isPending ? 'Checking…' : 'Verify email'}
          </Button>
        </form>
        <div className="flex flex-col gap-1 text-sm" aria-live="polite">
          {left > 0 ? (
            <p className="text-muted-foreground">
              Didn’t get it? You can ask for a new code in <strong>{formatSeconds(left)}</strong>.
            </p>
          ) : (
            <p>
              Didn’t get it?{' '}
              <button
                type="button"
                className="font-bold text-brand-text underline-offset-4 hover:underline"
                disabled={resend.isPending}
                onClick={() => resend.mutate({ body: { email } }, { onSuccess: restart })}
              >
                Send a new code
              </button>
            </p>
          )}
          {resend.isSuccess && left > 0 && (
            <p className="text-brand-text">New code sent. Only the latest code works.</p>
          )}
        </div>
        <InboxTips />
      </div>
    </AuthLayout>
  )
}

function InboxTips() {
  return (
    <div className="flex flex-col gap-2 rounded-xl bg-muted p-4 text-sm">
      <span className="flex items-center gap-2 font-bold">
        <MailIcon className="size-4" aria-hidden="true" />
        Can’t find the email?
      </span>
      <ul className="ml-6 list-disc text-muted-foreground">
        <li>Check your Spam or Promotions folder.</li>
        <li>Look for a message from no-reply@relay.app.</li>
        <li>Make sure the address above is right.</li>
      </ul>
    </div>
  )
}

function EmailVerified() {
  return (
    <div className="flex flex-col gap-5" role="status">
      <Alert variant="success">
        <CircleCheckIcon />
        <AlertTitle>Thanks, your email is confirmed</AlertTitle>
        <AlertDescription>Your account is ready. Next, a quick 5-minute setup.</AlertDescription>
      </Alert>
      <Button size="lg" asChild>
        <Link to="/">Continue</Link>
      </Button>
    </div>
  )
}

/**
 * Opened from the link in the verification email. An account with a password must
 * confirm it here: the link proves who owns the inbox, not who chose the password.
 */
export function VerifyEmailLink({ token }: { token: string }) {
  const verify = useVerifyEmailLink()
  const started = useRef(false)
  const [needsPassword, setNeedsPassword] = useState(false)
  const [password, setPassword] = useState('')

  useEffect(() => {
    if (started.current) return
    started.current = true
    // Without a password first: social-only accounts have none. A password account
    // answers password_required and the link stays usable.
    verify.mutate(
      { body: { token } },
      {
        onError: (error) => {
          if (isApiError(error) && error.code === 'password_required') setNeedsPassword(true)
        },
      },
    )
  }, [token, verify])

  if (verify.isSuccess) {
    return (
      <div className="flex flex-col gap-5" role="status">
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>Email verified</AlertTitle>
          <AlertDescription>Log in to finish setting up your account.</AlertDescription>
        </Alert>
        <Button size="lg" asChild>
          <Link to="/login">Log in</Link>
        </Button>
      </div>
    )
  }
  if (needsPassword) {
    const code = isApiError(verify.error) ? verify.error.code : undefined
    const fieldErr = code === 'wrong_password' ? errorMessage(verify.error) : undefined
    const formErr = verify.isError && code !== 'password_required' && !fieldErr
    return (
      <form
        noValidate
        className="flex flex-col gap-5"
        onSubmit={(e) => {
          e.preventDefault()
          if (password) verify.mutate({ body: { token, password } })
        }}
      >
        <p className="text-[15px] text-muted-foreground">
          Enter the password you chose when you signed up to confirm this email address.
        </p>
        {formErr && (
          <Alert variant="danger">
            <AlertDescription>{errorMessage(verify.error)}</AlertDescription>
          </Alert>
        )}
        <FormField id="verify-password" label="Password" error={fieldErr}>
          <PasswordInput
            id="verify-password"
            autoComplete="current-password"
            value={password}
            onChange={(e) => {
              setPassword(e.target.value)
              if (verify.isError) verify.reset()
            }}
            aria-invalid={!!fieldErr}
            aria-describedby={describedBy('verify-password', { error: fieldErr })}
            autoFocus
          />
        </FormField>
        <Button type="submit" size="lg" disabled={!password || verify.isPending}>
          {verify.isPending ? 'Checking…' : 'Verify email'}
        </Button>
        <p className="text-sm text-muted-foreground">
          Didn’t sign up, or don’t know this password? Don’t enter anything here.{' '}
          <Link to="/forgot-password" className="font-bold text-foreground">
            Set your own password
          </Link>{' '}
          instead.
        </p>
      </form>
    )
  }
  if (verify.isError) {
    return (
      <div className="flex flex-col gap-5">
        <Alert variant="danger">
          <AlertTitle>This link didn’t work</AlertTitle>
          <AlertDescription>
            {errorMessage(verify.error, 'It may have expired or been used already.')} Get a new one
            below.
          </AlertDescription>
        </Alert>
        <ResendVerification />
      </div>
    )
  }
  return <p className="text-muted-foreground">Checking your link…</p>
}

/** For someone who lands here without an email address (old link, new device). */
export function ResendVerification() {
  const resend = useResendVerification()
  const formRef = useRef<HTMLFormElement>(null)
  const form = useForm({
    defaultValues: { email: '' },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: ({ value }) =>
      resend.mutateAsync({ body: { email: value.email.trim() } }).catch(() => undefined),
  })

  if (resend.isSuccess) {
    return (
      <Alert variant="success" role="status">
        <CircleCheckIcon />
        <AlertTitle>Check your inbox</AlertTitle>
        <AlertDescription>
          If that address has an account waiting to be verified, a new email is on its way.
        </AlertDescription>
      </Alert>
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
      {resend.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(resend.error)}</AlertDescription>
        </Alert>
      )}
      <form.Field name="email" validators={{ onDynamic: emailSchema.shape.email }}>
        {(field) => {
          const error = fieldError(field)
          return (
            <FormField id="resend-email" label="Work email" error={error}>
              <Input
                id="resend-email"
                type="email"
                autoComplete="email"
                value={field.state.value}
                onChange={(e) => field.handleChange(e.target.value)}
                onBlur={field.handleBlur}
                aria-invalid={!!error}
                aria-describedby={describedBy('resend-email', { error })}
              />
            </FormField>
          )
        }}
      </form.Field>
      <form.Subscribe selector={(s) => s.isSubmitting}>
        {(submitting) => (
          <Button type="submit" size="lg" disabled={submitting}>
            {submitting ? 'Sending…' : 'Send verification email'}
          </Button>
        )}
      </form.Subscribe>
    </form>
  )
}
