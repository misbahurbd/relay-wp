import { useStore } from '@tanstack/react-form'
import { PlusIcon, XIcon } from 'lucide-react'
import { describedBy, FormField } from '#/components/form'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { cn } from '#/lib/utils'
import type { TemplateEditorState } from '../../hooks/use-template-editor'
import { BUTTON_TYPES, LIMITS, shortLinkWarnings } from '../../schemas/templates.schema'
import type { ButtonType } from '../../types'
import { BUTTON_ICON } from '../kit/button-icon'
import { FormSection } from '../kit/form-section'
import { WarningNote } from '../kit/warning-note'

const TYPE_TONE: Record<ButtonType, string> = {
  quick_reply: 'text-foreground',
  url: 'text-info',
  phone: 'text-brand-text',
}

/** 5. Buttons: up to 10, at most 2 website and 1 phone, quick replies kept together. */
export function ButtonsSection({ editor }: { editor: TemplateEditorState }) {
  const { form, errorOf, clearExtra } = editor
  const warnings = useStore(form.store, (s) => shortLinkWarnings(s.values))

  return (
    <FormSection id="sec-buttons" title="5. Buttons" aside="Optional · up to 10">
      <form.Field name="buttons" mode="array">
        {(list) => {
          const buttons = list.state.value
          const count = (t: ButtonType) => buttons.filter((b) => b.type === t).length
          const full = buttons.length >= LIMITS.buttons
          const listError = errorOf(list)

          function add(type: ButtonType) {
            const button = { type, text: '', value: '' }
            // New quick replies join the others, since Meta wants them grouped.
            const lastQuick = buttons.map((b) => b.type).lastIndexOf('quick_reply')
            if (type === 'quick_reply' && lastQuick >= 0) list.insertValue(lastQuick + 1, button)
            else list.pushValue(button)
            clearExtra('buttons')
          }

          return (
            <>
              {buttons.length > 0 && (
                <ul className="flex flex-col gap-3">
                  {buttons.map((b, i) => {
                    const info = BUTTON_TYPES[b.type]
                    const Icon = BUTTON_ICON[b.type]
                    return (
                      // biome-ignore lint/suspicious/noArrayIndexKey: rows are addressed by index in the form
                      <li key={i} className="flex flex-col gap-3 rounded-xl border bg-card p-3.5">
                        <div className="flex items-center justify-between gap-2">
                          <span
                            className={cn(
                              'inline-flex items-center gap-1.5 text-[13px] font-bold',
                              TYPE_TONE[b.type],
                            )}
                          >
                            <Icon className="size-4" aria-hidden="true" />
                            {info.label}
                          </span>
                          <Button
                            variant="ghost"
                            size="icon-sm"
                            aria-label={`Remove button ${b.text || info.label} (${i + 1})`}
                            onClick={() => {
                              list.removeValue(i)
                              clearExtra('buttons')
                            }}
                          >
                            <XIcon aria-hidden="true" />
                          </Button>
                        </div>
                        <div className="grid gap-3 sm:grid-cols-2">
                          <form.Field name={`buttons[${i}].text`}>
                            {(field) => {
                              const id = `tpl-button-${i}-text`
                              const error = errorOf(field)
                              return (
                                <FormField id={id} label="Button text" error={error}>
                                  <Input
                                    id={id}
                                    maxLength={LIMITS.buttonText}
                                    placeholder={
                                      b.type === 'quick_reply'
                                        ? 'e.g. Yes, interested'
                                        : 'e.g. Track order'
                                    }
                                    value={field.state.value}
                                    onChange={(e) => {
                                      field.handleChange(e.target.value)
                                      clearExtra(`buttons[${i}].text`)
                                    }}
                                    onBlur={field.handleBlur}
                                    aria-invalid={!!error}
                                    aria-describedby={describedBy(id, { error })}
                                  />
                                </FormField>
                              )
                            }}
                          </form.Field>
                          {b.type !== 'quick_reply' && (
                            <form.Field name={`buttons[${i}].value`}>
                              {(field) => {
                                const id = `tpl-button-${i}-value`
                                const error = errorOf(field)
                                return (
                                  <FormField id={id} label={info.valueLabel} error={error}>
                                    <Input
                                      id={id}
                                      type={b.type === 'phone' ? 'tel' : 'url'}
                                      inputMode={b.type === 'phone' ? 'tel' : 'url'}
                                      autoComplete="off"
                                      placeholder={info.placeholder}
                                      value={field.state.value}
                                      onChange={(e) => {
                                        field.handleChange(e.target.value)
                                        clearExtra(`buttons[${i}].value`)
                                      }}
                                      onBlur={field.handleBlur}
                                      aria-invalid={!!error}
                                      aria-describedby={describedBy(id, { error })}
                                    />
                                  </FormField>
                                )
                              }}
                            </form.Field>
                          )}
                        </div>
                        {warnings[`buttons[${i}].value`] && (
                          <WarningNote>{warnings[`buttons[${i}].value`]}</WarningNote>
                        )}
                      </li>
                    )
                  })}
                </ul>
              )}
              {listError && (
                <p role="alert" className="text-sm text-destructive">
                  {listError}
                </p>
              )}
              <div className="flex flex-wrap gap-2">
                <AddButton type="quick_reply" disabled={full} onAdd={add} />
                <AddButton
                  type="url"
                  disabled={full || count('url') >= LIMITS.urlButtons}
                  onAdd={add}
                />
                <AddButton
                  type="phone"
                  disabled={full || count('phone') >= LIMITS.phoneButtons}
                  onAdd={add}
                />
              </div>
              <p className="text-xs text-muted-foreground">
                Up to {LIMITS.buttons} buttons: at most {LIMITS.urlButtons} website buttons and{' '}
                {LIMITS.phoneButtons} phone button. Quick replies stay together.
              </p>
            </>
          )
        }}
      </form.Field>
    </FormSection>
  )
}

function AddButton({
  type,
  disabled,
  onAdd,
}: {
  type: ButtonType
  disabled: boolean
  onAdd: (type: ButtonType) => void
}) {
  return (
    <Button
      variant="outline"
      size="sm"
      className="h-10 border-dashed bg-transparent px-3.5"
      disabled={disabled}
      onClick={() => onAdd(type)}
    >
      <PlusIcon aria-hidden="true" />
      {BUTTON_TYPES[type].add}
    </Button>
  )
}
