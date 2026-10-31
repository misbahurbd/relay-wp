import { saveAutomation, setAutomationEnabled } from '@relay/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { AutomationSettings, Kind } from '../types'
import { invalidateAutomations } from './automations.keys'

export function useSaveAutomation(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ kind, settings }: { kind: Kind; settings: AutomationSettings }) => {
      await saveAutomation({
        headers: inWorkspace(workspaceId),
        path: { kind },
        body: settings,
        throwOnError: true,
      })
    },
    onSettled: () => invalidateAutomations(queryClient, workspaceId),
  })
}

export function useSetAutomationEnabled(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ kind, enabled }: { kind: Kind; enabled: boolean }) => {
      await setAutomationEnabled({
        headers: inWorkspace(workspaceId),
        path: { kind },
        body: { enabled },
        throwOnError: true,
      })
    },
    onSettled: () => invalidateAutomations(queryClient, workspaceId),
  })
}
