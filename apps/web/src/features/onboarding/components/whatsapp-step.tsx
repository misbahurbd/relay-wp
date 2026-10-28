import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import {
  AlertTriangleIcon,
  CheckIcon,
  CircleCheckIcon,
  LoaderCircleIcon,
  LockIcon,
} from 'lucide-react'
import { type ReactNode, useEffect, useState } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { launchEmbeddedSignup, loadFacebookSdk } from '../api/embedded-signup'
import {
  numbersQueryOptions,
  signupConfigQueryOptions,
  useConnectWhatsApp,
  useRefreshProgress,
  useSkipWhatsApp,
} from '../api/onboarding'
import type { SetupTaskState, WhatsAppNumber } from '../types'
import { StepFooter } from './step-footer'

const META_STEPS = [
  ['Log in with Facebook', 'Use the account that manages your business page.'],
  ['Pick your business', 'Choose your Meta Business account, or create one in the same window.'],
  ['Add your number', 'Enter the phone number and the name customers will see.'],
  ['Verify by code', 'Type the 6-digit code Meta sends by SMS or voice call.'],
] as const

const SETUP_TASKS: { id: keyof WhatsAppNumber['setup_tasks']; label: string }[] = [
  { id: 'register', label: 'Register your number' },
  { id: 'subscribe_webhooks', label: 'Connect your Inbox to new messages' },
  { id: 'sync_templates', label: 'Copy your approved templates' },
  { id: 'import_profile', label: 'Import your business profile' },
]

type Props = { userId: string; workspaceId: string; onBack: () => void; onDone: () => void }

export function WhatsAppStep({ userId, workspaceId, onBack, onDone }: Props) {
  const navigate = useNavigate()
  const numbers = useQuery(numbersQueryOptions(userId, workspaceId))
  const signupConfig = useQuery(signupConfigQueryOptions())
  const connect = useConnectWhatsApp(userId, workspaceId)
  const skip = useSkipWhatsApp(userId, workspaceId)
  const refreshProgress = useRefreshProgress(userId, workspaceId)
  const [metaClosed, setMetaClosed] = useState(false)
  const fb = useFacebookSdk(signupConfig.data)

  // A failed or disconnected number goes back to the connect screen, with the reason.
  const latest = numbers.data?.[0]
  const number =
    latest?.status === 'setting_up' || latest?.status === 'connected' ? latest : undefined
  const connected = number?.status === 'connected'
  const enabled = signupConfig.data?.enabled === true
  const config = signupConfig.data

  const openMeta = () => {
    if (!fb.sdk || !config) return
    setMetaClosed(false)
    connect.reset()
    launchEmbeddedSignup(fb.sdk, config.config_id).then((result) => {
      if (result.status === 'cancelled') return setMetaClosed(true)
      const { status: _, ...ids } = result
      connect.mutate({ ...ids, number_type: 'new' })
    })
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex flex-col gap-1">
        <h2 id="step-title" className="text-xl">
          Connect your WhatsApp number
        </h2>
        <p className="text-muted-foreground">
          A secure Meta window opens. It takes about 3 minutes, and you never share your password
          with us.
        </p>
      </div>

      {number ? (
        <NumberStatus number={number} />
      ) : (
        <>
          <ol className="grid gap-3 sm:grid-cols-2">
            {META_STEPS.map(([title, body], i) => (
              <li key={title} className="flex gap-3 rounded-lg bg-muted p-3.5">
                <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-info-soft text-sm font-extrabold text-info">
                  {i + 1}
                </span>
                <span className="flex flex-col">
                  <span className="font-bold">{title}</span>
                  <span className="text-sm text-muted-foreground">{body}</span>
                </span>
              </li>
            ))}
          </ol>

          <div className="flex flex-col gap-2.5">
            <h3 className="text-base">Before you start, have these ready</h3>
            <ul className="flex flex-col gap-2 text-sm">
              {[
                'A Facebook account with admin access to your business',
                'A phone number that can receive an SMS or call',
                'Your business website or page, so Meta can review your display name',
              ].map((t) => (
                <li key={t} className="flex gap-2">
                  <CheckIcon
                    className="mt-0.5 size-4 shrink-0 text-brand-text"
                    aria-hidden="true"
                  />
                  {t}
                </li>
              ))}
            </ul>
            <Alert variant="warning">
              <AlertTriangleIcon />
              <AlertDescription>
                <p>
                  The number must <strong>not</strong> be active on the WhatsApp or WhatsApp
                  Business app. Delete that account first, or use a new number.
                </p>
              </AlertDescription>
            </Alert>
          </div>

          {signupConfig.isSuccess && !enabled && (
            <Alert variant="info">
              <AlertTitle>WhatsApp connection isn’t switched on here</AlertTitle>
              <AlertDescription>
                This Relay server has no Meta app set up yet. Skip for now; you can connect your
                number later.
              </AlertDescription>
            </Alert>
          )}
          {latest?.status === 'failed' && (
            <Alert variant="danger" role="alert">
              <AlertTriangleIcon />
              <AlertTitle>We couldn’t finish setting up {latest.display_phone_number}</AlertTitle>
              <AlertDescription>
                {latest.last_error?.message ?? 'Meta didn’t accept one of the setup steps.'} Connect
                again to retry.
              </AlertDescription>
            </Alert>
          )}
          {latest?.status === 'disconnected' && (
            <Alert variant="danger" role="alert">
              <AlertTriangleIcon />
              <AlertTitle>WhatsApp is disconnected</AlertTitle>
              <AlertDescription>
                Access to {latest.display_phone_number} was removed in Meta. Connect it again to
                keep messaging.
              </AlertDescription>
            </Alert>
          )}
          {fb.failed && (
            <Alert variant="warning" role="alert">
              <AlertTriangleIcon />
              <AlertDescription>
                <p>
                  Meta’s sign-in couldn’t load. Check that nothing is blocking facebook.com, then
                  reload the page.
                </p>
              </AlertDescription>
            </Alert>
          )}
          {metaClosed && (
            <Alert variant="warning" role="alert">
              <AlertTriangleIcon />
              <AlertTitle>Meta’s window was closed</AlertTitle>
              <AlertDescription>
                Nothing was saved yet, so it’s safe to start again. If nothing opened, allow pop-ups
                for this site.
              </AlertDescription>
            </Alert>
          )}
          {connect.isError && (
            <Alert variant="danger">
              <AlertDescription>{errorMessage(connect.error)}</AlertDescription>
            </Alert>
          )}

          <div className="flex flex-col items-start gap-2">
            <Button
              variant="meta"
              size="lg"
              onClick={openMeta}
              disabled={!enabled || !fb.sdk || connect.isPending}
            >
              <LockIcon aria-hidden="true" />
              {connect.isPending
                ? 'Connecting…'
                : metaClosed
                  ? 'Open Meta’s window again'
                  : 'Connect with Meta'}
            </Button>
            <span className="text-sm text-muted-foreground">Secure Meta Embedded Signup</span>
          </div>
        </>
      )}

      {skip.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(skip.error)}</AlertDescription>
        </Alert>
      )}

      <StepFooter>
        <Button variant="ghost" onClick={onBack}>
          Back
        </Button>
        <div className="flex flex-col-reverse items-stretch gap-3 sm:flex-row sm:items-center">
          {!number && (
            <Button
              variant="ghost"
              disabled={skip.isPending}
              onClick={() =>
                skip.mutate(undefined, {
                  onSuccess: () => navigate({ to: '/w/$workspaceId', params: { workspaceId } }),
                })
              }
            >
              {skip.isPending ? 'Skipping…' : 'Skip for now'}
            </Button>
          )}
          {connected ? (
            <Button
              onClick={async () => {
                await refreshProgress()
                onDone()
              }}
              className="sm:min-w-36"
            >
              Continue
            </Button>
          ) : (
            <span className="text-sm text-muted-foreground">
              {number ? 'Finishing setup…' : 'Connect a number to continue'}
            </span>
          )}
        </div>
      </StepFooter>
    </div>
  )
}

/** Loads Facebook's SDK as soon as the step shows, so the click can open Meta's window. */
function useFacebookSdk(
  config: { enabled: boolean; app_id: string; graph_version: string } | undefined,
) {
  const [state, setState] = useState<{
    sdk?: Awaited<ReturnType<typeof loadFacebookSdk>>
    failed: boolean
  }>({ failed: false })
  useEffect(() => {
    if (!config?.enabled) return
    let live = true
    loadFacebookSdk(config.app_id, config.graph_version).then(
      (sdk) => live && setState({ sdk, failed: false }),
      () => live && setState({ failed: true }),
    )
    return () => {
      live = false
    }
  }, [config])
  return state
}

const MESSAGING_LIMITS: Record<string, string> = {
  TIER_50: '50',
  TIER_250: '250',
  TIER_1K: '1,000',
  TIER_10K: '10,000',
  TIER_100K: '100,000',
}

function messagingLimit(tier: string | null) {
  if (tier === 'TIER_UNLIMITED') return 'Unlimited'
  const n = tier ? MESSAGING_LIMITS[tier] : undefined
  return n ? `${n} customers / 24h` : 'Set by Meta'
}

function NumberStatus({ number }: { number: WhatsAppNumber }) {
  if (number.status === 'connected') {
    return (
      <div className="flex flex-col gap-4" role="status">
        <Alert variant="success">
          <CircleCheckIcon />
          <AlertTitle>WhatsApp connected</AlertTitle>
          <AlertDescription>Your number is ready to send and receive messages.</AlertDescription>
        </Alert>
        <dl className="grid gap-3 sm:grid-cols-2">
          <Fact label="Phone number" value={number.display_phone_number} />
          <Fact label="Display name" value={number.verified_name} />
          <Fact
            label="Name review"
            value={
              number.name_status === 'approved' ? (
                <Badge variant="success">Approved</Badge>
              ) : (
                <Badge variant="warning">In review by Meta</Badge>
              )
            }
          />
          <Fact label="Messaging limit" value={messagingLimit(number.messaging_limit)} />
        </dl>
        <p className="text-sm text-muted-foreground">
          Limits go up automatically as you send good-quality messages. Verify your business in Meta
          Business Settings to start at 1,000.
        </p>
      </div>
    )
  }
  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-col gap-0.5">
        <h3 className="text-base">Setting up your number</h3>
        <p className="text-sm text-muted-foreground">
          Meta approved the connection. We’re finishing the technical setup. Please keep this page
          open.
        </p>
      </div>
      <ul className="flex flex-col gap-2" aria-live="polite">
        {SETUP_TASKS.map((t) => (
          <SetupTask key={t.id} label={t.label} state={number.setup_tasks[t.id]} />
        ))}
      </ul>
    </div>
  )
}

function SetupTask({ label, state }: { label: string; state: SetupTaskState }) {
  return (
    <li
      className={cn(
        'flex items-center gap-3 rounded-lg border p-3',
        state === 'running' && 'border-transparent bg-info-soft',
      )}
    >
      {state === 'done' && (
        <CircleCheckIcon className="size-5 text-brand-text" aria-hidden="true" />
      )}
      {state === 'running' && (
        <LoaderCircleIcon className="size-5 animate-spin text-info" aria-hidden="true" />
      )}
      {(state === 'waiting' || state === 'failed') && (
        <span className="size-5 rounded-full border-2 border-input" aria-hidden="true" />
      )}
      <span className="flex-1 font-bold">{label}</span>
      <span className="text-sm text-muted-foreground">
        {{ done: 'Done', running: 'In progress…', waiting: 'Waiting', failed: 'Failed' }[state]}
      </span>
    </li>
  )
}

function Fact({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="flex flex-col gap-1 rounded-lg bg-muted p-3">
      <dt className="text-xs font-bold text-muted-foreground">{label}</dt>
      <dd className="font-bold">{value}</dd>
    </div>
  )
}
