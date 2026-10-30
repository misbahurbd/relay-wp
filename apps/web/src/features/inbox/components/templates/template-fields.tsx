import { Loader2Icon, PaperclipIcon } from 'lucide-react'
import { useRef } from 'react'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import type { InboxTemplate, Upload } from '../../types'
import { formatSize } from '../kit/format'

export type TemplateValues = {
  /** {{1}}..{{N}} of the body, in order */
  body: string[]
  /** The text header's {{1}} */
  header: string
  /** The header image, video or document, once uploaded */
  media: Upload | null
  /** Link values by button index */
  urls: Record<number, string>
}

const HEADER_ACCEPT: Record<string, string> = {
  image: 'image/jpeg,image/png',
  video: 'video/mp4,video/3gpp',
  document: '.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt',
}

/** "Fill variables": the body's {{n}}, a header variable or file, and URL button values. */
export function TemplateFields({
  template: t,
  values,
  onChange,
  uploading,
  onHeaderFile,
}: {
  template: InboxTemplate
  values: TemplateValues
  onChange: (next: TemplateValues) => void
  uploading: boolean
  onHeaderFile: (file: File) => void
}) {
  const file = useRef<HTMLInputElement>(null)
  const nothing =
    t.body_variables === 0 && !t.header_variable && !t.header_media && t.url_buttons.length === 0
  return (
    <section aria-labelledby="tpl-vars" className="flex flex-col gap-2.5 rounded-lg border p-3">
      <h3 id="tpl-vars" className="text-sm font-extrabold">
        Fill variables
      </h3>
      {nothing && (
        <p className="text-[13px] text-muted-foreground">This template has no variables.</p>
      )}

      {t.header_variable && (
        <div className="flex flex-col gap-1">
          <Label htmlFor="tpl-header">Header {'{{1}}'}</Label>
          <Input
            id="tpl-header"
            value={values.header}
            onChange={(e) => onChange({ ...values, header: e.target.value })}
          />
        </div>
      )}

      {t.header_media && (
        <div className="flex flex-col gap-1">
          <span className="text-sm font-bold">Header {t.header_type}</span>
          <div className="flex items-center gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              disabled={uploading}
              onClick={() => file.current?.click()}
            >
              {uploading ? (
                <Loader2Icon className="animate-spin" aria-hidden="true" />
              ) : (
                <PaperclipIcon aria-hidden="true" />
              )}
              {values.media ? 'Replace file' : 'Choose file'}
            </Button>
            {values.media && (
              <span className="min-w-0 truncate text-[13px] text-muted-foreground">
                {values.media.filename} · {formatSize(values.media.size)}
              </span>
            )}
          </div>
          <input
            ref={file}
            type="file"
            hidden
            accept={HEADER_ACCEPT[t.header_type]}
            onChange={(e) => {
              const f = e.target.files?.[0]
              e.target.value = ''
              if (f) onHeaderFile(f)
            }}
          />
        </div>
      )}

      {Array.from({ length: t.body_variables }, (_, i) => {
        const id = `tpl-var-${i + 1}`
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: variables are positional
          <div key={i} className="flex flex-col gap-1">
            <Label htmlFor={id}>{`{{${i + 1}}}`}</Label>
            <Input
              id={id}
              aria-label={`Variable ${i + 1}`}
              value={values.body[i] ?? ''}
              onChange={(e) => {
                const body = [...values.body]
                body[i] = e.target.value
                onChange({ ...values, body })
              }}
            />
          </div>
        )
      })}

      {t.url_buttons.map((index) => {
        const id = `tpl-url-${index}`
        return (
          <div key={index} className="flex flex-col gap-1">
            <Label htmlFor={id}>Link for “{t.buttons[index]?.text ?? `button ${index + 1}`}”</Label>
            <Input
              id={id}
              value={values.urls[index] ?? ''}
              placeholder={t.buttons[index]?.value}
              onChange={(e) =>
                onChange({ ...values, urls: { ...values.urls, [index]: e.target.value } })
              }
            />
          </div>
        )
      })}
    </section>
  )
}
