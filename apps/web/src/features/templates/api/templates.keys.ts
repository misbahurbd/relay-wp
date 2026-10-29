import type { QueryClient } from '@tanstack/react-query'
import type { TemplateFilter } from '../types'

/** Every templates query for one workspace sits under ['templates', workspaceId]. */
export const templatesKeys = {
  workspace: (workspaceId: string) => ['templates', workspaceId] as const,
  lists: (workspaceId: string) => ['templates', workspaceId, 'list'] as const,
  list: (workspaceId: string, filter: TemplateFilter = {}) =>
    ['templates', workspaceId, 'list', filter] as const,
  detail: (workspaceId: string, templateId: string) =>
    ['templates', workspaceId, 'detail', templateId] as const,
}

export function invalidateTemplateLists(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: templatesKeys.lists(workspaceId) })
}
