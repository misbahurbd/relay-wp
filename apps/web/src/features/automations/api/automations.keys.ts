import type { QueryClient } from '@tanstack/react-query'

/** Every automations query for one workspace sits under ['automations', workspaceId]. */
export const automationsKeys = {
  workspace: (workspaceId: string) => ['automations', workspaceId] as const,
  list: (workspaceId: string) => ['automations', workspaceId, 'list'] as const,
}

export function invalidateAutomations(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: automationsKeys.workspace(workspaceId) })
}
