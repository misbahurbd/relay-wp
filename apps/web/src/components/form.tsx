import type { ReactNode } from 'react'
import { Field, FieldDescription, FieldError, FieldLabel } from '#/components/ui/field'

type FieldLike = {
  state: { meta: { isBlurred: boolean; errors: unknown[] } }
  form: { state: { submissionAttempts: number } }
}

/**
 * The field's first validation message, shown once the user has left the field or tried to
 * submit. Pair with revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }).
 */
export function fieldError(field: FieldLike): string | undefined {
  const { meta } = field.state
  if (!meta.isBlurred && field.form.state.submissionAttempts === 0) return undefined
  for (const e of meta.errors) {
    if (typeof e === 'string') return e
    if (e && typeof e === 'object' && 'message' in e && typeof e.message === 'string') {
      return e.message
    }
  }
  return undefined
}

/** Moves focus to the first invalid control after a failed submit. */
export function focusFirstInvalid(form: HTMLFormElement | null) {
  requestAnimationFrame(() => {
    form?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  })
}

type FormFieldProps = {
  id: string
  label: ReactNode
  error?: string
  description?: ReactNode
  /** Rendered between the label and the control, e.g. a "Forgot password?" link. */
  labelAside?: ReactNode
  children: ReactNode
}

/**
 * Label, control, hint and error in the order the design system asks for: errors below the
 * control. The control must set id={id}, aria-invalid and aria-describedby={describedBy(id, …)}.
 */
export function FormField({ id, label, error, description, labelAside, children }: FormFieldProps) {
  return (
    <Field data-invalid={error ? true : undefined}>
      <div className="flex items-baseline justify-between gap-2">
        <FieldLabel htmlFor={id}>{label}</FieldLabel>
        {labelAside}
      </div>
      {children}
      {typeof description === 'string' ? (
        <FieldDescription id={`${id}-hint`}>{description}</FieldDescription>
      ) : (
        // Rich hints (like the password rules list) can't sit inside FieldDescription's <p>.
        description && (
          <div
            id={`${id}-hint`}
            data-slot="field-description"
            className="text-sm text-muted-foreground"
          >
            {description}
          </div>
        )
      )}
      {error && <FieldError id={`${id}-error`}>{error}</FieldError>}
    </Field>
  )
}

/** aria-describedby value for a FormField control. */
export function describedBy(id: string, { error, hint }: { error?: string; hint?: boolean }) {
  return [hint && `${id}-hint`, error && `${id}-error`].filter(Boolean).join(' ') || undefined
}
