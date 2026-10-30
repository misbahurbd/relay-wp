import type { QueryClient } from '@tanstack/react-query'

/** Every integrations query for one workspace sits under ['integrations', workspaceId]. */
export const integrationsKeys = {
  workspace: (workspaceId: string) => ['integrations', workspaceId] as const,
  overview: (workspaceId: string) => ['integrations', workspaceId, 'overview'] as const,
  apiKeys: (workspaceId: string) => ['integrations', workspaceId, 'api-keys'] as const,
  webhooks: (workspaceId: string) => ['integrations', workspaceId, 'webhooks'] as const,
  deliveries: (workspaceId: string, endpointId: string) =>
    ['integrations', workspaceId, 'webhooks', endpointId, 'deliveries'] as const,
  events: () => ['webhook-events'] as const,
}

export function invalidateIntegrations(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: integrationsKeys.workspace(workspaceId) })
}
