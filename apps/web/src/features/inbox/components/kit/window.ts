import { CHANNEL_COPY } from '../../schemas/inbox.schema'
import type { ConversationDetail } from '../../types'
import { firstName, formatDuration, windowLeft } from './format'

const URGENT_PCT = 25

/**
 * The channel's customer-service window (F5): time left, urgent, closed, or waiting after a
 * template. `limited` is false on a channel without a window (window_hours 0): always open.
 */
export function windowState(c: ConversationDetail, now: number) {
  const first = firstName(c.contact.name || c.contact.phone)
  const hours = c.capabilities.window_hours
  const templates = c.capabilities.templates
  const label = `${hours}-hour window`
  if (hours <= 0) {
    return {
      limited: false,
      open: true,
      first,
      hours,
      label,
      templates,
      pct: 100,
      tone: 'open',
      title: '',
      hint: '',
    } as const
  }
  const { ms, pct } = windowLeft(c.window_expires_at, now, hours)
  // The server decides (window_open); the clock only drives the countdown between refetches.
  const open = c.window_open && ms > 0
  const base = { limited: true, open, first, hours, label, templates } as const
  if (open) {
    const urgent = pct < URGENT_PCT
    return {
      ...base,
      pct,
      tone: urgent ? 'urgent' : 'open',
      title: `${formatDuration(ms)} left to reply freely`,
      hint: urgent
        ? templates
          ? 'Reply soon. After this you can only send approved templates.'
          : `Reply soon. After this you can reply again once ${first} writes.`
        : CHANNEL_COPY[c.channel.type].metaPricing
          ? 'Replies inside the window are free service messages.'
          : `You can reply freely until the ${label} ends.`,
    } as const
  }
  if (c.template_since_inbound) {
    return {
      ...base,
      pct: 0,
      tone: 'awaiting',
      title: `Template sent · waiting for ${first} to reply`,
      hint: `The window reopens as soon as ${first} replies.`,
    } as const
  }
  return {
    ...base,
    pct: 0,
    tone: 'closed',
    title: `${label} closed`,
    hint: templates
      ? 'Send an approved template to restart the conversation.'
      : `The window reopens as soon as ${first} writes again.`,
  } as const
}

export type WindowState = ReturnType<typeof windowState>
