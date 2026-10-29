import {
  completeOnboarding,
  connectWhatsapp,
  getOnboarding,
  getTestMessage,
  listWhatsappNumbers,
  type OnboardingDto,
  saveBusiness,
  sendTestMessage,
  skipWhatsapp,
  updateOnboardingMember,
  whatsappSignupConfig,
} from '@relay/api-client'
import { queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { toWorkspace, workspaceKeys } from '#/features/workspaces'
import { inWorkspace } from '#/lib/api'
import type { BusinessInput, NumberType, Onboarding, WhatsAppNumber } from '../types'

export const onboardingKeys = {
  progress: (userId: string, workspaceId: string) => ['onboarding', userId, workspaceId] as const,
  numbers: (workspaceId: string) => ['whatsapp-numbers', workspaceId] as const,
  testMessage: (workspaceId: string, messageId: string) =>
    ['whatsapp-test-message', workspaceId, messageId] as const,
  signupConfig: () => ['whatsapp-signup-config'] as const,
}

function toOnboarding(data: OnboardingDto): Onboarding {
  return { ...data, steps: data.steps ?? [], checklist: data.checklist ?? [] }
}

export const onboardingQueryOptions = (userId: string, workspaceId: string) =>
  queryOptions({
    queryKey: onboardingKeys.progress(userId, workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await getOnboarding({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return toOnboarding(data)
    },
  })

/** Polls every 2 s while a number is being set up, as the spec's "Setting up" screen does. */
export const numbersQueryOptions = (_userId: string, workspaceId: string) =>
  queryOptions({
    queryKey: onboardingKeys.numbers(workspaceId),
    queryFn: async ({ signal }): Promise<WhatsAppNumber[]> => {
      const { data } = await listWhatsappNumbers({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data.numbers ?? []
    },
    refetchInterval: (q) => (q.state.data?.some((n) => n.status === 'setting_up') ? 2000 : false),
  })

export const testMessageQueryOptions = (_userId: string, workspaceId: string, messageId: string) =>
  queryOptions({
    queryKey: onboardingKeys.testMessage(workspaceId, messageId),
    queryFn: async ({ signal }) => {
      const { data } = await getTestMessage({
        headers: inWorkspace(workspaceId),
        path: { messageId },
        signal,
        throwOnError: true,
      })
      return data
    },
    refetchInterval: (q) =>
      q.state.data?.status === 'delivered' ||
      q.state.data?.status === 'read' ||
      q.state.data?.status === 'failed'
        ? false
        : 1500,
  })

export const signupConfigQueryOptions = () =>
  queryOptions({
    queryKey: onboardingKeys.signupConfig(),
    queryFn: async ({ signal }) => {
      const { data } = await whatsappSignupConfig({ signal, throwOnError: true })
      return data
    },
    staleTime: Infinity,
  })

/** Writes the fresh progress into the cache and refreshes the workspace list's summary. */
function useOnboardingMutation<TInput>(
  userId: string,
  workspaceId: string,
  call: (input: TInput) => Promise<OnboardingDto>,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: TInput) => toOnboarding(await call(input)),
    onSuccess: (progress) => {
      queryClient.setQueryData(onboardingKeys.progress(userId, workspaceId), progress)
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.all(userId) })
    },
  })
}

export function useSaveBusiness(userId: string, workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: BusinessInput) => {
      const { data } = await saveBusiness({
        headers: inWorkspace(workspaceId),
        body: input,
        throwOnError: true,
      })
      return toWorkspace(data)
    },
    onSuccess: (workspace) => {
      queryClient.setQueryData(workspaceKeys.detail(userId, workspaceId), workspace)
      return Promise.all([
        queryClient.invalidateQueries({ queryKey: onboardingKeys.progress(userId, workspaceId) }),
        queryClient.invalidateQueries({ queryKey: workspaceKeys.all(userId), exact: true }),
      ])
    },
  })
}

export function useSkipWhatsApp(userId: string, workspaceId: string) {
  return useOnboardingMutation(userId, workspaceId, async () => {
    const { data } = await skipWhatsapp({ headers: inWorkspace(workspaceId), throwOnError: true })
    return data
  })
}

export function useCompleteOnboarding(userId: string, workspaceId: string) {
  return useOnboardingMutation(userId, workspaceId, async () => {
    const { data } = await completeOnboarding({
      headers: inWorkspace(workspaceId),
      throwOnError: true,
    })
    return data
  })
}

export function useUpdateMemberState(userId: string, workspaceId: string) {
  return useOnboardingMutation(
    userId,
    workspaceId,
    async (input: { tour_seen?: boolean; checklist_hidden?: boolean }) => {
      const { data } = await updateOnboardingMember({
        headers: inWorkspace(workspaceId),
        body: input,
        throwOnError: true,
      })
      return data
    },
  )
}

export function useConnectWhatsApp(userId: string, workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: {
      code: string
      waba_id: string
      phone_number_id: string
      number_type: NumberType
    }) => {
      const { data } = await connectWhatsapp({
        headers: inWorkspace(workspaceId),
        body: input,
        throwOnError: true,
      })
      return data
    },
    onSuccess: () =>
      Promise.all([
        queryClient.invalidateQueries({ queryKey: onboardingKeys.numbers(workspaceId) }),
        queryClient.invalidateQueries({ queryKey: onboardingKeys.progress(userId, workspaceId) }),
      ]),
  })
}

export function useSendTestMessage(_userId: string, workspaceId: string) {
  return useMutation({
    mutationFn: async (input: { numberId: string; to: string }) => {
      const { data } = await sendTestMessage({
        headers: inWorkspace(workspaceId),
        path: { numberId: input.numberId },
        body: { to: input.to },
        throwOnError: true,
      })
      return data
    },
  })
}

export function useRefreshProgress(userId: string, workspaceId: string) {
  const queryClient = useQueryClient()
  return () =>
    queryClient.invalidateQueries({ queryKey: onboardingKeys.progress(userId, workspaceId) })
}
