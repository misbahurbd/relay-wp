import {
  adminGetCatalog,
  type ErrorBody,
  getMe,
  login,
  logout,
  type MeOutputBody,
} from '@relay/api-client'
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { isApiError } from '#/lib/api'

export const sessionKeys = { me: () => ['session', 'me'] as const }

/** The signed-in account, or null when signed out. */
export function meQueryOptions() {
  return queryOptions({
    queryKey: sessionKeys.me(),
    queryFn: async (): Promise<MeOutputBody | null> => {
      try {
        const { data } = await getMe({ throwOnError: true })
        return data
      } catch (error) {
        if (isApiError(error)) return null
        throw error
      }
    },
    retry: false,
  })
}

const SESSION_NOT_KEPT: ErrorBody = {
  code: 'session_not_kept',
  message:
    'Signed in, but this browser didn’t keep the session. Open the CRM on the same scheme and site as VITE_API_URL.',
}

const NOT_STAFF: ErrorBody = {
  code: 'staff_only',
  message: 'This account isn’t Relay staff. Ask an admin to add your email to ADMIN_EMAILS.',
}

/**
 * Signs in, then checks the session stuck and the account may use /v1/admin. A non-staff account
 * is signed straight out, so the form shows why instead of an admin page it can't use.
 */
export function useSignIn() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: { email: string; password: string }) => {
      await login({ body: { ...body, client: 'web' }, throwOnError: true })
      const me = await getMe()
      if (!me.response?.ok) throw SESSION_NOT_KEPT
      const admin = await adminGetCatalog()
      if (isApiError(admin.error) && admin.error.code === 'staff_only') {
        await logout().catch(() => undefined)
        throw NOT_STAFF
      }
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: sessionKeys.me() }),
  })
}

export function useSignOut() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: () => logout({ throwOnError: true }),
    // Everything cached belonged to the account that left.
    onSettled: () => queryClient.resetQueries(),
  })
}
