import { useStore } from '@tanstack/react-form'
import { BoldIcon, ItalicIcon, PlusIcon } from 'lucide-react'
import { useRef } from 'react'
import { describedBy, FormField } from '#/components/form'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Textarea } from '#/components/ui/textarea'
import type { TemplateEditorState } from '../../hooks/use-template-editor'
import {
  charCount,
  exampleSlots,
  LIMITS,
  resizeExamples,
  shortLinkWarnings,
} from '../../schemas/templates.schema'
import { CharCounter } from '../kit/char-counter'
import { FormSection } from '../kit/form-section'
import { WarningNote } from '../kit/warning-note'

/** 3. Body: the message, formatting and variable tools, and one sample per {{n}}. */
export function BodySection({ editor }: { editor: TemplateEditorState }) {
  const { form, errorOf, clearExtra, extra } = editor
  const area = useRef<HTMLTextAreaElement>(null)
  const examples = useStore(form.store, (s) => s.values.body.examples)
  const warning = useStore(form.store, (s) => shortLinkWarnings(s.values)['body.text'])

  function setBody(text: string) {
    form.setFieldValue('body.text', text)
    form.setFieldValue('body.examples', resizeExamples(form.getFieldValue('body.examples'), text))
    clearExtra('body')
  }

  /** Wraps the selection (or inserts at the cursor) and keeps the cursor after the insert. */
  function insert(before: string, after = '') {
    const el = area.current
    const text = form.getFieldValue('body.text')
    const start = el?.selectionStart ?? text.length
    const end = el?.selectionEnd ?? text.length
    const next = text.slice(0, start) + before + text.slice(start, end) + after + text.slice(end)
    if (charCount(next) > LIMITS.body) return
    setBody(next)
    requestAnimationFrame(() => {
      el?.focus()
      // Inside the markers when nothing was selected; the wrapped text stays selected otherwise.
      const from = start + before.length
      el?.setSelectionRange(from, start === end ? from : end + before.length)
    })
  }

  return (
    <FormSection
      id="sec-body"
      title="3. Body"
      aside="Required"
      tools={
        <div role="toolbar" aria-label="Body formatting" className="flex flex-wrap gap-1.5">
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Bold"
            onClick={() => insert('*', '*')}
          >
            <BoldIcon aria-hidden="true" />
          </Button>
          <Button
            variant="outline"
            size="icon-sm"
            aria-label="Italic"
            onClick={() => insert('_', '_')}
          >
            <ItalicIcon aria-hidden="true" />
          </Button>
          <Button
            variant="soft"
            size="sm"
            className="border border-primary"
            onClick={() => insert(`{{${exampleSlots(form.getFieldValue('body.text')) + 1}}}`)}
          >
            <PlusIcon aria-hidden="true" />
            Add variable
          </Button>
        </div>
      }
    >
      <form.Field name="body.text">
        {(field) => {
          const error = errorOf(field)
          return (
            <FormField
              id="tpl-body"
              label={<span className="sr-only">Message text</span>}
              error={error}
              description={
                <span className="flex flex-wrap justify-between gap-2 text-xs">
                  <span>
                    Use {'{{1}}'}, {'{{2}}'}… for details that change per customer, like name or
                    order number.
                  </span>
                  <CharCounter value={field.state.value} max={LIMITS.body} />
                </span>
              }
            >
              <Textarea
                ref={area}
                id="tpl-body"
                rows={6}
                maxLength={LIMITS.body}
                className="min-h-36 leading-relaxed"
                placeholder="Hi {{1}}, your order {{2}} has shipped."
                value={field.state.value}
                onChange={(e) => setBody(e.target.value)}
                onBlur={field.handleBlur}
                aria-invalid={!!error}
                aria-describedby={describedBy('tpl-body', { error, hint: true })}
              />
            </FormField>
          )
        }}
      </form.Field>
      {warning && <WarningNote>{warning}</WarningNote>}

      {examples.length > 0 && (
        <div className="flex flex-col gap-2.5 rounded-xl bg-muted p-3.5">
          <p className="text-[13px] font-bold">
            Sample values{' '}
            <span className="font-medium text-muted-foreground">
              · Meta needs examples to review your template
            </span>
          </p>
          {extra['body.examples'] && (
            <p className="text-sm text-destructive">{extra['body.examples']}</p>
          )}
          <div className="grid gap-2.5 sm:grid-cols-2">
            {examples.map((_, i) => (
              <form.Field
                // biome-ignore lint/suspicious/noArrayIndexKey: one input per variable number
                key={i}
                name={`body.examples[${i}]`}
              >
                {(field) => {
                  const id = `tpl-example-${i + 1}`
                  const token = `{{${i + 1}}}`
                  const error = errorOf(field)
                  return (
                    <div className="flex flex-col gap-1">
                      <div className="flex items-center gap-2">
                        <span
                          aria-hidden="true"
                          className="inline-flex h-8 min-w-11 flex-none items-center justify-center rounded-lg bg-info-soft px-1.5 font-mono text-xs font-bold text-info"
                        >
                          {token}
                        </span>
                        <label htmlFor={id} className="sr-only">
                          Sample for {token}
                        </label>
                        <Input
                          id={id}
                          className="h-10"
                          placeholder="Sample value"
                          value={field.state.value}
                          onChange={(e) => {
                            field.handleChange(e.target.value)
                            clearExtra(`body.examples[${i}]`, 'body.examples')
                          }}
                          onBlur={field.handleBlur}
                          aria-invalid={!!error}
                          aria-describedby={describedBy(id, { error })}
                        />
                      </div>
                      {error && (
                        <p id={`${id}-error`} className="text-sm text-destructive">
                          {error}
                        </p>
                      )}
                    </div>
                  )
                }}
              </form.Field>
            ))}
          </div>
        </div>
      )}
    </FormSection>
  )
}
