import { useStore } from '@tanstack/react-form'
import { InfoIcon } from 'lucide-react'
import { describedBy, FormField } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Checkbox } from '#/components/ui/checkbox'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { cn } from '#/lib/utils'
import type { TemplateEditorState } from '../../hooks/use-template-editor'
import { LIMITS, OTP_TYPES } from '../../schemas/templates.schema'
import { CharCounter } from '../kit/char-counter'
import { FormSection } from '../kit/form-section'

/**
 * 2–3. An authentication template's options. Meta writes the message and the button in the
 * template's language, so the code, security line, expiry and button are all there is to pick.
 */
export function AuthSection({ editor }: { editor: TemplateEditorState }) {
  return (
    <>
      <MessageOptions editor={editor} />
      <ButtonOptions editor={editor} />
    </>
  )
}

function MessageOptions({ editor }: { editor: TemplateEditorState }) {
  const { form, errorOf, clearExtra } = editor
  return (
    <FormSection id="sec-auth-message" title="2. Message">
      <Alert variant="info">
        <InfoIcon aria-hidden="true" />
        <AlertDescription>
          Meta writes the text of one-time passcode messages, translated into the template’s
          language. You send the code when you use the template.
        </AlertDescription>
      </Alert>

      <form.Field name="auth.security_recommendation">
        {(field) => (
          <Label className="items-start gap-2.5 leading-snug font-normal">
            <Checkbox
              className="mt-0.5"
              checked={field.state.value}
              onCheckedChange={(v) => field.handleChange(v === true)}
              onBlur={field.handleBlur}
            />
            <span className="flex flex-col gap-0.5">
              <span className="font-bold">Add a security warning</span>
              <span className="text-muted-foreground">
                “For your security, do not share this code.”
              </span>
            </span>
          </Label>
        )}
      </form.Field>

      <form.Field name="auth.code_expiry_minutes">
        {(field) => {
          const error = errorOf(field)
          const on = field.state.value > 0
          return (
            <div className="flex flex-col gap-3">
              <Label className="items-start gap-2.5 leading-snug font-normal">
                <Checkbox
                  className="mt-0.5"
                  checked={on}
                  onCheckedChange={(v) => {
                    field.handleChange(v === true ? 10 : 0)
                    clearExtra('auth.code_expiry_minutes')
                  }}
                />
                <span className="flex flex-col gap-0.5">
                  <span className="font-bold">Say when the code expires</span>
                  <span className="text-muted-foreground">
                    Adds “This code expires in N minutes.” under the message.
                  </span>
                </span>
              </Label>
              {on && (
                <FormField
                  id="tpl-auth-expiry"
                  label="Expires after (minutes)"
                  error={error}
                  description={`1 to ${LIMITS.codeExpiry} minutes`}
                >
                  <Input
                    id="tpl-auth-expiry"
                    type="number"
                    inputMode="numeric"
                    min={1}
                    max={LIMITS.codeExpiry}
                    className="w-32"
                    value={Number.isNaN(field.state.value) ? '' : field.state.value}
                    onChange={(e) => {
                      field.handleChange(e.target.valueAsNumber)
                      clearExtra('auth.code_expiry_minutes')
                    }}
                    onBlur={field.handleBlur}
                    aria-invalid={!!error}
                    aria-describedby={describedBy('tpl-auth-expiry', { error, hint: true })}
                  />
                </FormField>
              )}
            </div>
          )
        }}
      </form.Field>
    </FormSection>
  )
}

function ButtonOptions({ editor }: { editor: TemplateEditorState }) {
  const { form, clearExtra } = editor
  const oneTap = useStore(form.store, (s) => s.values.auth.otp_type === 'one_tap')

  return (
    <FormSection id="sec-auth-button" title="3. Button">
      <form.Field name="auth.otp_type">
        {(field) => (
          <fieldset className="flex flex-col gap-2">
            <legend className="mb-2 text-sm font-bold">Button type</legend>
            <div className="grid gap-2.5 sm:grid-cols-2">
              {OTP_TYPES.map((t) => {
                const on = field.state.value === t.value
                return (
                  <label
                    key={t.value}
                    className={cn(
                      'flex min-h-[72px] cursor-pointer flex-col gap-0.5 rounded-lg border-2 px-3.5 py-3 has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
                      on ? 'border-primary bg-brand-soft' : 'border-border bg-card hover:bg-muted',
                    )}
                  >
                    <input
                      type="radio"
                      name="tpl-auth-otp-type"
                      value={t.value}
                      className="sr-only"
                      checked={on}
                      onChange={() => {
                        field.handleChange(t.value)
                        clearExtra('auth')
                      }}
                    />
                    <span className="text-sm font-bold">{t.label}</span>
                    <span className="text-xs text-muted-foreground">{t.hint}</span>
                  </label>
                )
              })}
            </div>
          </fieldset>
        )}
      </form.Field>

      <div className="grid gap-4 sm:grid-cols-2">
        <TextOption
          editor={editor}
          name="auth.button_text"
          id="tpl-auth-button"
          label="Copy code button text"
          max={LIMITS.buttonText}
        />
        {oneTap && (
          <TextOption
            editor={editor}
            name="auth.autofill_text"
            id="tpl-auth-autofill"
            label="Autofill button text"
            max={LIMITS.buttonText}
          />
        )}
      </div>

      {oneTap && (
        <div className="grid gap-4 sm:grid-cols-2">
          <TextOption
            editor={editor}
            name="auth.package_name"
            id="tpl-auth-package"
            label="Android package name"
            placeholder="com.example.app"
            mono
          />
          <TextOption
            editor={editor}
            name="auth.signature_hash"
            id="tpl-auth-hash"
            label="App signature hash"
            placeholder="K8a/AINcGX7"
            hint="The 11-character hash of your app’s signing key."
            mono
          />
        </div>
      )}
    </FormSection>
  )
}

type TextName =
  | 'auth.button_text'
  | 'auth.autofill_text'
  | 'auth.package_name'
  | 'auth.signature_hash'

function TextOption({
  editor,
  name,
  id,
  label,
  max,
  placeholder,
  hint,
  mono,
}: {
  editor: TemplateEditorState
  name: TextName
  id: string
  label: string
  max?: number
  placeholder?: string
  hint?: string
  mono?: boolean
}) {
  const { form, errorOf, clearExtra } = editor
  return (
    <form.Field name={name}>
      {(field) => {
        const error = errorOf(field)
        return (
          <FormField
            id={id}
            label={label}
            error={error}
            description={
              max ? (
                <span className="flex justify-end">
                  <CharCounter value={field.state.value} max={max} />
                </span>
              ) : (
                hint
              )
            }
          >
            <Input
              id={id}
              autoComplete="off"
              spellCheck={false}
              maxLength={max}
              placeholder={placeholder}
              className={cn(mono && 'font-mono text-[13px] md:text-[13px]')}
              value={field.state.value}
              onChange={(e) => {
                field.handleChange(e.target.value)
                clearExtra(name)
              }}
              onBlur={field.handleBlur}
              aria-invalid={!!error}
              aria-describedby={describedBy(id, { error, hint: !!(max || hint) })}
            />
          </FormField>
        )
      }}
    </form.Field>
  )
}
