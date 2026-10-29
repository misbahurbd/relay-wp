// A hosted checkout returns to a URL we set before we know the session ID, so the ID waits in
// sessionStorage for the return. Embedded checkouts put it in the URL themselves.
const KEY = 'relay.billing.checkout'

/** Where Stripe 3-D Secure or a hosted checkout returns: the checkout page in status mode. */
export function checkoutReturnUrl(sessionId?: string) {
  const url = new URL('/billing/checkout', window.location.origin)
  url.searchParams.set('session', sessionId ?? 'return')
  return url.toString()
}

export function rememberCheckout(sessionId: string) {
  try {
    sessionStorage.setItem(KEY, sessionId)
  } catch {
    // Storage can be off; the return page then asks the member to check billing.
  }
}

/** The session the last hosted checkout started in this tab, if any. */
export function rememberedCheckout() {
  try {
    return sessionStorage.getItem(KEY) ?? undefined
  } catch {
    return undefined
  }
}
