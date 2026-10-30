import { FileTextIcon, Loader2Icon, LockIcon, ReplyIcon, XIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'
import { TEMPLATE_PRICE } from '../../schemas/inbox.schema'
import type { Message, Upload } from '../../types'
import { formatSize } from '../kit/format'
import { messageSummaryText } from '../kit/summary'
import type { WindowState } from '../kit/window'

/** Above the composer when free-form replies are blocked (AD-6): only a template can go out. */
export function ClosedBanner({
  window: win,
  canTemplate,
  metaPricing,
  onOpenTemplate,
}: {
  window: WindowState
  /** The member may send templates and the channel has them. */
  canTemplate: boolean
  /** Add Meta's template rates (WhatsApp only). */
  metaPricing: boolean
  onOpenTemplate: () => void
}) {
  const awaiting = win.tone === 'awaiting'
  const closedHint = win.templates
    ? `You can only send an approved template until ${win.first} replies.${
        metaPricing
          ? ` Meta charges per template (Utility from ${TEMPLATE_PRICE.utility}, Marketing from ${TEMPLATE_PRICE.marketing}).`
          : ''
      }`
    : `You can reply again once ${win.first} writes.`
  return (
    <div
      className={cn(
        'flex flex-wrap items-center gap-3 rounded-lg px-3 py-2.5',
        awaiting ? 'bg-warning-soft text-warning' : 'bg-neutral-soft text-neutral',
      )}
    >
      <LockIcon className="size-4 flex-none" aria-hidden="true" />
      <span className="flex min-w-0 flex-[1_1_220px] flex-col text-[13px]">
        <span className="font-bold">
          {awaiting ? `Waiting for ${win.first} to reply` : `The ${win.label} is closed`}
        </span>
        <span className="text-muted-foreground">
          {awaiting
            ? `Your template was delivered. Free-form replies unlock when ${win.first} answers.`
            : closedHint}
        </span>
      </span>
      {canTemplate && (
        <Button size="sm" onClick={onOpenTemplate}>
          <FileTextIcon aria-hidden="true" />
          Send template
        </Button>
      )}
    </div>
  )
}

export function ReplyBar({
  message,
  first,
  onClear,
}: {
  message: Message
  first: string
  onClear: () => void
}) {
  return (
    <div className="flex items-center gap-2 rounded-md border-l-[3px] border-brand bg-muted px-3 py-1.5">
      <ReplyIcon className="size-4 flex-none text-muted-foreground" aria-hidden="true" />
      <span className="flex min-w-0 flex-1 flex-col text-xs">
        <span className="font-bold text-brand-text">
          Replying to {message.direction === 'out' ? 'You' : first}
        </span>
        <span className="truncate text-muted-foreground">{messageSummaryText(message)}</span>
      </span>
      <Button variant="ghost" size="icon-xs" aria-label="Cancel reply" onClick={onClear}>
        <XIcon aria-hidden="true" />
      </Button>
    </div>
  )
}

export function AttachmentBar({
  attachment,
  uploading,
  onRemove,
}: {
  attachment: Upload | null
  uploading: boolean
  onRemove: () => void
}) {
  return (
    <div className="flex items-center gap-2 rounded-md bg-muted px-3 py-1.5 text-xs" role="status">
      {uploading ? (
        <>
          <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
          Uploading…
        </>
      ) : (
        attachment && (
          <>
            <FileTextIcon className="size-4 flex-none" aria-hidden="true" />
            <span className="min-w-0 flex-1 truncate font-semibold">
              {attachment.filename} · {formatSize(attachment.size)}
            </span>
            <Button
              variant="ghost"
              size="icon-xs"
              aria-label={`Remove ${attachment.filename}`}
              onClick={onRemove}
            >
              <XIcon aria-hidden="true" />
            </Button>
          </>
        )
      )}
    </div>
  )
}
