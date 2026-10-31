import { useInfiniteQuery } from '@tanstack/react-query'
import { Loader2Icon } from 'lucide-react'
import { Fragment, useEffect, useRef, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { messagesQueryOptions } from '../../api/inbox.queries'
import { useLiveConnected } from '../../hooks/use-live-status'
import type { Member, Message } from '../../types'
import { firstName, formatDay, sameDay } from '../kit/format'
import { messageSummaryText } from '../kit/summary'
import { MessageBubble } from './message-bubble'
import { EventLine, eventText, NoteBubble } from './system-line'

/**
 * The thread (F3): oldest at the top, newest at the bottom. The scroller is column-reverse, so
 * it opens at the newest message and loading older ones above doesn't move what you're reading.
 */
export function MessageList({
  workspaceId,
  conversationId,
  customerName,
  members,
  meId,
  now,
  canReply,
  canReact,
  metaPricing,
  windowHours,
  onReply,
}: {
  workspaceId: string
  conversationId: string
  customerName: string
  members: Member[]
  meId: string
  now: number
  canReply: boolean
  /** The channel takes reactions (capabilities.reactions). */
  canReact: boolean
  /** Show Meta's template pricing hints (WhatsApp only). */
  metaPricing: boolean
  /** The channel's window length, for "window reopened" lines. */
  windowHours: number
  onReply: (m: Message) => void
}) {
  const live = useLiveConnected()
  const query = useInfiniteQuery(messagesQueryOptions(workspaceId, conversationId, !live))
  const messages = (query.data?.pages.flatMap((p) => p.items) ?? []).slice().reverse()

  // Older messages load when the top of the thread scrolls into view.
  const scroller = useRef<HTMLDivElement>(null)
  const sentinel = useRef<HTMLDivElement>(null)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  useEffect(() => {
    const el = sentinel.current
    if (!el || !hasNextPage) return
    const io = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && !isFetchingNextPage) void fetchNextPage()
      },
      { root: scroller.current, rootMargin: '200px 0px 0px 0px' },
    )
    io.observe(el)
    return () => io.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  // Screen readers hear new customer messages; the first load stays quiet.
  const newest = messages.at(-1)
  const seen = useRef<string | null>(null)
  const [announcement, setAnnouncement] = useState('')
  useEffect(() => {
    if (!newest) return
    if (
      seen.current &&
      seen.current !== newest.id &&
      newest.direction === 'in' &&
      newest.kind === 'message'
    ) {
      setAnnouncement(`New message from ${firstName(customerName)}: ${messageSummaryText(newest)}`)
    }
    seen.current = newest.id
  }, [newest, customerName])

  return (
    <div ref={scroller} className="flex min-h-0 flex-1 flex-col-reverse overflow-y-auto bg-chat">
      <div className="flex flex-col gap-2.5 px-3 py-4 sm:px-6">
        <div ref={sentinel} aria-hidden="true" className="h-px" />
        {isFetchingNextPage && (
          <Loader2Icon
            className="mx-auto size-5 animate-spin text-muted-foreground"
            aria-label="Loading older messages"
          />
        )}
        {query.isPending ? (
          <ThreadSkeleton />
        ) : query.isError ? (
          <Alert variant="danger">
            <AlertDescription>
              {errorMessage(query.error, 'We couldn’t load this chat.')}{' '}
              <Button
                variant="unstyled"
                type="button"
                className="font-bold underline"
                onClick={() => query.refetch()}
              >
                Try again
              </Button>
            </AlertDescription>
          </Alert>
        ) : messages.length === 0 ? (
          <p className="my-8 text-center text-muted-foreground">No messages yet.</p>
        ) : (
          messages.map((m, i) => {
            const prev = messages[i - 1]
            const showDay = !prev || !sameDay(prev.created_at, m.created_at)
            return (
              <Fragment key={m.id}>
                {showDay && (
                  <div className="self-center rounded-full bg-card px-3 py-1 text-xs font-bold text-muted-foreground shadow-card">
                    {formatDay(m.created_at, now)}
                  </div>
                )}
                {m.kind === 'event' ? (
                  <EventLine text={eventText(m, members, meId, now, windowHours)} />
                ) : m.kind === 'note' ? (
                  <NoteBubble message={m} members={members} />
                ) : (
                  <MessageBubble
                    workspaceId={workspaceId}
                    conversationId={conversationId}
                    message={m}
                    customerName={customerName}
                    canReply={canReply}
                    canReact={canReact}
                    metaPricing={metaPricing}
                    onReply={onReply}
                  />
                )}
              </Fragment>
            )
          })
        )}
        <div aria-live="polite" className="sr-only">
          {announcement}
        </div>
      </div>
    </div>
  )
}

function ThreadSkeleton() {
  return (
    <div role="status" aria-label="Loading messages" className="flex flex-col gap-3">
      <Skeleton className="h-12 w-2/3 self-start rounded-xl" />
      <Skeleton className="h-16 w-1/2 self-end rounded-xl" />
      <Skeleton className="h-10 w-3/5 self-start rounded-xl" />
      <Skeleton className="h-12 w-1/2 self-end rounded-xl" />
    </div>
  )
}
