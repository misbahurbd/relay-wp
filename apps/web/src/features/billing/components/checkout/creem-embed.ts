// Creem's checkout can run inside Relay as an overlay (https://docs.creem.io/features/checkout/embedded-checkout).
// Its script is loaded on first use, so pages that never pay with Creem never fetch it.
const SCRIPT_URL = 'https://www.creem.io/embed.js'
const LOAD_TIMEOUT_MS = 10_000

export type CreemCheckoutOptions = {
  checkoutUrl: string
  theme?: 'light' | 'dark'
  onReady?: () => void
  /** The browser's word only. The webhook decides whether the payment happened. */
  onComplete?: () => void
  /** The customer dismissed the overlay. */
  onClose?: () => void
}

export type CreemEmbed = {
  openCheckout(options: CreemCheckoutOptions): { close(): void }
  close(): void
}

declare global {
  interface Window {
    Creem?: CreemEmbed
  }
}

let loading: Promise<CreemEmbed> | undefined

/** Loads embed.js once. A failed load is forgotten, so a later call tries again. */
export function loadCreemEmbed(): Promise<CreemEmbed> {
  if (window.Creem) return Promise.resolve(window.Creem)
  loading ??= new Promise<CreemEmbed>((resolve, reject) => {
    const script = document.createElement('script')
    const fail = (reason: string) => {
      clearTimeout(timer)
      script.remove()
      loading = undefined
      reject(new Error(reason))
    }
    const timer = setTimeout(
      () => fail('Creem’s checkout script took too long to load'),
      LOAD_TIMEOUT_MS,
    )
    script.src = SCRIPT_URL
    script.async = true
    script.onload = () => {
      clearTimeout(timer)
      if (window.Creem) resolve(window.Creem)
      else fail('Creem’s checkout script loaded without its checkout')
    }
    script.onerror = () => fail('Creem’s checkout script didn’t load')
    document.head.append(script)
  })
  return loading
}
