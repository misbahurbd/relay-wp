import { getMe, login, logout, type MeOutputBody } from '@relay/api-client'
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

export function useSignIn() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: (body: { email: string; password: string }) =>
      login({ body: { ...body, client: 'web' }, throwOnError: true }),
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
