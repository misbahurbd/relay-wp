import { LockIcon } from 'lucide-react'
import { WhatsAppText } from '#/components/whatsapp-text'
import type { Member, Message } from '../../types'
import { formatClock, formatWhen } from '../kit/format'

/** The thread's line for an assign / close / snooze / reopen event (F7). */
export function eventText(
  m: Message,
  members: Member[],
  meId: string,
  now: number,
  windowHours: number,
) {
  const ev = m.content.event
  const time = formatClock(m.created_at)
  const nameOf = (id: string | null) => {
    if (!id) return null
    if (id === meId) return 'You'
    return members.find((x) => x.id === id)?.name ?? null
  }
  const actor = m.author?.name ?? nameOf(ev?.actor_id ?? null)
  const target = nameOf(ev?.target_id ?? null) ?? 'a teammate'
  switch (ev?.action) {
    case 'assigned':
      if (!actor) return `Assigned to ${target} automatically · ${time}`
      if (ev.actor_id && ev.actor_id === ev.target_id)
        return `${actor} assigned this chat to themselves · ${time}`
      return `${actor} assigned this chat to ${target} · ${time}`
    case 'unassigned':
      return actor ? `${actor} unassigned this chat · ${time}` : `Chat unassigned · ${time}`
    case 'closed':
      return actor ? `Chat closed by ${actor} · ${time}` : `Chat closed · ${time}`
    case 'reopened':
      return actor ? `Chat reopened by ${actor} · ${time}` : `Chat reopened · ${time}`
    case 'snoozed': {
      const until = ev.until ? formatWhen(ev.until, now) : 'later'
      return actor ? `Snoozed until ${until} by ${actor}` : `Snoozed until ${until}`
    }
    case 'unsnoozed':
      return `Snooze ended · chat moved to Open · ${time}`
    case 'template_pending':
      return actor
        ? `Template sent by ${actor} · chat moved to Pending`
        : 'Template sent · chat moved to Pending'
    case 'opted_out':
      return `Customer replied STOP · opted out of automated messages · ${time}`
    case 'window_reopened':
      return `Customer replied · ${windowHours}-hour window reopened`
    default:
      return m.body || `Chat updated · ${time}`
  }
}

export function EventLine({ text }: { text: string }) {
  return (
    <div className="self-center rounded-full bg-card/80 px-3 py-1 text-center text-xs text-muted-foreground">
      {text}
    </div>
  )
}

/** A private note (F6): amber, never sent to the customer, with who was notified. */
export function NoteBubble({ message: m, members }: { message: Message; members: Member[] }) {
  const notified = m.mentions
    .map((id) => members.find((x) => x.id === id)?.name)
    .filter((n): n is string => !!n)
  return (
    <div className="flex max-w-[85%] flex-col gap-1 self-end rounded-[14px_4px_14px_14px] border border-dashed border-warning bg-warning-soft px-3 py-2 sm:max-w-[72%]">
      <span className="flex items-center gap-1.5 text-[11px] font-extrabold text-warning">
        <LockIcon className="size-3" aria-hidden="true" />
        Private note · {m.author?.name ?? 'A teammate'}
      </span>
      <span className="text-sm break-words whitespace-pre-wrap">
        <WhatsAppText text={m.body} />
      </span>
      {notified.length > 0 && (
        <span className="flex flex-wrap gap-1">
          {notified.map((n) => (
            <span
              key={n}
              className="rounded-full bg-card px-2 py-px text-[11px] font-bold text-warning"
            >
              Notified {n}
            </span>
          ))}
        </span>
      )}
      <time dateTime={m.created_at} className="self-end text-[11px] text-muted-foreground">
        {formatClock(m.created_at)}
      </time>
    </div>
  )
}
