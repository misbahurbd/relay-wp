import type { QueryClient } from '@tanstack/react-query'
import type { ContactFilter } from '../types'

/** Every contacts query for one workspace sits under ['contacts', workspaceId]. */
export const contactsKeys = {
  workspace: (workspaceId: string) => ['contacts', workspaceId] as const,
  lists: (workspaceId: string) => ['contacts', workspaceId, 'list'] as const,
  list: (workspaceId: string, filter: ContactFilter, cursor?: string) =>
    ['contacts', workspaceId, 'list', filter, cursor ?? null] as const,
  summary: (workspaceId: string) => ['contacts', workspaceId, 'summary'] as const,
  detail: (workspaceId: string, contactId: string) =>
    ['contacts', workspaceId, 'detail', contactId] as const,
  tags: (workspaceId: string) => ['contacts', workspaceId, 'tags'] as const,
  agents: (workspaceId: string) => ['contacts', workspaceId, 'agents'] as const,
  import: (workspaceId: string, importId: string) =>
    ['contacts', workspaceId, 'import', importId] as const,
  segments: (workspaceId: string) => ['contacts', workspaceId, 'segments'] as const,
  segmentPreview: (workspaceId: string, rules: unknown) =>
    ['contacts', workspaceId, 'segments', 'preview', rules] as const,
}

/** Refreshes lists, counts, details and tags after a change. */
export function invalidateContacts(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: contactsKeys.workspace(workspaceId) })
}
