import type { QueryClient } from '@tanstack/react-query'
import type { ChannelType, InboxSort, InboxTab } from '../types'

export type ListFilter = { tab: InboxTab; q?: string; sort: InboxSort }

/** Every inbox query for one workspace sits under ['inbox', workspaceId]. */
export const inboxKeys = {
  workspace: (workspaceId: string) => ['inbox', workspaceId] as const,
  lists: (workspaceId: string) => ['inbox', workspaceId, 'conversations'] as const,
  list: (workspaceId: string, filter: ListFilter) =>
    ['inbox', workspaceId, 'conversations', filter] as const,
  detail: (workspaceId: string, conversationId: string) =>
    ['inbox', workspaceId, 'conversation', conversationId] as const,
  messages: (workspaceId: string, conversationId: string) =>
    ['inbox', workspaceId, 'messages', conversationId] as const,
  media: (workspaceId: string, messageId: string) =>
    ['inbox', workspaceId, 'media', messageId] as const,
  quickReplies: (workspaceId: string) => ['inbox', workspaceId, 'quick-replies'] as const,
  templates: (workspaceId: string, channelType: ChannelType) =>
    ['inbox', workspaceId, 'templates', channelType] as const,
  mentions: (workspaceId: string) => ['inbox', workspaceId, 'mentions'] as const,
  members: (workspaceId: string) => ['inbox', workspaceId, 'members'] as const,
}

/** Refreshes one chat (its header, window and thread) and the list rows and counts around it. */
export function invalidateConversation(
  queryClient: QueryClient,
  workspaceId: string,
  conversationId: string,
) {
  return Promise.all([
    queryClient.invalidateQueries({ queryKey: inboxKeys.lists(workspaceId) }),
    queryClient.invalidateQueries({ queryKey: inboxKeys.detail(workspaceId, conversationId) }),
    queryClient.invalidateQueries({ queryKey: inboxKeys.messages(workspaceId, conversationId) }),
  ])
}
