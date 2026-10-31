import type { QueryClient } from '@tanstack/react-query'
import { billingKeys } from '#/features/billing'
import type { Audience, BroadcastFilter, RecipientFilter } from '../types'

/** Every broadcasts query for one workspace sits under ['broadcasts', workspaceId]. */
export const broadcastsKeys = {
  workspace: (workspaceId: string) => ['broadcasts', workspaceId] as const,
  lists: (workspaceId: string) => ['broadcasts', workspaceId, 'list'] as const,
  list: (workspaceId: string, filter: BroadcastFilter = {}) =>
    ['broadcasts', workspaceId, 'list', filter] as const,
  detail: (workspaceId: string, broadcastId: string) =>
    ['broadcasts', workspaceId, 'detail', broadcastId] as const,
  recipients: (workspaceId: string, broadcastId: string, filter?: RecipientFilter) =>
    ['broadcasts', workspaceId, 'detail', broadcastId, 'recipients', filter ?? 'all'] as const,
  audience: (workspaceId: string, a: Audience, numberId?: string) =>
    [
      'broadcasts',
      workspaceId,
      'audience',
      { ...a, tag_ids: [...a.tag_ids].sort(), segment_ids: [...a.segment_ids].sort() },
      numberId ?? '',
    ] as const,
  numbers: (workspaceId: string) => ['broadcasts', workspaceId, 'numbers'] as const,
}

/** Refreshes every list and report; the billing meter moves too when a send starts or ends. */
export function invalidateBroadcasts(queryClient: QueryClient, workspaceId: string) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: broadcastsKeys.workspace(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: billingKeys.monthly(workspaceId) }),
  ])
}
