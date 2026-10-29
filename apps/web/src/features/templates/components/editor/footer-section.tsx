import { describedBy, FormField } from '#/components/form'
import { Input } from '#/components/ui/input'
import type { TemplateEditorState } from '../../hooks/use-template-editor'
import { LIMITS } from '../../schemas/templates.schema'
import { CharCounter } from '../kit/char-counter'
import { FormSection } from '../kit/form-section'

/** 4. Footer: small gray text under the message, no variables. */
export function FooterSection({ editor }: { editor: TemplateEditorState }) {
  const { form, errorOf, clearExtra } = editor
  return (
    <FormSection id="sec-footer" title="4. Footer" aside="Optional · small gray text">
      <form.Field name="footer.text">
        {(field) => {
          const error = errorOf(field)
          return (
            <FormField
              id="tpl-footer"
              label="Footer text"
              error={error}
              labelAside={<CharCounter value={field.state.value} max={LIMITS.footer} />}
            >
              <Input
                id="tpl-footer"
                maxLength={LIMITS.footer}
                placeholder="e.g. Reply STOP to unsubscribe"
                value={field.state.value}
                onChange={(e) => {
                  field.handleChange(e.target.value)
                  clearExtra('footer')
                }}
                onBlur={field.handleBlur}
                aria-invalid={!!error}
                aria-describedby={describedBy('tpl-footer', { error })}
              />
            </FormField>
          )
        }}
      </form.Field>
    </FormSection>
  )
}
