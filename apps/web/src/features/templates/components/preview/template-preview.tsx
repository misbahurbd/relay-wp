import {
  BadgeCheckIcon,
  ChevronLeftIcon,
  CopyIcon,
  FileTextIcon,
  ImageIcon,
  SendHorizontalIcon,
  VideoIcon,
} from 'lucide-react'
import { WhatsAppText } from '#/components/whatsapp-text'
import type { LocalSample } from '../../hooks/use-template-editor'
import { authMessage, isMediaHeader, SAMPLE_CODE, sampleKind } from '../../schemas/templates.schema'
import type { HeaderSample, TemplateFormValues } from '../../types'
import { BUTTON_ICON } from '../kit/button-icon'
import { formatClock } from '../kit/format'
import { fillVariables } from './preview-text'

const MEDIA_ICON = { image: ImageIcon, video: VideoIcon, document: FileTextIcon }

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

/** WhatsApp-styled text: variables replaced by their samples, *bold* / _italic_ / ~strike~. */
function Rich({ text, samples }: { text: string; samples: string[] }) {
  return <WhatsAppText text={fillVariables(text, samples)} />
}

/** The phone on the right of the form: exactly what the customer will see. */
export function TemplatePreview({
  values,
  sample,
  localSample,
  businessName,
}: {
  values: TemplateFormValues
  sample?: HeaderSample
  localSample?: LocalSample
  businessName: string
}) {
  // Authentication templates show what Meta will send for the chosen options.
  const auth = values.category === 'authentication' ? authMessage(values.auth) : null
  const { header, body, footer, buttons } = auth
    ? {
        header: { type: 'none' as const, text: '', example: '' },
        body: { text: auth.body, examples: [SAMPLE_CODE] },
        footer: { text: auth.footer },
        buttons: [],
      }
    : values
  const media = isMediaHeader(header.type) ? header.type : null
  const MediaIcon = media ? MEDIA_ICON[media] : null
  const hasSample = media && sampleKind(sample?.content_type) === media

  return (
    <div
      role="img"
      aria-label="Phone preview of the template message"
      className="mx-auto flex w-full max-w-[340px] flex-col overflow-hidden rounded-[44px] border-[10px] border-[#1A1F1C] bg-chat shadow-[0_20px_50px_rgba(0,0,0,0.18)]"
    >
      <div className="flex items-center gap-2.5 bg-panel px-3.5 pt-4 pb-3 text-panel-foreground">
        <ChevronLeftIcon className="size-[18px]" aria-hidden="true" />
        <span className="flex size-[34px] flex-none items-center justify-center rounded-full bg-brand-soft text-[13px] font-extrabold text-brand-text">
          {initials(businessName)}
        </span>
        <span className="flex min-w-0 flex-col leading-tight">
          <span className="flex items-center gap-1 truncate text-sm font-bold">
            {businessName}
            <BadgeCheckIcon className="size-3.5 flex-none" aria-hidden="true" />
          </span>
          <span className="text-[11px] text-panel-muted">Business account</span>
        </span>
      </div>

      <div className="flex min-h-[520px] flex-col gap-1 px-3 py-4">
        <span className="mb-2 self-center rounded-lg bg-card px-2.5 py-0.5 text-[11px] font-semibold text-muted-foreground">
          Today
        </span>
        <div className="w-[86%] overflow-hidden rounded-xl rounded-tl-[4px] bg-bubble-in p-1 shadow-xs">
          {media && MediaIcon && (
            <div>
              {media === 'image' && localSample?.url ? (
                <img
                  src={localSample.url}
                  alt=""
                  className="aspect-[1.91/1] w-full rounded-lg object-cover"
                />
              ) : (
                <div className="flex h-[150px] w-full flex-col items-center justify-center gap-1.5 rounded-lg bg-brand-soft text-brand-text">
                  <MediaIcon className="size-7" aria-hidden="true" />
                  <span className="px-2 text-center text-xs font-semibold">
                    {hasSample ? (localSample?.name ?? `Sample ${media}`) : `Your ${media} here`}
                  </span>
                </div>
              )}
            </div>
          )}
          <div className="flex flex-col gap-1 px-2 pt-1.5 pb-0.5 text-sm leading-[1.4] break-words whitespace-pre-wrap text-foreground">
            {header.type === 'text' && header.text.trim() && (
              <span className="text-[15px] font-bold">
                <Rich text={header.text} samples={[header.example]} />
              </span>
            )}
            {body.text.trim() ? (
              <span>
                <Rich text={body.text} samples={body.examples} />
              </span>
            ) : (
              <span className="text-muted-foreground">Your message will appear here.</span>
            )}
            {footer.text.trim() && (
              <span className="text-xs text-muted-foreground">{footer.text}</span>
            )}
            <span className="self-end text-[11px] text-tick-sent">{formatClock()}</span>
          </div>
        </div>
        {auth && (
          <div className="flex min-h-10 w-[86%] items-center justify-center gap-1.5 rounded-[10px] bg-bubble-in p-2 text-sm font-semibold text-info shadow-xs">
            <CopyIcon className="size-[15px]" aria-hidden="true" />
            <span className="truncate">{auth.button}</span>
          </div>
        )}
        {buttons.map((b, i) => {
          const Icon = BUTTON_ICON[b.type]
          return (
            <div
              // biome-ignore lint/suspicious/noArrayIndexKey: mirrors the form rows by position
              key={i}
              className="flex min-h-10 w-[86%] items-center justify-center gap-1.5 rounded-[10px] bg-bubble-in p-2 text-sm font-semibold text-info shadow-xs"
            >
              <Icon className="size-[15px]" aria-hidden="true" />
              <span className="truncate">{b.text.trim() || 'Button text'}</span>
            </div>
          )
        })}
      </div>

      <div className="flex items-center gap-2 bg-chat px-2.5 pt-2 pb-3.5">
        <span className="flex h-[38px] flex-1 items-center rounded-full bg-card px-3.5 text-[13px] text-muted-foreground">
          Message
        </span>
        <span className="flex size-[38px] flex-none items-center justify-center rounded-full bg-panel text-panel-foreground">
          <SendHorizontalIcon className="size-4" aria-hidden="true" />
        </span>
      </div>
    </div>
  )
}
