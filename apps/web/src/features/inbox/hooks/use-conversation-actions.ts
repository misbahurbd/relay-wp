import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import { useAssign, useSetStatus } from '../api/inbox.mutations'
import type { ConversationDetail, Member } from '../types'

const UNDO_MS = 5_000

/**
 * Assign, snooze, close and reopen one chat (F7), each confirmed by a toast. Close and snooze
 * can be undone for 5 s by setting the previous status again.
 */
export function useConversationActions(
  workspaceId: string,
  conversation: ConversationDetail,
  me: { id: string; name: string },
) {
  const assignMutation = useAssign(workspaceId)
  const statusMutation = useSetStatus(workspaceId)
  const id = conversation.id
  const name = conversation.contact.name || conversation.contact.phone
  const onError = (e: unknown) => toast.error(errorMessage(e))

  const restore = (prev: ConversationDetail['status']) =>
    statusMutation.mutate(
      // Pending only comes from a snooze; undoing one goes back to open.
      { conversationId: id, body: { status: prev === 'closed' ? 'closed' : 'open' } },
      { onError },
    )

  return {
    pending: assignMutation.isPending || statusMutation.isPending,
    assign: (member: Member | null) => {
      if ((conversation.assignee?.id ?? null) === (member?.id ?? null)) return
      assignMutation.mutate(
        { conversationId: id, userId: member?.id ?? null },
        {
          onSuccess: () =>
            toast.success(
              member
                ? `Assigned to ${member.id === me.id ? `${me.name} (you)` : member.name}`
                : 'Chat moved to the unassigned queue',
            ),
          onError,
        },
      )
    },
    snooze: (option: { label: string; until: Date }) => {
      const prev = conversation.status
      statusMutation.mutate(
        { conversationId: id, body: { snooze_until: option.until.toISOString() } },
        {
          onSuccess: () =>
            toast(`Snoozed until ${option.label} · moved to Pending`, {
              duration: UNDO_MS,
              action: { label: 'Undo', onClick: () => restore(prev) },
            }),
          onError,
        },
      )
    },
    close: () => {
      const prev = conversation.status
      statusMutation.mutate(
        { conversationId: id, body: { status: 'closed' } },
        {
          onSuccess: () =>
            toast(`Chat with ${name} closed`, {
              duration: UNDO_MS,
              action: { label: 'Undo', onClick: () => restore(prev) },
            }),
          onError,
        },
      )
    },
    reopen: () =>
      statusMutation.mutate(
        { conversationId: id, body: { status: 'open' } },
        { onSuccess: () => toast.success('Chat reopened'), onError },
      ),
  }
}
