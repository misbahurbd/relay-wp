import { Elements, PaymentElement, useElements, useStripe } from '@stripe/react-stripe-js'
import { loadStripe, type Stripe } from '@stripe/stripe-js'
import { LockIcon } from 'lucide-react'
import { useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { useTheme } from '#/hooks/use-theme'

// One Stripe.js instance per publishable key, loaded on first use.
const stripes = new Map<string, Promise<Stripe | null>>()
function stripeFor(publishableKey: string) {
  let s = stripes.get(publishableKey)
  if (!s) {
    s = loadStripe(publishableKey)
    stripes.set(publishableKey, s)
  }
  return s
}

/**
 * Stripe Elements for a checkout's client secret. The secret goes straight from the API to
 * Stripe and is never stored. A SetupIntent (seti_…) saves a card for a trial; anything else
 * is a PaymentIntent.
 */
export function StripePaymentForm({
  publishableKey,
  clientSecret,
  returnUrl,
  payLabel,
  onPaid,
}: {
  publishableKey: string
  clientSecret: string
  returnUrl: string
  payLabel: string
  onPaid: () => void
}) {
  const { resolvedTheme } = useTheme()
  return (
    <Elements
      key={clientSecret}
      stripe={stripeFor(publishableKey)}
      options={{
        clientSecret,
        appearance: {
          theme: resolvedTheme === 'dark' ? 'night' : 'stripe',
          variables: {
            fontFamily: '"Plus Jakarta Sans Variable", system-ui, sans-serif',
            borderRadius: '10px',
            fontSizeBase: '15px',
          },
        },
      }}
    >
      <ConfirmForm
        clientSecret={clientSecret}
        returnUrl={returnUrl}
        payLabel={payLabel}
        onPaid={onPaid}
      />
    </Elements>
  )
}

function ConfirmForm({
  clientSecret,
  returnUrl,
  payLabel,
  onPaid,
}: {
  clientSecret: string
  returnUrl: string
  payLabel: string
  onPaid: () => void
}) {
  const stripe = useStripe()
  const elements = useElements()
  const [ready, setReady] = useState(false)
  const [submitting, setSubmitting] = useState(false)
  const [error, setError] = useState<string>()

  const submit = async () => {
    if (!stripe || !elements) return
    setSubmitting(true)
    setError(undefined)
    const confirmParams = { return_url: returnUrl }
    const result = clientSecret.startsWith('seti_')
      ? await stripe.confirmSetup({ elements, confirmParams, redirect: 'if_required' })
      : await stripe.confirmPayment({ elements, confirmParams, redirect: 'if_required' })
    setSubmitting(false)
    if (result.error) {
      // Card and validation errors are written for the payer; anything else gets a generic line.
      setError(
        result.error.type === 'card_error' || result.error.type === 'validation_error'
          ? result.error.message
          : 'The payment didn’t go through. Please try again.',
      )
      return
    }
    onPaid()
  }

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        void submit()
      }}
    >
      <PaymentElement
        onReady={() => setReady(true)}
        onLoadError={() =>
          setError('The payment form couldn’t load. Check your connection and try again.')
        }
      />
      {error && (
        <Alert variant="danger">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}
      <Button type="submit" size="lg" disabled={!stripe || !ready || submitting}>
        <LockIcon aria-hidden="true" />
        {submitting ? 'Processing…' : payLabel}
      </Button>
    </form>
  )
}
