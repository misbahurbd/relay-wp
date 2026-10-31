import { CircleAlertIcon } from 'lucide-react'
import { toast } from 'sonner'
import { Popover, PopoverContent, PopoverTrigger } from '#/components/ui/popover'
import { cn } from '#/lib/utils'
import { useReact } from '../../api/inbox.mutations'
import { categoryLabel, REACTIONS, templatePrice } from '../../schemas/inbox.schema'
import type { Message } from '../../types'
import { CHANNEL_UNAVAILABLE, inboxErrorMessage } from '../kit/errors'
import { firstName, formatClock } from '../kit/format'
import { messageSummaryText } from '../kit/summary'
import { Ticks } from '../kit/ticks'
import { MessageContent } from './message-content'

const actionBtn =
  'h-7 rounded-sm border bg-card px-2 text-xs font-semibold hover:bg-muted focus-visible:opacity-100'

/** A customer (left) or agent (right) message with its quote, ticks, reactions and actions. */
export function MessageBubble({
  workspaceId,
  conversationId,
  message: m,
  customerName,
  canReply,
  canReact,
  metaPricing,
  onReply,
}: {
  workspaceId: string
  conversationId: string
  message: Message
  customerName: string
  /** Reply and React need inbox.reply and an open window. */
  canReply: boolean
  /** The channel takes reactions; React also needs canReply. */
  canReact: boolean
  /** Show Meta's estimated charge on template messages. */
  metaPricing: boolean
  onReply: (m: Message) => void
}) {
  const out = m.direction === 'out'
  const react = useReact(workspaceId)
  const quoteWho = m.reply_to
    ? m.reply_to.direction === 'out'
      ? 'You'
      : firstName(customerName)
    : ''

  const copy = async () => {
    try {
      await navigator.clipboard.writeText(messageSummaryText(m))
      toast.success('Message copied')
    } catch {
      toast.error('Copying isn’t allowed in this browser.')
    }
  }
  const toggleReaction = (emoji: string) =>
    react.mutate(
      { conversationId, messageId: m.id, emoji: m.reactions.out === emoji ? '' : emoji },
      { onError: (e) => toast.error(inboxErrorMessage(e)) },
    )

  return (
    <div
      className={cn(
        'group flex max-w-[85%] flex-col gap-1 sm:max-w-[72%]',
        out ? 'self-end items-end' : 'self-start items-start',
      )}
    >
      <div
        className={cn(
          'flex flex-col gap-1.5 px-3 py-2 shadow-card',
          out
            ? 'rounded-[14px_4px_14px_14px] bg-bubble-out'
            : 'rounded-[4px_14px_14px_14px] bg-bubble-in',
        )}
      >
        {m.reply_to && (
          <div className="flex flex-col rounded-sm border-l-[3px] border-brand bg-card/60 px-2 py-1 text-xs">
            <span className="font-bold text-brand-text">{quoteWho}</span>
            <span className="line-clamp-2 text-muted-foreground">
              {messageSummaryText(m.reply_to)}
            </span>
          </div>
        )}
        <MessageContent workspaceId={workspaceId} conversationId={conversationId} message={m} />
        <span className="flex items-center justify-end gap-1 self-end text-[11px] text-muted-foreground">
          {out && m.author && <span>{firstName(m.author.name)} ·</span>}
          {out && m.automated && <span>Automation ·</span>}
          <time dateTime={m.created_at}>{formatClock(m.created_at)}</time>
          {out && <Ticks status={m.status} />}
        </span>
        {out && m.status === 'failed' && (
          <span
            role="alert"
            className="flex items-start gap-1 text-xs font-semibold text-destructive"
          >
            <CircleAlertIcon className="mt-px size-3.5 flex-none" aria-hidden="true" />
            {m.error_code === 'channel_unavailable' ? (
              `Not delivered: ${CHANNEL_UNAVAILABLE}`
            ) : (
              <>
                Not delivered{m.error_title ? `: ${m.error_title}` : ''}
                {m.error_code ? ` (${m.error_code})` : ''}
              </>
            )}
          </span>
        )}
      </div>

      {(m.reactions.in || m.reactions.out) && (
        <span className="-mt-2 flex gap-1 px-2">
          {m.reactions.in && (
            <span
              className="rounded-full border bg-card px-1.5 text-sm"
              role="img"
              aria-label={`${firstName(customerName)} reacted ${m.reactions.in}`}
            >
              {m.reactions.in}
            </span>
          )}
          {m.reactions.out && (
            <span
              className="rounded-full border bg-card px-1.5 text-sm"
              role="img"
              aria-label={`You reacted ${m.reactions.out}`}
            >
              {m.reactions.out}
            </span>
          )}
        </span>
      )}

      {m.type === 'template' && m.content.template && (
        <span className="text-[11px] text-muted-foreground">
          {categoryLabel(m.content.template.category)}
          {metaPricing && ` · ${templatePrice(m.content.template.category)} est. Meta charge`}
        </span>
      )}

      <div className="flex gap-1 opacity-0 transition-opacity group-focus-within:opacity-100 group-hover:opacity-100 [@media(hover:none)]:opacity-100">
        {canReply && (
          <button
            type="button"
            className={actionBtn}
            aria-label="Reply to this message"
            onClick={() => onReply(m)}
          >
            Reply
          </button>
        )}
        <button type="button" className={actionBtn} aria-label="Copy this message" onClick={copy}>
          Copy
        </button>
        {canReply && canReact && (
          <Popover>
            <PopoverTrigger asChild>
              <button type="button" className={actionBtn} aria-label="React to this message">
                React
              </button>
            </PopoverTrigger>
            <PopoverContent className="flex w-auto gap-1 p-1.5" align={out ? 'end' : 'start'}>
              {REACTIONS.map((r) => (
                <button
                  key={r.emoji}
                  type="button"
                  aria-label={`React with ${r.label}`}
                  aria-pressed={m.reactions.out === r.emoji}
                  onClick={() => toggleReaction(r.emoji)}
                  className={cn(
                    'flex size-10 items-center justify-center rounded-md text-xl hover:bg-muted',
                    m.reactions.out === r.emoji && 'bg-brand-soft',
                  )}
                >
                  {r.emoji}
                </button>
              ))}
            </PopoverContent>
          </Popover>
        )}
      </div>
    </div>
  )
}
