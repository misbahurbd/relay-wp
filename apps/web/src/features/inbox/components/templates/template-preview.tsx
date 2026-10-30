import { WhatsAppText } from '#/components/whatsapp-text'
import { CATEGORY_LABEL, TEMPLATE_PRICE } from '../../schemas/inbox.schema'
import type { InboxTemplate } from '../../types'
import type { WindowState } from '../kit/window'
import type { TemplateValues } from './template-fields'

/** Replaces {{n}} with the n-th value, or leaves {{n}} showing while it's empty. */
function fill(text: string, values: string[]) {
  return text.replace(/\{\{(\d+)\}\}/g, (all, n: string) => values[Number(n) - 1]?.trim() || all)
}

/** What the customer will see, with the category and (on WhatsApp) Meta's estimated charge. */
export function TemplatePreview({
  template: t,
  values,
  window: win,
  metaPricing,
}: {
  template: InboxTemplate | null
  values: TemplateValues
  window: WindowState
  /** Show Meta's estimated charge (WhatsApp only). */
  metaPricing: boolean
}) {
  return (
    <aside aria-label="Preview" className="flex flex-col gap-3">
      <span className="text-[11px] font-extrabold tracking-wider text-muted-foreground uppercase">
        Preview
      </span>
      <div className="rounded-lg bg-chat p-3">
        {t ? (
          <div className="flex flex-col gap-1.5 rounded-[4px_14px_14px_14px] bg-bubble-out px-3 py-2 text-sm shadow-card">
            {t.header_type === 'text' && t.header_text && (
              <strong>{fill(t.header_text, [values.header])}</strong>
            )}
            {t.header_media && (
              <span className="rounded-sm bg-card/70 px-2 py-6 text-center text-xs text-muted-foreground">
                {values.media?.filename ?? `Header ${t.header_type}`}
              </span>
            )}
            <span className="break-words whitespace-pre-wrap">
              <WhatsAppText text={fill(t.body, values.body)} />
            </span>
            {t.footer && <span className="text-xs text-muted-foreground">{t.footer}</span>}
            {t.buttons.length > 0 && (
              <div className="flex flex-col gap-1 border-t pt-1.5">
                {t.buttons.map((b) => (
                  <span
                    key={`${b.type}-${b.text}`}
                    className="text-center text-[13px] font-bold text-info"
                  >
                    {b.text}
                  </span>
                ))}
              </div>
            )}
          </div>
        ) : (
          <p className="py-8 text-center text-[13px] text-muted-foreground">
            Choose a template to preview it.
          </p>
        )}
      </div>
      {t && (
        <dl className="flex flex-col gap-1 text-[13px]">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Category</dt>
            <dd className="font-bold">{CATEGORY_LABEL[t.category]}</dd>
          </div>
          {metaPricing && (
            <div className="flex justify-between">
              <dt className="text-muted-foreground">Est. Meta charge</dt>
              <dd className="font-bold">{TEMPLATE_PRICE[t.category]}</dd>
            </div>
          )}
        </dl>
      )}
      {metaPricing && win.limited && (
        <p className="text-xs text-muted-foreground">
          {win.open
            ? `Inside the ${win.label} you could also reply for free without a template.`
            : 'This restarts the conversation. Meta bills the template when it is delivered.'}
        </p>
      )}
    </aside>
  )
}
