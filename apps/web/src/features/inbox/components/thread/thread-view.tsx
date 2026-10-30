import { useQuery } from '@tanstack/react-query'
import { ArrowLeftIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Sheet, SheetContent, SheetTitle } from '#/components/ui/sheet'
import { Skeleton } from '#/components/ui/skeleton'
import { useAccess } from '#/features/team'
import { errorMessage, isApiError } from '#/lib/api'
import { useMarkRead } from '../../api/inbox.mutations'
import { conversationQueryOptions, membersQueryOptions } from '../../api/inbox.queries'
import { useLiveConnected } from '../../hooks/use-live-status'
import { useNow } from '../../hooks/use-now'
import { CHANNEL_COPY } from '../../schemas/inbox.schema'
import type { Message } from '../../types'
import { Composer } from '../composer/composer'
import { windowState } from '../kit/window'
import { CustomerPanel } from '../panel/customer-panel'
import { TemplateDialog } from '../templates/template-dialog'
import { MessageList } from './message-list'
import { ThreadHeader } from './thread-header'
import { WindowBar } from './window-bar'

/** The middle column for one chat, plus its customer panel (inline on wide screens, a sheet below). */
export function ThreadView({
  workspaceId,
  conversationId,
  me,
  onBack,
}: {
  workspaceId: string
  conversationId: string
  me: { id: string; name: string }
  onBack: () => void
}) {
  const live = useLiveConnected()
  const now = useNow()
  const { can } = useAccess(workspaceId)
  const detail = useQuery(conversationQueryOptions(workspaceId, conversationId, !live))
  const { data: members = [] } = useQuery(membersQueryOptions(workspaceId))
  const [replyTo, setReplyTo] = useState<Message | null>(null)
  const [templateOpen, setTemplateOpen] = useState(false)
  const [detailsOpen, setDetailsOpen] = useState(false)
  const { mutate: read } = useMarkRead(workspaceId)

  // Opening a chat clears its unread count for everyone (F9); so does a new message while open.
  const lastRead = useRef('')
  const c = detail.data
  useEffect(() => {
    if (!c || c.unread_count === 0) return
    const key = `${c.id}|${c.last_message_at}`
    if (lastRead.current === key) return
    lastRead.current = key
    read(c.id)
  }, [c, read])

  if (detail.isPending) return <ThreadSkeleton />
  if (detail.isError) {
    const gone = isApiError(detail.error) && detail.error.code === 'conversation_not_found'
    return (
      <section className="flex flex-1 flex-col gap-4 bg-chat p-6">
        <Button variant="ghost" size="sm" className="self-start md:hidden" onClick={onBack}>
          <ArrowLeftIcon aria-hidden="true" />
          Back to chats
        </Button>
        <Alert variant={gone ? 'info' : 'danger'}>
          <AlertDescription>
            {gone
              ? 'This chat isn’t available. It may have moved to a team you can’t see.'
              : errorMessage(detail.error, 'We couldn’t load this chat.')}
          </AlertDescription>
        </Alert>
      </section>
    )
  }

  const conversation = detail.data
  const win = windowState(conversation, now)
  const name = conversation.contact.name || conversation.contact.phone
  const panel = (className: string) => (
    <CustomerPanel
      workspaceId={workspaceId}
      conversation={conversation}
      me={me}
      members={members}
      className={className}
    />
  )

  return (
    <>
      <section
        aria-label={`Conversation with ${name}`}
        className="flex min-h-0 min-w-0 flex-1 flex-col bg-chat"
      >
        <ThreadHeader
          workspaceId={workspaceId}
          conversation={conversation}
          me={me}
          members={members}
          now={now}
          onBack={onBack}
          onShowDetails={() => setDetailsOpen(true)}
        />
        <WindowBar state={win} />
        <MessageList
          workspaceId={workspaceId}
          conversationId={conversationId}
          customerName={name}
          members={members}
          meId={me.id}
          now={now}
          canReply={can('inbox.reply') && win.open}
          canReact={conversation.capabilities.reactions}
          metaPricing={CHANNEL_COPY[conversation.channel.type].metaPricing}
          windowHours={conversation.capabilities.window_hours}
          onReply={setReplyTo}
        />
        <Composer
          key={conversationId}
          workspaceId={workspaceId}
          conversation={conversation}
          window={win}
          me={me}
          members={members}
          replyTo={win.open ? replyTo : null}
          onClearReply={() => setReplyTo(null)}
          onOpenTemplate={() => setTemplateOpen(true)}
        />
      </section>

      {panel('hidden w-[300px] flex-none border-l xl:flex')}
      <Sheet open={detailsOpen} onOpenChange={setDetailsOpen}>
        <SheetContent side="right" className="w-[320px] max-w-[90vw] gap-0 p-0 xl:hidden">
          <SheetTitle className="sr-only">Customer details</SheetTitle>
          {panel('flex h-full pt-12')}
        </SheetContent>
      </Sheet>

      {can('inbox.send_template') && conversation.capabilities.templates && (
        <TemplateDialog
          workspaceId={workspaceId}
          conversationId={conversationId}
          channelType={conversation.channel.type}
          name={name}
          window={win}
          open={templateOpen}
          onOpenChange={setTemplateOpen}
        />
      )}
    </>
  )
}

function ThreadSkeleton() {
  return (
    <section role="status" aria-label="Loading chat" className="flex flex-1 flex-col bg-chat">
      <div className="flex items-center gap-3 border-b bg-card px-[18px] py-3">
        <Skeleton className="size-10 rounded-full" />
        <div className="flex flex-col gap-1.5">
          <Skeleton className="h-4 w-36" />
          <Skeleton className="h-3 w-28" />
        </div>
      </div>
      <Skeleton className="h-10 w-full rounded-none" />
      <div className="flex flex-1 flex-col justify-end gap-3 p-6">
        <Skeleton className="h-12 w-2/3 self-start rounded-xl" />
        <Skeleton className="h-16 w-1/2 self-end rounded-xl" />
      </div>
    </section>
  )
}
