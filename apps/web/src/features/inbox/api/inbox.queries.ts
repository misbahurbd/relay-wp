import {
  getInboxConversation,
  getInboxMedia,
  type InboxConversationDetailDto,
  type InboxConversationDto,
  type InboxMessageDto,
  listInboxConversations,
  listInboxMentions,
  listInboxMessages,
  listInboxQuickReplies,
  listInboxTemplates,
  listMembers,
} from '@relay/api-client'
import { infiniteQueryOptions, keepPreviousData, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  ChannelType,
  Conversation,
  ConversationDetail,
  ConversationPage,
  InboxTemplate,
  Member,
  Mention,
  Message,
  MessagePage,
  QuickReply,
} from '../types'
import { inboxKeys, type ListFilter } from './inbox.keys'

/** While the live stream is down, lists and threads poll at this pace instead (AD-8). */
export const POLL_MS = 15_000

function toConversation<T extends InboxConversationDto | InboxConversationDetailDto>(c: T) {
  return { ...c, assignee: c.assignee ?? null, team_id: c.team_id ?? null }
}

/** Fills the fields the API sends as null, so the thread never checks them twice. */
export function toMessage(m: InboxMessageDto): Message {
  const content = (m.content ?? {}) as Message['content']
  return {
    ...m,
    author: m.author ?? null,
    reply_to: m.reply_to ?? null,
    mentions: m.mentions ?? [],
    content,
    reactions: { in: m.reactions?.in ?? '', out: m.reactions?.out ?? '' },
  }
}

export const conversationsQueryOptions = (workspaceId: string, filter: ListFilter, poll = false) =>
  infiniteQueryOptions({
    queryKey: inboxKeys.list(workspaceId, filter),
    queryFn: async ({ pageParam, signal }): Promise<ConversationPage> => {
      const { data } = await listInboxConversations({
        headers: inWorkspace(workspaceId),
        query: { ...filter, q: filter.q || undefined, cursor: pageParam },
        signal,
        throwOnError: true,
      })
      return {
        items: (data.items ?? []).map((c): Conversation => toConversation(c)),
        next: data.next_cursor ?? null,
        counts: data.counts,
      }
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next ?? undefined,
    placeholderData: keepPreviousData,
    refetchInterval: poll ? POLL_MS : false,
  })

export const conversationQueryOptions = (
  workspaceId: string,
  conversationId: string,
  poll = false,
) =>
  queryOptions({
    queryKey: inboxKeys.detail(workspaceId, conversationId),
    queryFn: async ({ signal }): Promise<ConversationDetail> => {
      const { data } = await getInboxConversation({
        headers: inWorkspace(workspaceId),
        path: { conversationId },
        signal,
        throwOnError: true,
      })
      const media = (data.capabilities.media ?? []).map((m) => ({
        ...m,
        mime_types: m.mime_types ?? [],
      }))
      return { ...toConversation(data), capabilities: { ...data.capabilities, media } }
    },
    refetchInterval: poll ? POLL_MS : false,
  })

/** The thread, newest page first; each page holds up to 50 messages, newest first. */
export const messagesQueryOptions = (workspaceId: string, conversationId: string, poll = false) =>
  infiniteQueryOptions({
    queryKey: inboxKeys.messages(workspaceId, conversationId),
    queryFn: async ({ pageParam, signal }): Promise<MessagePage> => {
      const { data } = await listInboxMessages({
        headers: inWorkspace(workspaceId),
        path: { conversationId },
        query: { before: pageParam },
        signal,
        throwOnError: true,
      })
      return { items: (data.items ?? []).map(toMessage), before: data.next_before || null }
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.before ?? undefined,
    refetchInterval: poll ? POLL_MS : false,
  })

/** A message's file as a Blob, fetched through the API (never an S3 or Meta URL, AD-11). */
export const mediaQueryOptions = (workspaceId: string, conversationId: string, messageId: string) =>
  queryOptions({
    queryKey: inboxKeys.media(workspaceId, messageId),
    queryFn: async ({ signal }): Promise<Blob> => {
      const { data } = await getInboxMedia({
        headers: inWorkspace(workspaceId),
        path: { conversationId, messageId },
        parseAs: 'blob',
        signal,
        throwOnError: true,
      })
      return data as Blob
    },
    // A stored file never changes; keep it while the chat is open nearby.
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: 5 * 60_000,
  })

export const quickRepliesQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: inboxKeys.quickReplies(workspaceId),
    queryFn: async ({ signal }): Promise<QuickReply[]> => {
      const { data } = await listInboxQuickReplies({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data.items ?? []
    },
    staleTime: 60_000,
  })

/** Approved templates for one channel type; 409 templates_unsupported when it has none. */
export const templatesQueryOptions = (workspaceId: string, channelType: ChannelType) =>
  queryOptions({
    queryKey: inboxKeys.templates(workspaceId, channelType),
    queryFn: async ({ signal }): Promise<InboxTemplate[]> => {
      const { data } = await listInboxTemplates({
        headers: inWorkspace(workspaceId),
        query: { channel_type: channelType },
        signal,
        throwOnError: true,
      })
      return (data.items ?? []).map((t) => ({
        ...t,
        buttons: t.buttons ?? [],
        url_buttons: t.url_buttons ?? [],
      }))
    },
    staleTime: 60_000,
  })

export const mentionsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: inboxKeys.mentions(workspaceId),
    queryFn: async ({ signal }): Promise<{ items: Mention[]; unseen: number }> => {
      const { data } = await listInboxMentions({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return {
        items: (data.items ?? []).map((m) => ({ ...m, author: m.author ?? null })),
        unseen: data.unseen,
      }
    },
  })

/** Active members, for "Assign to" and @mentions (first 100, by the API's order). */
export const membersQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: inboxKeys.members(workspaceId),
    queryFn: async ({ signal }): Promise<Member[]> => {
      const { data } = await listMembers({
        headers: inWorkspace(workspaceId),
        query: { status: 'active', page_size: 100 },
        signal,
        throwOnError: true,
      })
      return (data.members ?? []).map((m) => ({ id: m.user_id, name: m.name, role: m.role_key }))
    },
    staleTime: 5 * 60_000,
  })
