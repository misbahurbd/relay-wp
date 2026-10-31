import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowRightIcon, CheckIcon, ClockIcon, LinkIcon, XIcon } from 'lucide-react'
import { type FormEvent, type ReactNode, useState } from 'react'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage, isApiError } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useAcceptInvitation, useInviteSignup, useRequestRenewal } from '../../api/team.mutations'
import { invitePreviewQueryOptions } from '../../api/team.queries'
import { passwordSchema } from '../../schemas/team.schema'
import { initials } from '../kit/avatar'
import { Btn } from '../kit/btn'
import { ErrorText, inputClass } from '../kit/field'
import { formatDate } from '../kit/format'
import { Pill } from '../kit/pill'

const TOKEN_KEY = 'relay-invite-token'

/**
 * The invite token travels in the link's #fragment, so it never reaches server logs. It's
 * kept for this tab while the invitee signs in or creates an account, then forgotten.
 */
export function inviteTokenFromLink(hash: string): string | null {
  const fromHash = new URLSearchParams(hash.replace(/^#/, '')).get('t')
  try {
    if (fromHash) sessionStorage.setItem(TOKEN_KEY, fromHash)
    return fromHash ?? sessionStorage.getItem(TOKEN_KEY)
  } catch {
    return fromHash
  }
}

/** An invite opened in this tab but not accepted yet, e.g. while the invitee signed up. */
export function pendingInviteToken(): string | null {
  try {
    return sessionStorage.getItem(TOKEN_KEY)
  } catch {
    return null
  }
}

function forgetInviteToken() {
  try {
    sessionStorage.removeItem(TOKEN_KEY)
  } catch {
    // Nothing kept.
  }
}

type Props = {
  token: string | null
  /** The signed-in user's email, or null when signed out. */
  signedInAs: string | null
  onSwitchAccount: () => void
  /** Opens a workspace the invitee belongs to; the app picks it in the switcher. */
  onOpenWorkspace: (workspaceId: string) => void
  /** "Continue with Google" and friends, shown above the password form when signed out. */
  social?: ReactNode
}

/** The page an invite link opens (prototype 25): one card, centred. */
export function InviteAccept(props: Props) {
  return (
    <section
      aria-label="Accept invite"
      className="flex w-full flex-col gap-5 rounded-xl border bg-card p-6 shadow-card sm:p-7"
    >
      <AcceptBody {...props} />
    </section>
  )
}

function StateIcon({ tone, children }: { tone: 'green' | 'amber' | 'red'; children: ReactNode }) {
  return (
    <span
      className={cn(
        'flex size-16 items-center justify-center rounded-full',
        tone === 'green' && 'bg-primary text-primary-foreground',
        tone === 'amber' && 'bg-warning-soft text-warning',
        tone === 'red' && 'bg-destructive-soft text-destructive',
      )}
    >
      {children}
    </span>
  )
}

function Centered({ children }: { children: ReactNode }) {
  return <div className="flex flex-col items-center gap-3.5 py-5 text-center">{children}</div>
}

function AcceptBody({ token, signedInAs, onSwitchAccount, onOpenWorkspace, social }: Props) {
  const preview = useQuery({ ...invitePreviewQueryOptions(token ?? ''), enabled: !!token })
  const accept = useAcceptInvitation()
  const renew = useRequestRenewal()
  const signup = useInviteSignup()
  const [joined, setJoined] = useState<string | null>(null)
  const [name, setName] = useState('')
  const [password, setPassword] = useState('')
  const [errors, setErrors] = useState<{ name?: string; password?: string; form?: ReactNode }>({})

  if (!token || preview.isError) {
    return (
      <Centered>
        <StateIcon tone="red">
          <LinkIcon className="size-7" aria-hidden="true" />
        </StateIcon>
        <h1 className="text-2xl font-extrabold">This invite link doesn’t work</h1>
        <p className="max-w-[380px] text-muted-foreground">
          Check you opened the whole link from the email, or ask whoever invited you to send a new
          one.
        </p>
      </Centered>
    )
  }
  if (!preview.data) return <Skeleton className="h-96 w-full" />

  const inv = preview.data
  const first = (name.trim() || inv.email).split(/[\s@]/)[0]

  if (joined) {
    return (
      <Centered>
        <StateIcon tone="green">
          <CheckIcon className="size-7" aria-hidden="true" />
        </StateIcon>
        <h1 className="text-2xl font-extrabold">You’re in{name.trim() ? `, ${first}` : ''}</h1>
        <p className="max-w-[360px] text-muted-foreground">
          You joined {inv.workspace_name} as {inv.role_name}
          {inv.team_names.length > 0 &&
            ` on the ${inv.team_names.join(' and ')} ${inv.team_names.length === 1 ? 'team' : 'teams'}`}
          . {inv.invited_by_name} has been notified.
        </p>
        <div className="flex w-full max-w-[360px] flex-col gap-2 rounded-lg bg-muted p-3.5 text-left text-[13px]">
          <span className="font-bold">Quick start</span>
          <span>1. Set your status to Online so chats can be assigned to you</span>
          <span>2. Open a chat and try a quick reply</span>
          <span>3. Turn on browser notifications</span>
        </div>
        <Button
          variant="unstyled"
          type="button"
          onClick={() => onOpenWorkspace(joined)}
          className="inline-flex h-12 items-center gap-2 rounded-lg bg-primary px-[22px] font-extrabold text-primary-foreground hover:bg-primary/90"
        >
          Open {inv.workspace_name}
          <ArrowRightIcon className="size-[18px]" aria-hidden="true" />
        </Button>
      </Centered>
    )
  }

  if (inv.state === 'revoked') {
    return (
      <Centered>
        <StateIcon tone="red">
          <XIcon className="size-7" aria-hidden="true" />
        </StateIcon>
        <h1 className="text-2xl font-extrabold">This invite is no longer active</h1>
        <p className="max-w-[380px] text-muted-foreground">
          {inv.invited_by_name} cancelled this invite. If you think this is a mistake, contact the
          person who invited you.
        </p>
        <Link to="/signup" className="font-bold">
          Create your own Relay workspace
        </Link>
      </Centered>
    )
  }
  if (inv.state === 'accepted') {
    return (
      <Centered>
        <StateIcon tone="green">
          <CheckIcon className="size-7" aria-hidden="true" />
        </StateIcon>
        <h1 className="text-2xl font-extrabold">This invite was already used</h1>
        <p className="max-w-[380px] text-muted-foreground">
          {inv.email} already joined {inv.workspace_name}. Log in to open it.
        </p>
        <Button
          variant="unstyled"
          type="button"
          onClick={() => onOpenWorkspace(inv.workspace_id)}
          className="inline-flex h-12 items-center rounded-lg bg-primary px-[22px] font-extrabold text-primary-foreground hover:bg-primary/90"
        >
          Open {inv.workspace_name}
        </Button>
      </Centered>
    )
  }
  if (inv.state === 'expired') {
    return (
      <Centered>
        <StateIcon tone="amber">
          <ClockIcon className="size-7" aria-hidden="true" />
        </StateIcon>
        <h1 className="text-2xl font-extrabold">This invite expired</h1>
        <p className="max-w-[380px] text-muted-foreground">
          It ran out on {formatDate(inv.expires_at)}. Ask {inv.invited_by_name} to resend it, or
          request a new one and we’ll let them know.
        </p>
        {renew.isSuccess ? (
          <span
            role="status"
            className="inline-flex items-center gap-2 rounded-[10px] bg-brand-soft px-3.5 py-2.5 font-bold text-brand-text"
          >
            <CheckIcon className="size-[18px]" aria-hidden="true" />
            Request sent to {inv.invited_by_name}
          </span>
        ) : (
          <Btn
            className="h-12 px-[22px] font-extrabold"
            onClick={() => renew.mutate(token)}
            disabled={renew.isPending}
          >
            {renew.isPending ? 'Sending…' : 'Request a new invite'}
          </Btn>
        )}
        <ErrorText>{renew.isError && errorMessage(renew.error)}</ErrorText>
      </Centered>
    )
  }

  const wrongAccount = !!signedInAs && signedInAs.toLowerCase() !== inv.email.toLowerCase()
  const can = [
    ...inv.can_do,
    ...(inv.number_labels.length > 0 ? [`Work from ${inv.number_labels.join(', ')}`] : []),
  ]

  function submit(e: FormEvent) {
    e.preventDefault()
    const next: typeof errors = {}
    if (!name.trim()) next.name = 'Enter your name'
    const pw = passwordSchema.safeParse(password)
    if (!pw.success) next.password = pw.error.issues[0]?.message
    setErrors(next)
    if (next.name || next.password || !token) return
    const [firstName, ...rest] = name.trim().split(/\s+/)
    signup.mutate(
      {
        token,
        first_name: firstName,
        last_name: rest.join(' ') || undefined,
        password,
        accept_terms: true,
      },
      {
        onSuccess: (data) => {
          forgetInviteToken()
          if (data.workspace_id) setJoined(data.workspace_id)
          else
            setErrors({
              form: data.join_error ?? 'Your account is ready, but joining didn’t work.',
            })
        },
        onError: (err) => {
          if (isApiError(err) && err.code === 'account_exists') {
            setErrors({
              form: (
                <>
                  {err.message}{' '}
                  <Link
                    to="/login"
                    search={{ email: inv.email, redirect: '/invite' }}
                    className="font-bold"
                  >
                    Log in to accept
                  </Link>
                </>
              ),
            })
          } else if (isApiError(err) && /password/i.test(err.message)) {
            setErrors({ password: err.message })
          } else {
            setErrors({ form: errorMessage(err) })
          }
        },
      },
    )
  }

  return (
    <>
      <div className="flex flex-col items-center gap-2.5 text-center">
        <span className="flex size-[60px] items-center justify-center rounded-[18px] bg-brand-soft text-xl font-extrabold text-brand-text">
          {initials(inv.workspace_name)}
        </span>
        <h1 className="text-2xl font-extrabold tracking-[-0.02em]">Join {inv.workspace_name}</h1>
        <p className="text-muted-foreground">
          Invited by {inv.invited_by_name}
          {inv.invited_by_role && ` (${inv.invited_by_role})`} ·{' '}
          <Pill tone="blue">{inv.role_name}</Pill>
        </p>
      </div>
      {inv.message && (
        <blockquote className="rounded-[10px] bg-muted px-3.5 py-3 text-sm">
          “{inv.message}”
        </blockquote>
      )}
      {can.length > 0 && (
        <div className="flex flex-col gap-2 rounded-lg bg-muted px-4 py-3.5">
          <span className="font-bold">You’ll be able to</span>
          <ul className="flex flex-col gap-2">
            {can.map((c) => (
              <li key={c} className="flex gap-2 text-[13px]">
                <CheckIcon className="mt-0.5 size-4 flex-none text-brand-text" aria-hidden="true" />
                {c}
              </li>
            ))}
            {inv.team_names.length > 0 && (
              <li className="flex gap-2 text-[13px]">
                <CheckIcon className="mt-0.5 size-4 flex-none text-brand-text" aria-hidden="true" />
                Work on the {inv.team_names.join(' and ')}{' '}
                {inv.team_names.length === 1 ? 'team' : 'teams'}
              </li>
            )}
          </ul>
        </div>
      )}

      {wrongAccount ? (
        <div className="flex flex-col items-start gap-3 rounded-lg bg-warning-soft px-4 py-3.5 text-sm">
          <span>
            <strong>You’re signed in as {signedInAs}.</strong> This invite is for {inv.email}. Sign
            out, then sign in with that address.
          </span>
          <Btn variant="outline" size="md" onClick={onSwitchAccount}>
            Switch account
          </Btn>
        </div>
      ) : signedInAs ? (
        <>
          <Btn
            className="h-[50px] w-full text-[15px] font-extrabold"
            disabled={accept.isPending}
            onClick={() =>
              accept.mutate(token, {
                onSuccess: (workspaceId) => {
                  forgetInviteToken()
                  setJoined(workspaceId)
                },
              })
            }
          >
            {accept.isPending ? 'Joining…' : `Accept and join ${inv.workspace_name}`}
          </Btn>
          <ErrorText>{accept.isError && errorMessage(accept.error)}</ErrorText>
          <p className="text-center text-xs text-muted-foreground">
            Signed in as {signedInAs} · link expires {formatDate(inv.expires_at)}
          </p>
        </>
      ) : (
        <form onSubmit={submit} noValidate className="flex flex-col gap-5">
          {social}
          <label className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Email</span>
            <Input
              unstyled
              type="email"
              readOnly
              value={inv.email}
              className={cn(inputClass, 'h-[46px] text-muted-foreground')}
            />
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Full name</span>
            <Input
              unstyled
              type="text"
              autoComplete="name"
              value={name}
              onChange={(e) => setName(e.target.value)}
              aria-invalid={!!errors.name}
              aria-describedby={errors.name ? 'join-name-error' : undefined}
              className={cn(inputClass, 'h-[46px] bg-card')}
            />
            <ErrorText id="join-name-error">{errors.name}</ErrorText>
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Create a password</span>
            <Input
              unstyled
              type="password"
              autoComplete="new-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              placeholder="At least 8 characters"
              aria-invalid={!!errors.password}
              aria-describedby={errors.password ? 'join-password-error' : undefined}
              className={cn(inputClass, 'h-[46px] bg-card')}
            />
            <ErrorText id="join-password-error">{errors.password}</ErrorText>
          </label>
          <ErrorText>{errors.form}</ErrorText>
          <Btn
            type="submit"
            className="h-[50px] w-full text-[15px] font-extrabold"
            disabled={signup.isPending}
          >
            {signup.isPending ? 'Joining…' : 'Accept and join'}
          </Btn>
          <p className="text-center text-xs text-muted-foreground">
            By joining you agree to the{' '}
            <a href="/terms" target="_blank" rel="noreferrer">
              Terms
            </a>
            . Already have a Relay account?{' '}
            <Link
              to="/login"
              search={{ email: inv.email, redirect: '/invite' }}
              className="font-bold"
            >
              Log in to accept
            </Link>
          </p>
        </form>
      )}
    </>
  )
}
