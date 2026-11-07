import { LoaderCircleIcon } from 'lucide-react'
import { useEffect, useEffectEvent, useState } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { rememberCheckout } from './checkout-return'
import { loadCreemEmbed } from './creem-embed'

type Phase = 'opening' | 'open' | 'closed'

/**
 * A provider's hosted checkout shown over Relay instead of a full-page redirect. Completing it
 * only moves the page to the status step, which polls until the webhook confirms the payment.
 * If the provider's script can't load or open, `onUnavailable` lets the caller redirect instead.
 */
export function EmbeddedCheckout({
  url,
  sessionId,
  provider,
  expiresAt,
  onPaid,
  onUnavailable,
  onRestart,
}: {
  url: string
  sessionId: string
  provider: string
  expiresAt: string
  onPaid: (sessionId: string) => void
  onUnavailable: () => void
  onRestart?: () => void
}) {
  const [phase, setPhase] = useState<Phase>('opening')
  const [attempt, setAttempt] = useState(0)
  const paid = useEffectEvent(() => onPaid(sessionId))
  const unavailable = useEffectEvent(onUnavailable)

  // biome-ignore lint/correctness/useExhaustiveDependencies: `attempt` re-opens the window
  useEffect(() => {
    let live = true
    let done = false
    let handle: { close(): void } | undefined
    // If the provider redirects instead, the return page still knows which checkout this was.
    rememberCheckout(sessionId)
    loadCreemEmbed()
      .then((creem) => {
        if (!live) return
        handle = creem.openCheckout({
          checkoutUrl: url,
          theme: document.documentElement.classList.contains('dark') ? 'dark' : 'light',
          onComplete: () => {
            if (!live || done) return
            done = true
            // Closing here cancels the provider's own redirect; the status step takes over.
            creem.close()
            paid()
          },
          onClose: () => {
            if (live && !done) setPhase('closed')
          },
        })
        setPhase('open')
      })
      .catch(() => {
        if (live) unavailable()
      })
    return () => {
      live = false
      try {
        handle?.close()
      } catch {
        // Already closed.
      }
    }
  }, [attempt, url, sessionId])

  if (phase === 'opening') {
    return (
      <p className="flex items-center gap-2 text-sm" role="status">
        <LoaderCircleIcon className="size-4 animate-spin text-brand-text" aria-hidden="true" />
        Opening {provider}’s secure checkout…
      </p>
    )
  }

  if (phase === 'open') {
    return (
      <p className="text-sm text-muted-foreground" role="status">
        Finish paying in the {provider} window. This page updates when the payment is confirmed.
      </p>
    )
  }

  if (Date.parse(expiresAt) <= Date.now()) {
    return (
      <div className="flex flex-col gap-3">
        <Alert variant="info">
          <AlertTitle>This payment window expired</AlertTitle>
          <AlertDescription>
            {onRestart
              ? 'Start again to get a new one.'
              : 'Close this and start the payment again to get a new one.'}
          </AlertDescription>
        </Alert>
        {onRestart && (
          <Button size="lg" onClick={onRestart}>
            Start again
          </Button>
        )}
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-3">
      <Alert variant="info">
        <AlertTitle>Payment window closed</AlertTitle>
        <AlertDescription>You can open it again.</AlertDescription>
      </Alert>
      <Button
        size="lg"
        onClick={() => {
          setPhase('opening')
          setAttempt((n) => n + 1)
        }}
      >
        Open {provider} checkout
      </Button>
    </div>
  )
}
