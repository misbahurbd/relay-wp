import {
  forgotPasswordMutation,
  getMeOptions,
  getMeQueryKey,
  listOauthProvidersOptions,
  loginMutation,
  logoutMutation,
  oauthCompleteMutation,
  oauthCompleteVerifyMutation,
  resendVerificationMutation,
  resetPasswordMutation,
  signupMutation,
  verifyEmailCodeMutation,
  verifyEmailMutation,
} from '@relay/api-client'
import { type QueryClient, useMutation, useQueryClient } from '@tanstack/react-query'
import { env } from '#/lib/env'

// No retries: a 401 just means signed out, and retrying would slow every page change.
export const meQueryOptions = () => ({ ...getMeOptions(), retry: false })
export const providersQueryOptions = () => ({ ...listOauthProvidersOptions(), staleTime: Infinity })

export type SocialProvider = 'google' | 'facebook' | 'apple'
const PROVIDER_ORDER: SocialProvider[] = ['google', 'facebook', 'apple']

export function sortProviders(providers: string[] | null | undefined): SocialProvider[] {
  return PROVIDER_ORDER.filter((p) => providers?.includes(p))
}

// Social sign-in is a full-page redirect through the API, not a fetch. It comes
// back to returnPath (a page of this app), with ?error= or ?complete_token= when
// the sign-in needs the person again.
export function oauthStartUrl(provider: SocialProvider, returnPath = '/login') {
  const params = new URLSearchParams({
    client: 'web',
    redirect_to: new URL(returnPath, window.location.origin).toString(),
    timezone: Intl.DateTimeFormat().resolvedOptions().timeZone,
  })
  return `${env.VITE_API_URL}/v1/auth/oauth/${provider}/start?${params}`
}

/** Null when signed out; throws on other errors. */
export async function fetchMe(queryClient: QueryClient) {
  try {
    return await queryClient.ensureQueryData(meQueryOptions())
  } catch (error) {
    if (typeof error === 'object' && error && 'code' in error) return null
    throw error
  }
}

export function useSignup() {
  return useMutation(signupMutation())
}

export function useLogin() {
  const queryClient = useQueryClient()
  return useMutation({
    ...loginMutation(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getMeQueryKey() }),
  })
}

export function useLogout() {
  const queryClient = useQueryClient()
  return useMutation({
    ...logoutMutation(),
    onSettled: () => queryClient.clear(),
  })
}

/** The right code verifies the email and signs the user in, like login. */
export function useVerifyEmailCode() {
  const queryClient = useQueryClient()
  return useMutation({
    ...verifyEmailCodeMutation(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getMeQueryKey() }),
  })
}

export function useVerifyEmailLink() {
  return useMutation(verifyEmailMutation())
}

export function useResendVerification() {
  return useMutation(resendVerificationMutation())
}

export function useForgotPassword() {
  return useMutation(forgotPasswordMutation())
}

export function useResetPassword() {
  return useMutation(resetPasswordMutation())
}

/** Social sign-in without a provider-verified email, step 1: email a code to the address. */
export function useOauthComplete() {
  return useMutation(oauthCompleteMutation())
}

/** Step 2: the right code creates (or links) the account and signs in. */
export function useOauthCompleteVerify() {
  const queryClient = useQueryClient()
  return useMutation({
    ...oauthCompleteVerifyMutation(),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: getMeQueryKey() }),
  })
}

// The sign-up code must come with the password chosen at sign-up (anyone can sign
// up with someone else's address). Sign-up and login keep the password they just
// sent here, in memory only (never storage or the URL), so the code page doesn't
// ask again. A reload loses it, and the page asks.
let pending: { email: string; password: string } | null = null

export function rememberSignupPassword(email: string, password: string) {
  pending = { email: email.toLowerCase(), password }
}

export function rememberedSignupPassword(email: string) {
  return pending?.email === email.toLowerCase() ? pending.password : undefined
}

export function forgetSignupPassword() {
  pending = null
}
