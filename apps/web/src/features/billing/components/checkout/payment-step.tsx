import { ExternalLinkIcon, FlaskConicalIcon } from 'lucide-react'
import { useEffect } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { PROVIDER_LABEL } from '../../schemas/billing.schema'
import type { StartedCheckout } from '../../types'
import { checkoutReturnUrl, rememberCheckout } from './checkout-return'
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
 * a client secret, the provider's own page when it sent a URL.
 */
export function PaymentStep({
  started,
  payLabel,
  onPaid,
}: {
  started: StartedCheckout
  payLabel: string
  onPaid: (sessionId: string) => void
}) {
  const { session } = started
  if (isFake(started)) return <FakeGateway onDone={() => onPaid(session.id)} />
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
    return (
      <HostedCheckout
        url={started.url}
        sessionId={session.id}
        provider={PROVIDER_LABEL[session.provider]}
      />
    )
  // Nothing to pay (a full discount or credit): the session settles by itself.
  if (session.amount_minor === 0 || session.status !== 'pending') {
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

function FakeGateway({ onDone }: { onDone: () => void }) {
  return (
    <div className="flex flex-col gap-3">
      <Alert variant="info">
        <FlaskConicalIcon aria-hidden="true" />
        <AlertTitle>Test gateway</AlertTitle>
        <AlertDescription>
          <p>
            This server uses the fake payment gateway, so there is no card form. The checkout
            completes when a signed fake webhook arrives.
          </p>
        </AlertDescription>
      </Alert>
      <Button size="lg" onClick={onDone}>
        Check payment status
      </Button>
    </div>
  )
}
