import { useQuery, useQueryClient } from '@tanstack/react-query'
import { ExternalLinkIcon, FlaskConicalIcon, LoaderCircleIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { invalidateBilling } from '../../api/billing.keys'
import { useCompleteTestCheckout } from '../../api/billing.mutations'
import { plansQueryOptions } from '../../api/billing.queries'
import { PROVIDER_LABEL } from '../../schemas/billing.schema'
import type { StartedCheckout } from '../../types'
import { checkoutReturnUrl, rememberCheckout } from './checkout-return'
import { EmbeddedCheckout } from './embedded-checkout'
import { StripePaymentForm } from './stripe-form'

/** The fake gateway (PAYMENT_PROVIDERS=stripe:fake) hands out these instead of real keys. */
function isFake(started: StartedCheckout) {
  return (
    started.publishable_key === 'pk_fake' ||
    started.client_secret?.startsWith('fake_') ||
    started.url?.startsWith('https://fake.invalid/')
  )
}

/**
 * Pays a started checkout the way its provider works: an embedded Stripe form when the API sent
 * a client secret, the provider's page over this one when it sent an embeddable URL, else a
 * redirect to that URL. A checkout that is already settled (Relay completes one with nothing to
 * pay) goes straight to its result. `onRestart` starts a fresh checkout once this one has
 * expired; without it the member is told to start over.
 */
export function PaymentStep({
  workspaceId,
  started,
  payLabel,
  onPaid,
  onRestart,
}: {
  workspaceId: string
  started: StartedCheckout
  payLabel: string
  onPaid: (sessionId: string) => void
  onRestart?: () => void
}) {
  const { session } = started
  if (session.status !== 'pending') {
    return <Settled workspaceId={workspaceId} onDone={() => onPaid(session.id)} />
  }
  if (isFake(started))
    return (
      <FakeGateway
        workspaceId={workspaceId}
        sessionId={session.id}
        onDone={() => onPaid(session.id)}
      />
    )
  if (started.client_secret && started.publishable_key) {
    return (
      <StripePaymentForm
        publishableKey={started.publishable_key}
        clientSecret={started.client_secret}
        returnUrl={checkoutReturnUrl(session.id)}
        payLabel={payLabel}
        onPaid={() => onPaid(session.id)}
      />
    )
  }
  if (started.url)
    return <UrlCheckout started={started} url={started.url} onPaid={onPaid} onRestart={onRestart} />
  // Nothing to pay (a full discount or credit): the session settles by itself.
  if (session.amount_minor === 0) {
    return (
      <Button size="lg" onClick={() => onPaid(session.id)}>
        Continue
      </Button>
    )
  }
  return (
    <Alert variant="danger">
      <AlertTitle>This payment method can’t start</AlertTitle>
      <AlertDescription>Choose another payment method, or try again later.</AlertDescription>
    </Alert>
  )
}

function Settled({ workspaceId, onDone }: { workspaceId: string; onDone: () => void }) {
  const queryClient = useQueryClient()
  // biome-ignore lint/correctness/useExhaustiveDependencies: run once, when the checkout settles
  useEffect(() => {
    void invalidateBilling(queryClient, workspaceId)
    onDone()
  }, [])
  return (
    <p className="flex items-center gap-2 text-sm" role="status">
      <LoaderCircleIcon className="size-4 animate-spin text-brand-text" aria-hidden="true" />
      Finishing your checkout…
    </p>
  )
}

/** The provider's own checkout page: over this page when the catalog says it can be, else a redirect. */
function UrlCheckout({
  started,
  url,
  onPaid,
  onRestart,
}: {
  started: StartedCheckout
  url: string
  onPaid: (sessionId: string) => void
  onRestart?: () => void
}) {
  const { session } = started
  const catalog = useQuery(plansQueryOptions())
  const [embedFailed, setEmbedFailed] = useState(false)
  const provider = PROVIDER_LABEL[session.provider]

  if (catalog.isPending) {
    return (
      <p className="flex items-center gap-2 text-sm" role="status">
        <LoaderCircleIcon className="size-4 animate-spin text-brand-text" aria-hidden="true" />
        Preparing your checkout…
      </p>
    )
  }
  const embeddable = catalog.data?.providers.find(
    (p) => p.name === session.provider,
  )?.embeddable_url
  if (embeddable && !embedFailed) {
    return (
      <EmbeddedCheckout
        url={url}
        sessionId={session.id}
        provider={provider}
        expiresAt={session.expires_at}
        onPaid={onPaid}
        onUnavailable={() => setEmbedFailed(true)}
        onRestart={onRestart}
      />
    )
  }
  return <HostedCheckout url={url} sessionId={session.id} provider={provider} />
}

function HostedCheckout({
  url,
  sessionId,
  provider,
}: {
  url: string
  sessionId: string
  provider: string
}) {
  useEffect(() => {
    rememberCheckout(sessionId)
    window.location.assign(url)
  }, [url, sessionId])
  return (
    <div className="flex flex-col gap-3" role="status">
      <p className="text-sm">Opening {provider}’s secure checkout…</p>
      <Button asChild variant="outline" size="lg">
        <a href={url}>
          Open {provider} checkout
          <ExternalLinkIcon aria-hidden="true" />
        </a>
      </Button>
    </div>
  )
}

function FakeGateway({
  workspaceId,
  sessionId,
  onDone,
}: {
  workspaceId: string
  sessionId: string
  onDone: () => void
}) {
  const complete = useCompleteTestCheckout(workspaceId)
  return (
    <div className="flex flex-col gap-3">
      <Alert variant="info">
        <FlaskConicalIcon aria-hidden="true" />
        <AlertTitle>Test gateway</AlertTitle>
        <AlertDescription>
          <p>
            This server uses the fake payment gateway, so there is no card form. Complete the test
            payment to send the signed webhook a real provider would.
          </p>
        </AlertDescription>
      </Alert>
      {complete.isError && (
        <Alert variant="danger">
          <AlertDescription>The test payment didn’t go through. Try again.</AlertDescription>
        </Alert>
      )}
      <Button
        size="lg"
        disabled={complete.isPending}
        onClick={() => complete.mutate(sessionId, { onSuccess: onDone })}
      >
        {complete.isPending ? 'Completing…' : 'Complete test payment'}
      </Button>
      <Button variant="outline" size="lg" onClick={onDone}>
        Check payment status
      </Button>
    </div>
  )
}
