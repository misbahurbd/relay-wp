/**
 * Meta's WhatsApp Embedded Signup (docs/PHASE-3-ONBOARDING.md, "Connect a number"). Facebook's
 * JS SDK opens Meta's window; it posts WA_EMBEDDED_SIGNUP messages with the WABA and phone
 * number IDs, and FB.login's callback returns a code the API exchanges for a token.
 */

type LoginResponse = { authResponse?: { code?: string } | null }

type FacebookSdk = {
  init(options: { appId: string; autoLogAppEvents: boolean; xfbml: boolean; version: string }): void
  login(callback: (response: LoginResponse) => void, options: Record<string, unknown>): void
}

declare global {
  interface Window {
    FB?: FacebookSdk
    fbAsyncInit?: () => void
  }
}

let sdk: Promise<FacebookSdk> | null = null

/** Loads the SDK once. Load it before the click: FB.login must run inside the click to open. */
export function loadFacebookSdk(appId: string, version: string): Promise<FacebookSdk> {
  if (sdk) return sdk
  sdk = new Promise((resolve, reject) => {
    window.fbAsyncInit = () => {
      const fb = window.FB
      if (!fb) return reject(new Error('Facebook SDK did not load'))
      fb.init({ appId, autoLogAppEvents: true, xfbml: false, version })
      resolve(fb)
    }
    const script = document.createElement('script')
    script.src = 'https://connect.facebook.net/en_US/sdk.js'
    script.async = true
    script.crossOrigin = 'anonymous'
    script.onerror = () => {
      sdk = null
      reject(new Error('Facebook SDK did not load'))
    }
    document.body.appendChild(script)
  })
  return sdk
}

export type SignupResult =
  | { status: 'finished'; code: string; waba_id: string; phone_number_id: string }
  | { status: 'cancelled'; step?: string }

type SignupMessage = {
  type?: string
  event?: string
  data?: { waba_id?: string; phone_number_id?: string; current_step?: string }
}

function fromFacebook(origin: string) {
  try {
    return /(^|\.)facebook\.com$/.test(new URL(origin).hostname)
  } catch {
    return false
  }
}

/** Opens Meta's window. Call it straight from a click handler. */
export function launchEmbeddedSignup(
  fb: FacebookSdk,
  configId: string,
  options: { coexistence?: boolean } = {},
): Promise<SignupResult> {
  return new Promise((resolve) => {
    let code: string | undefined
    let ids: { waba_id: string; phone_number_id: string } | undefined
    let cancelledAt: string | undefined
    let settled = false

    const settle = (result: SignupResult) => {
      if (settled) return
      settled = true
      window.removeEventListener('message', onMessage)
      resolve(result)
    }
    const tryFinish = () => {
      if (code && ids) settle({ status: 'finished', code, ...ids })
    }
    function onMessage(event: MessageEvent) {
      if (!fromFacebook(event.origin)) return
      let msg: SignupMessage
      try {
        msg = typeof event.data === 'string' ? JSON.parse(event.data) : event.data
      } catch {
        return
      }
      if (msg?.type !== 'WA_EMBEDDED_SIGNUP') return
      const finished =
        msg.event === 'FINISH' || msg.event === 'FINISH_WHATSAPP_BUSINESS_APP_ONBOARDING'
      if (finished && msg.data?.waba_id && msg.data.phone_number_id) {
        ids = { waba_id: msg.data.waba_id, phone_number_id: msg.data.phone_number_id }
        tryFinish()
      } else if (msg.event === 'CANCEL') {
        cancelledAt = msg.data?.current_step
      }
    }
    window.addEventListener('message', onMessage)

    fb.login(
      (response) => {
        code = response.authResponse?.code
        if (!code) return settle({ status: 'cancelled', step: cancelledAt })
        tryFinish()
        // The IDs message normally arrives first; don't wait forever if it never does.
        setTimeout(() => settle({ status: 'cancelled', step: cancelledAt }), 10_000)
      },
      {
        config_id: configId,
        response_type: 'code',
        override_default_response_type: true,
        extras: {
          setup: {},
          sessionInfoVersion: '3',
          ...(options.coexistence ? { featureType: 'whatsapp_business_app_onboarding' } : {}),
        },
      },
    )
  })
}
