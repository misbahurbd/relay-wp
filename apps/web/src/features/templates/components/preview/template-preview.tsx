import {
  BadgeCheckIcon,
  ChevronLeftIcon,
  FileTextIcon,
  ImageIcon,
  MicIcon,
  VideoIcon,
} from 'lucide-react'
import { cn } from '#/lib/utils'
import type { LocalSample } from '../../hooks/use-template-editor'
import { isMediaHeader, sampleKind } from '../../schemas/templates.schema'
import type { HeaderSample, TemplateFormValues } from '../../types'
import { BUTTON_ICON } from '../kit/button-icon'
import { formatClock } from '../kit/format'
import { fillVariables, formatSegments } from './preview-text'

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
  return (
    <>
      {formatSegments(fillVariables(text, samples)).map((s, i) => (
        <span
          // biome-ignore lint/suspicious/noArrayIndexKey: segments have no identity of their own
          key={i}
          className={cn(s.bold && 'font-bold', s.italic && 'italic', s.strike && 'line-through')}
        >
          {s.text}
        </span>
      ))}
    </>
  )
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
  const { header, body, footer, buttons } = values
  const media = isMediaHeader(header.type) ? header.type : null
  const MediaIcon = media ? MEDIA_ICON[media] : null
  const hasSample = media && sampleKind(sample?.content_type) === media

  return (
    <div
      role="img"
      aria-label="Phone preview of the template message"
      className="mx-auto flex w-full max-w-[340px] flex-col overflow-hidden rounded-[28px] border-[6px] border-foreground/85 bg-chat shadow-card"
    >
      <div className="flex items-center gap-2.5 bg-panel px-3 py-2.5 text-panel-foreground">
        <ChevronLeftIcon className="size-5" aria-hidden="true" />
        <span className="flex size-8 items-center justify-center rounded-full bg-brand-soft text-xs font-extrabold text-brand-text">
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

      <div className="flex min-h-[360px] flex-col gap-1.5 px-3 py-4">
        <span className="mb-1 self-center rounded-md bg-card px-2 py-0.5 text-[11px] font-semibold text-muted-foreground">
          Today
        </span>
        <div className="max-w-[92%] overflow-hidden rounded-lg rounded-tl-none bg-bubble-in shadow-xs">
          {media && MediaIcon && (
            <div className="p-1">
              {media === 'image' && localSample?.url ? (
                <img
                  src={localSample.url}
                  alt=""
                  className="aspect-[1.91/1] w-full rounded-md object-cover"
                />
              ) : (
                <div className="flex aspect-[1.91/1] w-full flex-col items-center justify-center gap-1.5 rounded-md bg-muted text-muted-foreground">
                  <MediaIcon className="size-7" aria-hidden="true" />
                  <span className="px-2 text-center text-[11px]">
                    {hasSample ? (localSample?.name ?? `Sample ${media}`) : `Your ${media} here`}
                  </span>
                </div>
              )}
            </div>
          )}
          <div className="flex flex-col gap-1 px-2.5 pt-1.5 pb-1 text-[13.5px] leading-snug break-words whitespace-pre-wrap text-foreground">
            {header.type === 'text' && header.text.trim() && (
              <span className="font-bold">
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
            <span className="self-end text-[10.5px] text-tick-sent">{formatClock()}</span>
          </div>
        </div>
        {buttons.map((b, i) => {
          const Icon = BUTTON_ICON[b.type]
          return (
            <div
              // biome-ignore lint/suspicious/noArrayIndexKey: mirrors the form rows by position
              key={i}
              className="flex max-w-[92%] items-center justify-center gap-1.5 rounded-lg bg-bubble-in px-3 py-2 text-[13px] font-semibold text-info shadow-xs"
            >
              <Icon className="size-3.5" aria-hidden="true" />
              <span className="truncate">{b.text.trim() || 'Button text'}</span>
            </div>
          )
        })}
      </div>

      <div className="flex items-center gap-2 bg-chat px-3 pb-3">
        <span className="flex h-9 flex-1 items-center rounded-full bg-card px-3.5 text-[13px] text-muted-foreground">
          Message
        </span>
        <span className="flex size-9 items-center justify-center rounded-full bg-brand text-primary-foreground">
          <MicIcon className="size-4" aria-hidden="true" />
        </span>
      </div>
    </div>
  )
}
