import { CornerDownRightIcon, MapPinIcon, MegaphoneIcon, UserIcon } from 'lucide-react'
import { WhatsAppText } from '#/components/whatsapp-text'
import type { Message } from '../../types'
import { MediaDocument, MediaImage, MediaPlayer } from './media'

type SharedContact = {
  name?: { formatted_name?: string; first_name?: string }
  phones?: { phone?: string; wa_id?: string }[]
  emails?: { email?: string }[]
}

function sharedContacts(raw: unknown): SharedContact[] {
  return Array.isArray(raw) ? (raw as SharedContact[]) : []
}

function Text({ text }: { text: string }) {
  if (!text) return null
  return (
    <span className="text-sm break-words whitespace-pre-wrap">
      <WhatsAppText text={text} />
    </span>
  )
}

/** The inside of a customer or agent bubble, by WhatsApp message type (F3). */
export function MessageContent({
  workspaceId,
  conversationId,
  message: m,
}: {
  workspaceId: string
  conversationId: string
  message: Message
}) {
  const media = { workspaceId, conversationId, message: m }
  const { content } = m
  const referral = content.referral && (
    <div className="flex items-start gap-2 rounded-md bg-card/70 p-2 text-xs">
      <MegaphoneIcon className="mt-0.5 size-3.5 flex-none" aria-hidden="true" />
      <span className="flex min-w-0 flex-col">
        <span className="font-bold">
          From an ad{content.referral.headline ? `: ${content.referral.headline}` : ''}
        </span>
        {content.referral.body && (
          <span className="text-muted-foreground">{content.referral.body}</span>
        )}
      </span>
    </div>
  )

  switch (m.type) {
    case 'image':
      return (
        <>
          {referral}
          <MediaImage {...media} />
          <Text text={m.body} />
        </>
      )
    case 'sticker':
      return <MediaImage {...media} sticker />
    case 'video':
    case 'audio':
      return (
        <>
          {referral}
          <MediaPlayer {...media} kind={m.type} />
          <Text text={m.body} />
        </>
      )
    case 'document':
      return (
        <>
          <MediaDocument {...media} />
          <Text text={m.body} />
        </>
      )
    case 'location': {
      const loc = content.location
      if (!loc) return <Text text={m.body} />
      return (
        <a
          href={`https://www.google.com/maps/search/?api=1&query=${loc.latitude},${loc.longitude}`}
          target="_blank"
          rel="noreferrer"
          className="flex w-64 max-w-full items-start gap-2 rounded-md bg-card/70 p-2.5 text-foreground no-underline hover:bg-card"
        >
          <MapPinIcon className="mt-0.5 size-4 flex-none text-destructive" aria-hidden="true" />
          <span className="flex min-w-0 flex-col text-[13px] leading-snug">
            <span className="font-bold">{loc.name || 'Shared location'}</span>
            {loc.address && <span className="text-muted-foreground">{loc.address}</span>}
            <span className="text-xs font-bold text-brand-text">Open in Maps</span>
          </span>
        </a>
      )
    }
    case 'contacts': {
      const cards = sharedContacts(content.contacts)
      if (cards.length === 0) return <Text text={m.body || 'Contact card'} />
      return (
        <div className="flex flex-col gap-1.5">
          {cards.map((card, i) => (
            <div
              // biome-ignore lint/suspicious/noArrayIndexKey: shared cards have no ID
              key={i}
              className="flex w-60 max-w-full items-center gap-2.5 rounded-md bg-card/70 p-2.5"
            >
              <span className="flex size-9 flex-none items-center justify-center rounded-full bg-info-soft text-info">
                <UserIcon className="size-4" aria-hidden="true" />
              </span>
              <span className="flex min-w-0 flex-col text-[13px] leading-snug">
                <span className="truncate font-bold">
                  {card.name?.formatted_name || card.name?.first_name || 'Contact'}
                </span>
                {card.phones?.map((p) => (
                  <span key={p.phone ?? p.wa_id} className="text-muted-foreground tabular-nums">
                    {p.phone ?? p.wa_id}
                  </span>
                ))}
                {card.emails?.map((e) => (
                  <span key={e.email} className="truncate text-muted-foreground">
                    {e.email}
                  </span>
                ))}
              </span>
            </div>
          ))}
        </div>
      )
    }
    case 'interactive':
    case 'button': {
      const reply = content.interactive
      return (
        <span className="flex items-start gap-1.5 text-sm">
          <CornerDownRightIcon
            className="mt-0.5 size-4 flex-none text-muted-foreground"
            aria-label="Tapped"
          />
          <span className="flex flex-col">
            <span className="font-semibold">{reply?.title || m.body}</span>
            {reply?.description && (
              <span className="text-xs text-muted-foreground">{reply.description}</span>
            )}
          </span>
        </span>
      )
    }
    case 'template': {
      const t = content.template
      return (
        <div className="flex flex-col gap-1">
          <span className="text-[11px] font-extrabold tracking-wide text-muted-foreground uppercase">
            Template · {t?.name ?? 'template'}
          </span>
          {t?.params?.header_text && <strong className="text-sm">{t.params.header_text}</strong>}
          <Text text={m.body} />
        </div>
      )
    }
    case 'unsupported':
      return (
        <span className="text-sm text-muted-foreground italic">
          This message type isn’t supported. Ask the customer to send it another way.
        </span>
      )
    default:
      return (
        <>
          {referral}
          <Text text={m.body} />
        </>
      )
  }
}
