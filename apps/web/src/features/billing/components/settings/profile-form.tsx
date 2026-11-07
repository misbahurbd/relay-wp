import { revalidateLogic, useForm } from '@tanstack/react-form'
import { type ReactNode, useRef, useState } from 'react'
import { toast } from 'sonner'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { errorMessage, isApiError } from '#/lib/api'
import { useUpdateProfile } from '../../api/billing.mutations'
import { type BillingProfileValues, billingProfileSchema } from '../../schemas/billing.schema'
import type { BillingProfile, BillingProfileInput } from '../../types'

type Key = keyof BillingProfileValues

const FIELDS: {
  name: Key
  label: string
  autoComplete?: string
  type?: string
  hint?: string
  placeholder?: string
}[] = [
  {
    name: 'legal_name',
    label: 'Name on invoices',
    autoComplete: 'organization',
    hint: 'A person or a company.',
  },
  {
    name: 'email',
    label: 'Billing email',
    type: 'email',
    autoComplete: 'email',
    hint: 'Invoices and payment emails go here.',
  },
  { name: 'phone', label: 'Phone (optional)', type: 'tel', autoComplete: 'tel' },
  { name: 'address_line1', label: 'Address (optional)', autoComplete: 'address-line1' },
  { name: 'address_line2', label: 'Address line 2 (optional)', autoComplete: 'address-line2' },
  { name: 'city', label: 'City (optional)', autoComplete: 'address-level2' },
  { name: 'state', label: 'State or region (optional)', autoComplete: 'address-level1' },
  { name: 'postal_code', label: 'Postal code (optional)', autoComplete: 'postal-code' },
  {
    name: 'country',
    label: 'Country (optional)',
    autoComplete: 'country',
    placeholder: 'e.g. US',
    hint: 'The 2-letter code.',
  },
  { name: 'tax_id_type', label: 'Tax ID type (optional)', placeholder: 'e.g. eu_vat' },
  {
    name: 'tax_id',
    label: 'Tax ID (optional)',
    hint: 'Shown on future invoices. Past invoices keep the details they were issued with.',
  },
]

const REQUIRED: Key[] = ['legal_name', 'email']

function toValues(p: BillingProfile | null): BillingProfileValues {
  return {
    legal_name: p?.legal_name ?? '',
    email: p?.email ?? '',
    phone: p?.phone ?? '',
    address_line1: p?.address_line1 ?? '',
    address_line2: p?.address_line2 ?? '',
    city: p?.city ?? '',
    state: p?.state ?? '',
    postal_code: p?.postal_code ?? '',
    country: p?.country ?? '',
    tax_id_type: p?.tax_id_type ?? '',
    tax_id: p?.tax_id ?? '',
  }
}

function toInput(v: BillingProfileValues): BillingProfileInput {
  const opt = (s: string) => s.trim() || undefined
  return {
    legal_name: v.legal_name.trim(),
    email: v.email.trim(),
    phone: opt(v.phone),
    address_line1: opt(v.address_line1),
    address_line2: opt(v.address_line2),
    city: opt(v.city),
    state: opt(v.state),
    postal_code: opt(v.postal_code),
    country: opt(v.country.toUpperCase()),
    tax_id_type: opt(v.tax_id_type),
    tax_id: opt(v.tax_id),
  }
}

function sameInput(a: BillingProfileInput, b: BillingProfileInput) {
  return (Object.keys(a) as (keyof BillingProfileInput)[]).every((k) => a[k] === b[k])
}

/**
 * The billing details form's state. A save that changes nothing skips the request, so a caller
 * can submit on every "continue" without a pointless write. `onSaved` gets the stored profile.
 */
export function useProfileForm({
  workspaceId,
  initial,
  onSaved,
}: {
  workspaceId: string
  initial: BillingProfile | null
  onSaved?: (profile: BillingProfile) => void
}) {
  const update = useUpdateProfile(workspaceId)
  const formRef = useRef<HTMLFormElement>(null)
  const form = useForm({
    defaultValues: toValues(initial),
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: billingProfileSchema },
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value }) => {
      const input = toInput(value)
      if (initial && sameInput(input, toInput(toValues(initial)))) {
        onSaved?.(initial)
        return
      }
      try {
        onSaved?.(await update.mutateAsync(input))
      } catch {
        // The form shows the error from `update`.
      }
    },
  })
  return { form, formRef, update }
}

export type ProfileFormState = ReturnType<typeof useProfileForm>

/** The fields, the API's error and a form element; `children` (the submit button) sit inside it. */
export function ProfileFields({
  profile: { form, formRef, update },
  collapsible,
  children,
}: {
  profile: ProfileFormState
  /** Show only the required fields until the member asks for the rest (or already has some). */
  collapsible?: boolean
  children?: ReactNode
}) {
  const [expanded, setExpanded] = useState(
    () =>
      !collapsible ||
      FIELDS.some((f) => !REQUIRED.includes(f.name) && form.state.values[f.name] !== ''),
  )
  const details = isApiError(update.error) ? (update.error.details ?? []) : []

  return (
    <form
      ref={formRef}
      noValidate
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        form.handleSubmit()
      }}
    >
      {update.isError && (
        <Alert variant="danger">
          <AlertDescription>
            <p>{errorMessage(update.error)}</p>
            {details.length > 0 && (
              <ul className="list-disc pl-4">
                {details.map((d) => (
                  <li key={`${d.field}-${d.message}`}>{d.message}</li>
                ))}
              </ul>
            )}
          </AlertDescription>
        </Alert>
      )}
      {FIELDS.filter((f) => expanded || REQUIRED.includes(f.name)).map((f) => (
        <form.Field key={f.name} name={f.name}>
          {(field) => {
            const id = `billing-${f.name}`
            const error = fieldError(field)
            return (
              <FormField id={id} label={f.label} error={error} description={f.hint}>
                <Input
                  id={id}
                  name={f.name}
                  type={f.type ?? 'text'}
                  autoComplete={f.autoComplete}
                  placeholder={f.placeholder}
                  value={field.state.value}
                  onChange={(e) => field.handleChange(e.target.value)}
                  onBlur={field.handleBlur}
                  aria-invalid={!!error}
                  aria-describedby={describedBy(id, { error, hint: !!f.hint })}
                />
              </FormField>
            )
          }}
        </form.Field>
      ))}
      {!expanded && (
        <Button
          type="button"
          variant="link"
          className="h-auto self-start p-0"
          aria-expanded={false}
          onClick={() => setExpanded(true)}
        >
          Add address, country and tax ID (optional)
        </Button>
      )}
      {children}
    </form>
  )
}

/** The billing details form. Past invoices keep the details they were issued with. */
export function ProfileForm({
  workspaceId,
  initial,
  onSaved,
}: {
  workspaceId: string
  initial: BillingProfile | null
  onSaved?: () => void
}) {
  const profile = useProfileForm({
    workspaceId,
    initial,
    onSaved: () => {
      toast.success('Billing details saved')
      onSaved?.()
    },
  })
  return (
    <ProfileFields profile={profile}>
      <profile.form.Subscribe selector={(s) => s.isSubmitting}>
        {(submitting) => (
          <Button type="submit" size="lg" className="self-start" disabled={submitting}>
            {submitting ? 'Saving…' : 'Save billing details'}
          </Button>
        )}
      </profile.form.Subscribe>
    </ProfileFields>
  )
}
