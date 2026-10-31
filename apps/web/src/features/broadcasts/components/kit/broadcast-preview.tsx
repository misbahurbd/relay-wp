import {
  BadgeCheckIcon,
  ChevronLeftIcon,
  FileTextIcon,
  GlobeIcon,
  ImageIcon,
  PhoneIcon,
  ReplyIcon,
  VideoIcon,
} from 'lucide-react'
import { WhatsAppText } from '#/components/whatsapp-text'
import type { Template } from '#/features/templates'
import { fill, sampleValue } from '../../schemas/broadcasts.schema'
import type { BroadcastForm } from '../../types'

const MEDIA_ICON = { image: ImageIcon, video: VideoIcon, document: FileTextIcon }
const BUTTON_ICON = { quick_reply: ReplyIcon, url: GlobeIcon, phone: PhoneIcon }

function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase())
      .join('') || 'R'
  )
}

/**
 * The phone beside the template step: the message as a customer sees it, from the sending
 * number's verified name, with sample values for contact fields.
 */
export function BroadcastPreview({
  template,
  form,
  senderName,
}: {
  template?: Template
  form: BroadcastForm
  /** The sending number's verified name: what customers see at the top of the chat. */
  senderName?: string
}) {
  const c = template?.components
  const body = c ? fill(c.body.text, form.body.map(sampleValue)) : ''
  const header = c?.header.type === 'text' ? fill(c.header.text, [sampleValue(form.header)]) : ''
  const media = c && c.header.type in MEDIA_ICON ? (c.header.type as keyof typeof MEDIA_ICON) : null
  const MediaIcon = media ? MEDIA_ICON[media] : null
  const name = senderName || 'Your business'
  const usesContact = [...form.body, ...(form.header ? [form.header] : [])].some(
    (v) => v.source !== 'text',
  )

  return (
    <div className="flex flex-col gap-2.5 lg:sticky lg:top-5">
      <span className="text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase">
        Preview
      </span>
      <div
        role="img"
        aria-label={
          c ? `Phone preview of the ${template?.name} message` : 'Phone preview, no template chosen'
        }
        className="mx-auto flex w-full max-w-[320px] flex-col overflow-hidden rounded-[40px] border-[9px] border-[#1A1F1C] bg-chat shadow-[0_20px_50px_rgba(0,0,0,0.16)]"
      >
        <div className="flex items-center gap-2.5 bg-panel px-3.5 pt-4 pb-3 text-panel-foreground">
          <ChevronLeftIcon className="size-[18px] flex-none" aria-hidden="true" />
          <span className="flex size-[34px] flex-none items-center justify-center rounded-full bg-brand-soft text-[13px] font-extrabold text-brand-text">
            {initials(name)}
          </span>
          <span className="flex min-w-0 flex-col leading-tight">
            <span className="flex items-center gap-1 text-sm font-bold">
              <span className="truncate">{name}</span>
              <BadgeCheckIcon className="size-3.5 flex-none" aria-hidden="true" />
            </span>
            <span className="text-[11px] text-panel-muted">Business account</span>
          </span>
        </div>

        <div className="flex min-h-[400px] flex-col gap-1.5 px-3 py-4">
          <span className="mb-1 self-center rounded-md bg-card/80 px-2 py-0.5 text-[11px] font-semibold text-muted-foreground shadow-sm">
            Today
          </span>

          {c ? (
            <>
              <div className="max-w-[88%] self-start overflow-hidden rounded-[4px_12px_12px_12px] bg-bubble-in text-[13.5px] leading-snug shadow-[0_1px_1px_rgba(0,0,0,.1)]">
                {MediaIcon && (
                  <div className="m-1 mb-0 flex h-[130px] flex-col items-center justify-center gap-1 rounded-[8px] bg-muted text-xs font-semibold text-muted-foreground">
                    <MediaIcon className="size-7" aria-hidden="true" />
                    {media === 'document' ? 'Document' : media === 'video' ? 'Video' : 'Image'}
                  </div>
                )}
                <div className="flex flex-col gap-1 px-2.5 pt-2 pb-1.5">
                  {header && <span className="font-bold">{header}</span>}
                  <span className="break-words whitespace-pre-wrap">
                    <WhatsAppText text={body} />
                  </span>
                  {c.footer.text && (
                    <span className="text-xs text-muted-foreground">{c.footer.text}</span>
                  )}
                  <span className="self-end text-[10.5px] text-muted-foreground">10:00 AM</span>
                </div>
              </div>
              {c.buttons.map((b) => {
                const Icon = BUTTON_ICON[b.type as keyof typeof BUTTON_ICON] ?? ReplyIcon
                return (
                  <span
                    key={`${b.type}-${b.text}`}
                    className="flex w-[88%] items-center justify-center gap-1.5 self-start rounded-[10px] bg-bubble-in px-3 py-2 text-[13px] font-semibold text-info shadow-[0_1px_1px_rgba(0,0,0,.1)]"
                  >
                    <Icon className="size-3.5" aria-hidden="true" />
                    {b.text}
                  </span>
                )
              })}
            </>
          ) : (
            <div className="flex flex-1 flex-col items-start gap-3">
              <div
                aria-hidden="true"
                className="flex w-[78%] flex-col gap-2 rounded-[4px_12px_12px_12px] border-2 border-dashed border-border bg-card/60 p-3"
              >
                <span className="h-2.5 w-3/4 rounded-full bg-muted" />
                <span className="h-2.5 w-full rounded-full bg-muted" />
                <span className="h-2.5 w-2/3 rounded-full bg-muted" />
              </div>
              <p className="mx-auto mt-auto max-w-[220px] text-center text-[13px] text-muted-foreground">
                Pick a template to see exactly what customers will get.
              </p>
            </div>
          )}
        </div>
      </div>
      {c && usesContact && (
        <p className="mx-auto max-w-[320px] text-center text-xs text-muted-foreground">
          Contact fields show sample values, like “Ayesha” for a name.
        </p>
      )}
    </div>
  )
}
