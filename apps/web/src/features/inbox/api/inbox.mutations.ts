import {
  addInboxNote,
  assignInboxConversation,
  type CreateQuickReplyInputBody,
  createInboxQuickReply,
  deleteInboxQuickReply,
  markInboxMentionsSeen,
  reactInboxMessage,
  readInboxConversation,
  type SendMessageInputBody,
  type SetConversationStatusInputBody,
  sendInboxMessage,
  setInboxConversationStatus,
  type UpdateQuickReplyInputBody,
  updateInboxQuickReply,
  uploadInboxMedia,
} from '@relay/api-client'
import { type InfiniteData, useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { Message, MessagePage, Upload } from '../types'
import { inboxKeys, invalidateConversation } from './inbox.keys'
import { toMessage } from './inbox.queries'

type Sent = { conversationId: string; message: Message }

/** Shows a message the API just stored (still pending) at the bottom of the thread right away. */
function useAppendAndRefresh(workspaceId: string) {
  const queryClient = useQueryClient()
  return ({ conversationId, message }: Sent) => {
    queryClient.setQueryData<InfiniteData<MessagePage, string | undefined>>(
      inboxKeys.messages(workspaceId, conversationId),
      (old) => {
        if (!old || old.pages.length === 0) return old
        const [first, ...rest] = old.pages
        if (first.items.some((m) => m.id === message.id)) return old
        return { ...old, pages: [{ ...first, items: [message, ...first.items] }, ...rest] }
      },
    )
    return invalidateConversation(queryClient, workspaceId, conversationId)
  }
}

export function useSendMessage(workspaceId: string) {
  const refresh = useAppendAndRefresh(workspaceId)
  return useMutation({
    mutationFn: async ({
      conversationId,
      body,
    }: {
      conversationId: string
      body: SendMessageInputBody
    }): Promise<Sent> => {
      const { data } = await sendInboxMessage({
        headers: inWorkspace(workspaceId),
        path: { conversationId },
        body,
        throwOnError: true,
      })
      return { conversationId, message: toMessage(data) }
    },
    onSuccess: refresh,
  })
}

export function useAddNote(workspaceId: string) {
  const refresh = useAppendAndRefresh(workspaceId)
  return useMutation({
    mutationFn: async ({
      conversationId,
      text,
      mentions,
    }: {
      conversationId: string
      text: string
      mentions: string[]
    }): Promise<Sent> => {
      const { data } = await addInboxNote({
        headers: inWorkspace(workspaceId),
        path: { conversationId },
        body: { text, mentions },
        throwOnError: true,
      })
      return { conversationId, message: toMessage(data) }
    },
    onSuccess: refresh,
  })
}

export function useReact(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (v: { conversationId: string; messageId: string; emoji: string }) => {
      await reactInboxMessage({
        headers: inWorkspace(workspaceId),
        path: { conversationId: v.conversationId, messageId: v.messageId },
        body: { emoji: v.emoji },
        throwOnError: true,
      })
      return v.conversationId
    },
    onSuccess: (conversationId) =>
      queryClient.invalidateQueries({ queryKey: inboxKeys.messages(workspaceId, conversationId) }),
  })
}

/** Clears the chat's unread count and sends the read receipt (F9). Failures are harmless. */
export function useMarkRead(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (conversationId: string) => {
      await readInboxConversation({
        headers: inWorkspace(workspaceId),
        path: { conversationId },
        throwOnError: true,
      })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: inboxKeys.lists(workspaceId) }),
  })
}

export function useAssign(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (v: { conversationId: string; userId: string | null }) => {
      await assignInboxConversation({
        headers: inWorkspace(workspaceId),
        path: { conversationId: v.conversationId },
        body: { user_id: v.userId },
        throwOnError: true,
      })
      return v.conversationId
    },
    onSuccess: (id) => invalidateConversation(queryClient, workspaceId, id),
  })
}

export function useSetStatus(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (v: { conversationId: string; body: SetConversationStatusInputBody }) => {
      await setInboxConversationStatus({
        headers: inWorkspace(workspaceId),
        path: { conversationId: v.conversationId },
        body: v.body,
        throwOnError: true,
      })
      return v.conversationId
    },
    onSuccess: (id) => invalidateConversation(queryClient, workspaceId, id),
  })
}

export function useUploadMedia(workspaceId: string) {
  return useMutation({
    mutationFn: async (file: File): Promise<Upload> => {
      const { data } = await uploadInboxMedia({
        headers: inWorkspace(workspaceId),
        body: { file },
        throwOnError: true,
      })
      return data
    },
  })
}

function useQuickReplyRefresh(workspaceId: string) {
  const queryClient = useQueryClient()
  return () => queryClient.invalidateQueries({ queryKey: inboxKeys.quickReplies(workspaceId) })
}

export function useCreateQuickReply(workspaceId: string) {
  const onSuccess = useQuickReplyRefresh(workspaceId)
  return useMutation({
    mutationFn: async (body: CreateQuickReplyInputBody) => {
      const { data } = await createInboxQuickReply({
        headers: inWorkspace(workspaceId),
        body,
        throwOnError: true,
      })
      return data
    },
    onSuccess,
  })
}

export function useUpdateQuickReply(workspaceId: string) {
  const onSuccess = useQuickReplyRefresh(workspaceId)
  return useMutation({
    mutationFn: async ({ id, body }: { id: string; body: UpdateQuickReplyInputBody }) => {
      const { data } = await updateInboxQuickReply({
        headers: inWorkspace(workspaceId),
        path: { quickReplyId: id },
        body,
        throwOnError: true,
      })
      return data
    },
    onSuccess,
  })
}

export function useDeleteQuickReply(workspaceId: string) {
  const onSuccess = useQuickReplyRefresh(workspaceId)
  return useMutation({
    mutationFn: async (id: string) => {
      await deleteInboxQuickReply({
        headers: inWorkspace(workspaceId),
        path: { quickReplyId: id },
        throwOnError: true,
      })
    },
    onSuccess,
  })
}

export function useMarkMentionsSeen(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (messageIds?: string[]) => {
      await markInboxMentionsSeen({
        headers: inWorkspace(workspaceId),
        body: { message_ids: messageIds },
        throwOnError: true,
      })
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: inboxKeys.mentions(workspaceId) }),
  })
}
