import { describedBy, FormField } from '#/components/form'
import { Input } from '#/components/ui/input'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { cn } from '#/lib/utils'
import type { TemplateEditorState } from '../../hooks/use-template-editor'
import { CATEGORIES, LANGUAGES, toTemplateName } from '../../schemas/templates.schema'
import { FormSection } from '../kit/form-section'

/** 1. Basics: name, language and category. Name and language lock once Meta has the template. */
export function BasicsSection({
  editor,
  locked,
}: {
  editor: TemplateEditorState
  locked: boolean
}) {
  const { form, errorOf, clearExtra } = editor
  const lockedHint = 'Meta keeps the name and language of a submitted template.'

  return (
    <FormSection id="sec-basics" title="1. Basics">
      <div className="grid gap-4 sm:grid-cols-2">
        <form.Field name="name">
          {(field) => {
            const error = errorOf(field)
            return (
              <FormField
                id="tpl-name"
                label="Template name"
                error={error}
                description={
                  locked ? lockedHint : 'Lowercase letters, numbers and underscores only.'
                }
              >
                <Input
                  id="tpl-name"
                  autoComplete="off"
                  spellCheck={false}
                  placeholder="e.g. order_shipped_update"
                  className="font-mono text-[13px] md:text-[13px]"
                  disabled={locked}
                  value={field.state.value}
                  onChange={(e) => {
                    field.handleChange(toTemplateName(e.target.value))
                    clearExtra('name')
                  }}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy('tpl-name', { error, hint: true })}
                />
              </FormField>
            )
          }}
        </form.Field>
        <form.Field name="language">
          {(field) => {
            const error = errorOf(field)
            const known = LANGUAGES.some((l) => l.value === field.state.value)
            return (
              <FormField id="tpl-language" label="Language" error={error}>
                <Select
                  value={field.state.value}
                  disabled={locked}
                  onValueChange={(v) => {
                    field.handleChange(v)
                    field.handleBlur()
                    clearExtra('language', 'name')
                  }}
                >
                  <SelectTrigger
                    id="tpl-language"
                    className="w-full"
                    aria-invalid={!!error}
                    aria-describedby={describedBy('tpl-language', { error })}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {!known && (
                      <SelectItem value={field.state.value}>{field.state.value}</SelectItem>
                    )}
                    {LANGUAGES.map((l) => (
                      <SelectItem key={l.value} value={l.value}>
                        {l.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </FormField>
            )
          }}
        </form.Field>
      </div>

      <form.Field name="category">
        {(field) => {
          const error = errorOf(field)
          return (
            <fieldset
              className="flex flex-col gap-2"
              aria-describedby={error ? 'tpl-category-error' : undefined}
            >
              <legend className="mb-2 text-sm font-bold">Category</legend>
              <div className="grid gap-2.5 sm:grid-cols-3">
                {CATEGORIES.map((c) => {
                  const on = field.state.value === c.value
                  return (
                    <label
                      key={c.value}
                      className={cn(
                        'relative flex min-h-[72px] cursor-pointer flex-col gap-0.5 rounded-lg border-2 px-3.5 py-3 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
                        on
                          ? 'border-primary bg-brand-soft'
                          : 'border-border bg-card hover:bg-muted',
                      )}
                    >
                      <input
                        type="radio"
                        name="tpl-category"
                        value={c.value}
                        className="sr-only"
                        checked={on}
                        onChange={() => {
                          field.handleChange(c.value)
                          clearExtra('category')
                        }}
                      />
                      <span className="text-sm font-bold">{c.label}</span>
                      <span className="text-xs text-muted-foreground">{c.hint}</span>
                    </label>
                  )
                })}
              </div>
              {error && (
                <p id="tpl-category-error" className="text-sm text-destructive">
                  {error}
                </p>
              )}
            </fieldset>
          )
        }}
      </form.Field>
    </FormSection>
  )
}
