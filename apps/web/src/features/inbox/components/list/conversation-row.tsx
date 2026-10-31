import { CheckCheckIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'
import { CHANNEL_COPY, channelHandle } from '../../schemas/inbox.schema'
import type { Conversation } from '../../types'
import { Avatar } from '../kit/avatar'
import { ChannelIcon } from '../kit/channel-badge'
import {
  assigneeShort,
  formatDuration,
  formatListTime,
  waitingMinutes,
  waitTone,
} from '../kit/format'

/** One chat in the list (F2): name, channel, time, last message, unread count, waiting badge, assignee. */
export function ConversationRow({
  conversation: c,
  meId,
  now,
  selected,
  onSelect,
}: {
  conversation: Conversation
  meId: string
  now: number
  selected: boolean
  onSelect: () => void
}) {
  const name = c.contact.name || c.contact.phone
  const unread = c.unread_count > 0
  const lastOut = c.last_message_direction === 'out'
  // An agent's reply clears waiting_since; an automated one leaves the customer waiting.
  const wait = c.status !== 'closed' ? waitingMinutes(c.waiting_since, now) : null

  return (
    <Button
      variant="unstyled"
      type="button"
      onClick={onSelect}
      aria-current={selected ? 'true' : undefined}
      className={cn(
        'flex w-full gap-3 rounded-lg px-2.5 py-3 text-left hover:bg-muted',
        selected && 'bg-brand-soft hover:bg-brand-soft',
      )}
    >
      <Avatar name={name} seed={c.contact.id} />
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">
        <span className="flex items-center gap-2">
          <span
            className={cn('min-w-0 flex-1 truncate', unread ? 'font-extrabold' : 'font-semibold')}
          >
            {name}
          </span>
          <span
            className="flex-none text-muted-foreground"
            title={`${CHANNEL_COPY[c.channel.type].label} · ${channelHandle(c.channel)}`}
          >
            <ChannelIcon type={c.channel.type} className="size-3.5" />
            <span className="sr-only">
              via {CHANNEL_COPY[c.channel.type].label}, {channelHandle(c.channel)}
            </span>
          </span>
          <span
            className={cn(
              'text-xs font-semibold whitespace-nowrap',
              unread ? 'text-brand-text' : 'text-muted-foreground',
            )}
          >
            {formatListTime(c.last_message_at, now)}
          </span>
        </span>
        <span className="flex items-center gap-1.5">
          {lastOut && (
            <CheckCheckIcon aria-label="You replied" className="size-4 flex-none text-tick-read" />
          )}
          <span className="min-w-0 flex-1 truncate text-[13px] text-muted-foreground">
            {c.last_message_preview || ' '}
          </span>
          {unread && (
            <span className="inline-flex h-5 min-w-5 items-center justify-center rounded-full bg-primary px-1.5 text-[11px] font-extrabold text-primary-foreground">
              <span className="sr-only">Unread messages: </span>
              {c.unread_count}
            </span>
          )}
        </span>
        <span className="mt-0.5 flex flex-wrap items-center gap-1">
          {wait !== null && (
            <span className={cn('text-[11px] font-bold', waitTone(wait))}>
              Waiting {formatDuration(wait * 60_000)}
            </span>
          )}
          <span className="ml-auto text-[11px] text-muted-foreground">
            {assigneeShort(c.assignee, meId)}
          </span>
        </span>
      </span>
    </Button>
  )
}
