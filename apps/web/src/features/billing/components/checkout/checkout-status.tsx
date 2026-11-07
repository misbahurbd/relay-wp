import { useQuery, useQueryClient } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CircleCheckIcon, CircleXIcon, ClockIcon, LoaderCircleIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '#/components/ui/button'
import { invalidateBilling } from '../../api/billing.keys'
import { checkoutSessionQueryOptions } from '../../api/billing.queries'
import { Section } from '../kit/section'
import { ErrorState } from '../kit/states'
import { rememberedCheckout } from './checkout-return'

/** After paying: waits for the provider's webhook to complete the checkout, then says so. */
export function CheckoutStatus({
  workspaceId,
  sessionId,
}: {
  workspaceId: string
  sessionId: string
}) {
  const id = sessionId === 'return' ? rememberedCheckout() : sessionId
  return (
    <main className="mx-auto flex w-full max-w-[560px] flex-col gap-5 p-4 sm:p-7">
      <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Payment</h1>
      {id ? (
        <SessionState workspaceId={workspaceId} sessionId={id} />
      ) : (
        <Section>
          <p className="text-sm">
            We couldn’t tell which checkout you came back from. Your billing page shows whether the
            payment went through.
          </p>
          <BillingLink />
        </Section>
      )}
    </main>
  )
}

/** How long to poll a pending checkout before telling the member to check back later. */
const WAIT_MS = 60_000

function SessionState({ workspaceId, sessionId }: { workspaceId: string; sessionId: string }) {
  const queryClient = useQueryClient()
  const [gaveUp, setGaveUp] = useState(false)
  const options = checkoutSessionQueryOptions(workspaceId, sessionId)
  const session = useQuery({
    ...options,
    refetchInterval: gaveUp ? false : options.refetchInterval,
  })
  const status = session.data?.status
  const waiting = !session.isError && (!status || status === 'pending')

  useEffect(() => {
    if (!waiting || gaveUp) return
    const timer = setTimeout(() => setGaveUp(true), WAIT_MS)
    return () => clearTimeout(timer)
  }, [waiting, gaveUp])

  useEffect(() => {
    if (status === 'completed') void invalidateBilling(queryClient, workspaceId)
  }, [status, queryClient, workspaceId])

  if (session.isError) {
    return (
      <ErrorState
        title="Couldn’t check this payment"
        error={session.error}
        onRetry={() => void session.refetch()}
      />
    )
  }

  if (waiting && gaveUp) {
    return (
      <Section role="status" aria-live="polite">
        <div className="flex items-center gap-3">
          <ClockIcon className="size-5 text-warning" aria-hidden="true" />
          <p className="font-bold">Still waiting for the payment provider</p>
        </div>
        <p className="text-sm text-muted-foreground">
          Your plan updates as soon as it confirms. If it stays quiet, we follow up with it
          ourselves, so there’s no need to pay again.
        </p>
        <div className="flex flex-wrap gap-2">
          <Button
            disabled={session.isFetching}
            onClick={() => {
              setGaveUp(false)
              void session.refetch()
            }}
          >
            {session.isFetching ? 'Checking…' : 'Check again'}
          </Button>
          <BillingLink variant="outline" />
        </div>
      </Section>
    )
  }

  if (waiting) {
    return (
      <Section role="status" aria-live="polite">
        <div className="flex items-center gap-3">
          <LoaderCircleIcon className="size-5 animate-spin text-brand-text" aria-hidden="true" />
          <p className="font-bold">Confirming your payment…</p>
        </div>
        <p className="text-sm text-muted-foreground">
          This usually takes a few seconds. You can leave this page; your plan updates as soon as
          the payment is confirmed.
        </p>
      </Section>
    )
  }

  if (status === 'completed') {
    return (
      <Section role="status">
        <div className="flex items-center gap-3">
          <CircleCheckIcon className="size-6 text-brand-text" aria-hidden="true" />
          <p className="text-lg font-extrabold">Payment confirmed</p>
        </div>
        <p className="text-sm text-muted-foreground">
          Your plan is active. The receipt is in your billing page’s invoices.
        </p>
        <BillingLink />
      </Section>
    )
  }

  return (
    <Section role="alert">
      <div className="flex items-center gap-3">
        <CircleXIcon className="size-6 text-destructive" aria-hidden="true" />
        <p className="text-lg font-extrabold">
          {status === 'expired' ? 'This checkout expired' : 'The payment didn’t go through'}
        </p>
      </div>
      <p className="text-sm text-muted-foreground">
        You haven’t been charged for this checkout. Start again to pay with another card or method.
      </p>
      <div className="flex flex-wrap gap-2">
        <Button asChild>
          <Link to="/billing/plans">Choose a plan</Link>
        </Button>
        <BillingLink variant="outline" />
      </div>
    </Section>
  )
}

function BillingLink({ variant = 'default' }: { variant?: 'default' | 'outline' }) {
  return (
    <Button asChild variant={variant} className="self-start">
      <Link to="/billing">Go to billing</Link>
    </Button>
  )
}
