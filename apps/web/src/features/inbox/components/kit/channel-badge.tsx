import { cn } from '#/lib/utils'
import { CHANNEL_COPY, channelHandle } from '../../schemas/inbox.schema'
import type { Channel, ChannelType } from '../../types'

/** Simple monochrome brand marks (lucide has no brand icons); decorative, the label carries the name. */
const MARKS: Record<ChannelType, React.ReactNode> = {
  whatsapp: (
    <>
      <path
        d="M12 3.5a8.5 8.5 0 0 0-7.3 12.8L3.5 20.5l4.3-1.1A8.5 8.5 0 1 0 12 3.5Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinejoin="round"
      />
      <path
        d="M9.2 7.8c-.3 0-.7.1-1 .5-.3.4-.8 1-.8 2.2 0 1.3.9 2.5 1 2.7.2.2 1.8 2.8 4.3 3.8 2.1.8 2.5.7 3 .6.5-.1 1.5-.6 1.7-1.2.2-.6.2-1.1.1-1.2l-.5-.3-1.7-.8c-.2-.1-.4-.1-.6.1l-.8 1c-.1.2-.3.2-.5.1-.3-.1-1-.4-2-1.2-.7-.6-1.2-1.4-1.3-1.7-.1-.2 0-.4.1-.5l.4-.5.3-.4v-.5l-.8-1.8c-.2-.5-.4-.4-.6-.4h-.4Z"
        fill="currentColor"
      />
    </>
  ),
  messenger: (
    <>
      <path
        d="M12 3.5c-4.8 0-8.5 3.5-8.5 8.1 0 2.4 1 4.6 2.7 6v3l2.8-1.6c.9.3 1.9.4 3 .4 4.8 0 8.5-3.5 8.5-8S16.8 3.5 12 3.5Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
        strokeLinejoin="round"
      />
      <path d="m7.2 14 3.3-3.6 2.2 1.9 4.1-2.3-3.3 3.6-2.2-1.9Z" fill="currentColor" />
    </>
  ),
  instagram: (
    <>
      <rect
        x="3.5"
        y="3.5"
        width="17"
        height="17"
        rx="5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.8"
      />
      <circle cx="12" cy="12" r="3.9" fill="none" stroke="currentColor" strokeWidth="1.8" />
      <circle cx="17.1" cy="6.9" r="1.1" fill="currentColor" />
    </>
  ),
}

/** The channel's brand mark. Pair it with CHANNEL_COPY's label, visible or sr-only. */
export function ChannelIcon({ type, className }: { type: ChannelType; className?: string }) {
  return (
    <svg
      viewBox="0 0 24 24"
      aria-hidden="true"
      focusable="false"
      className={cn('size-4 flex-none', className)}
    >
      {MARKS[type]}
    </svg>
  )
}

/** "WhatsApp · +1 415 555 0142": the channel's mark, label and handle (or "Disconnected channel"). */
export function ChannelBadge({ channel, className }: { channel: Channel; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex min-w-0 items-center gap-1 text-xs font-semibold text-muted-foreground',
        className,
      )}
    >
      <ChannelIcon type={channel.type} className="size-3.5" />
      <span className="truncate">
        {CHANNEL_COPY[channel.type].label} · {channelHandle(channel)}
      </span>
    </span>
  )
}
