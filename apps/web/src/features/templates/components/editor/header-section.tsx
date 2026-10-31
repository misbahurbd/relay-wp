import { useStore } from '@tanstack/react-form'
import { FileTextIcon, ImageIcon, UploadIcon, VideoIcon } from 'lucide-react'
import { useRef } from 'react'
import { describedBy, FormField } from '#/components/form'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import type { TemplateEditorState } from '../../hooks/use-template-editor'
import {
  HEADER_TYPES,
  isMediaHeader,
  LIMITS,
  MEDIA,
  type MediaHeaderType,
  sampleFileProblem,
  sampleKind,
} from '../../schemas/templates.schema'
import type { HeaderType } from '../../types'
import { CharCounter } from '../kit/char-counter'
import { FormSection } from '../kit/form-section'
import { formatBytes } from '../kit/format'

const MEDIA_ICON = { image: ImageIcon, video: VideoIcon, document: FileTextIcon }

/** 2. Header: none, text (with one optional {{1}}) or a media sample for Meta's review. */
export function HeaderSection({ editor }: { editor: TemplateEditorState }) {
  const { form, errorOf, clearExtra } = editor
  const type = useStore(form.store, (s) => s.values.header.type)
  const headerText = useStore(form.store, (s) => s.values.header.text)
  const hasVariable = headerText.includes('{{')

  return (
    <FormSection id="sec-header" title="2. Header" aside="Optional">
      <form.Field name="header.type">
        {(field) => (
          <ToggleGroup
            type="single"
            variant="outline"
            aria-label="Header type"
            className="flex flex-wrap justify-start gap-0.5 self-start rounded-xl bg-muted p-1"
            spacing={1}
            value={field.state.value}
            onValueChange={(v) => {
              if (!v) return
              field.handleChange(v as HeaderType)
              clearExtra('header')
            }}
          >
            {HEADER_TYPES.map((h) => (
              <ToggleGroupItem
                key={h.value}
                value={h.value}
                className="h-9 rounded-[9px] border-transparent bg-transparent px-3.5 text-[13px] font-bold text-muted-foreground hover:text-foreground data-[state=on]:bg-card data-[state=on]:text-foreground data-[state=on]:shadow-card"
              >
                {h.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
        )}
      </form.Field>

      {type === 'text' && (
        <>
          <form.Field name="header.text">
            {(field) => {
              const error = errorOf(field)
              return (
                <FormField
                  id="tpl-header-text"
                  label="Header text"
                  error={error}
                  labelAside={
                    <CharCounter
                      id="tpl-header-text-count"
                      value={field.state.value}
                      max={LIMITS.headerText}
                    />
                  }
                  description="You can add one variable, {{1}}."
                >
                  <Input
                    id="tpl-header-text"
                    maxLength={LIMITS.headerText}
                    placeholder="e.g. Your order is on the way"
                    value={field.state.value}
                    onChange={(e) => {
                      field.handleChange(e.target.value)
                      clearExtra('header.text', 'header.example')
                    }}
                    onBlur={field.handleBlur}
                    aria-invalid={!!error}
                    aria-describedby={describedBy('tpl-header-text', { error, hint: true })}
                  />
                </FormField>
              )
            }}
          </form.Field>
          {hasVariable && (
            <form.Field name="header.example">
              {(field) => {
                const error = errorOf(field)
                return (
                  <FormField
                    id="tpl-header-example"
                    label="Sample for {{1}} in the header"
                    error={error}
                  >
                    <Input
                      id="tpl-header-example"
                      placeholder="Sample value"
                      value={field.state.value}
                      onChange={(e) => {
                        field.handleChange(e.target.value)
                        clearExtra('header.example')
                      }}
                      onBlur={field.handleBlur}
                      aria-invalid={!!error}
                      aria-describedby={describedBy('tpl-header-example', { error })}
                    />
                  </FormField>
                )
              }}
            </form.Field>
          )}
        </>
      )}

      {isMediaHeader(type) && <MediaSample editor={editor} type={type} />}
    </FormSection>
  )
}

function MediaSample({ editor, type }: { editor: TemplateEditorState; type: MediaHeaderType }) {
  const { current, localSample, extra, clearExtra, setExtraError, run, busy } = editor
  const input = useRef<HTMLInputElement>(null)
  const Icon = MEDIA_ICON[type]
  const sample = current?.header_sample
  const fits = sample && sampleKind(sample.content_type) === type
  const error = extra.header
  const uploading = busy === 'upload'

  return (
    <div className="flex flex-col gap-2">
      <div className="flex flex-wrap items-center gap-3.5 rounded-xl border-2 border-dashed border-input bg-muted p-4">
        <span className="flex size-12 flex-none items-center justify-center rounded-xl bg-card text-brand-text">
          {fits ? (
            <Icon className="size-[22px]" aria-hidden="true" />
          ) : (
            <UploadIcon className="size-[22px]" aria-hidden="true" />
          )}
        </span>
        <span className="flex min-w-0 flex-[1_1_200px] flex-col leading-snug">
          <span className="truncate text-sm font-bold">
            {fits ? (localSample?.name ?? `Sample ${type} uploaded`) : `No sample ${type} yet`}
          </span>
          <span id="tpl-header-sample-hint" className="text-xs text-muted-foreground">
            Sample for Meta’s review · {MEDIA[type].hint}
            {fits && ` · ${formatBytes(sample.size)}`}
            {!current && ' · Uploading saves a draft first'}
          </span>
        </span>
        <Button
          variant="outline"
          size="sm"
          disabled={uploading}
          aria-describedby={['tpl-header-sample-hint', error && 'tpl-header-sample-error']
            .filter(Boolean)
            .join(' ')}
          aria-invalid={!!error}
          onClick={() => input.current?.click()}
        >
          <UploadIcon aria-hidden="true" />
          {uploading ? 'Uploading…' : fits ? 'Replace sample' : 'Upload sample'}
        </Button>
        <Input
          unstyled
          ref={input}
          type="file"
          tabIndex={-1}
          aria-hidden="true"
          className="sr-only"
          accept={MEDIA[type].accept}
          onChange={(e) => {
            const file = e.target.files?.[0]
            e.target.value = ''
            if (!file) return
            clearExtra('header')
            const problem = sampleFileProblem(type, file)
            if (problem) return setExtraError('header', problem)
            void run({ action: 'upload', file })
          }}
        />
      </div>
      {error && (
        <p id="tpl-header-sample-error" className="text-sm text-destructive">
          {error}
        </p>
      )}
    </div>
  )
}
