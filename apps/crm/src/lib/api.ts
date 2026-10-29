import type { ErrorBody } from '@relay/api-client'
import { client, refreshToken } from '@relay/api-client'
import { env } from './env'

// Cookies carry the session for the web app; the API also accepts Bearer tokens for other clients.
client.setConfig({
  baseUrl: env.VITE_API_URL,
  credentials: 'include',
})

// The access cookie lasts 15 minutes. On a 401, refresh once and replay the request.
// Concurrent 401s share one refresh: the API rotates the refresh token and treats reuse as theft.
const NO_RETRY = ['/v1/auth/login', '/v1/auth/signup', '/v1/auth/token/refresh', '/v1/auth/logout']
const replays = new WeakMap<Request, Request>()
let refreshing: Promise<boolean> | null = null

function refreshSession() {
  refreshing ??= refreshToken({ client })
    .then((r) => r.response?.ok ?? false)
    .catch(() => false)
    .finally(() => {
      refreshing = null
    })
  return refreshing
}

client.interceptors.request.use((request) => {
  replays.set(request, request.clone())
  return request
})

client.interceptors.response.use(async (response, request) => {
  if (response.status !== 401) return response
  if (NO_RETRY.some((path) => new URL(request.url).pathname === path)) return response
  const replay = replays.get(request)
  if (!replay || !(await refreshSession())) return response
  return fetch(replay)
})

export function isApiError(error: unknown): error is ErrorBody {
  return typeof error === 'object' && error !== null && 'code' in error && 'message' in error
}

// The API's message when it sent one, otherwise a generic fallback. Never shows raw exceptions.
export function errorMessage(error: unknown, fallback = 'Something went wrong. Please try again.') {
  return isApiError(error) ? error.message : fallback
}

export { client }
