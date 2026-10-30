import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { useRef } from 'react'
import { toast } from 'sonner'
import { describedBy, FormField, fieldError, focusFirstInvalid } from '#/components/form'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage, isApiError } from '#/lib/api'
import { useUpdateProfile } from '../../api/billing.mutations'
import { profileQueryOptions } from '../../api/billing.queries'
import { type BillingProfileValues, billingProfileSchema } from '../../schemas/billing.schema'
import type { BillingProfile, BillingProfileInput } from '../../types'
import { Section, SectionHeader } from '../kit/section'
import { ErrorState, NoPayPermission } from '../kit/states'

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

export function ProfileSection({ workspaceId, canPay }: { workspaceId: string; canPay: boolean }) {
  const profile = useQuery(profileQueryOptions(workspaceId))
  return (
    <Section aria-labelledby="profile-heading">
      <SectionHeader
        id="profile-heading"
        title="Billing details"
        description="Who invoices are made out to."
      />
      {profile.isPending ? (
        <div
          className="flex flex-col gap-4"
          role="status"
          aria-busy="true"
          aria-label="Loading billing details"
        >
          {[0, 1, 2, 3].map((i) => (
            <Skeleton key={i} className="h-[72px] w-full" />
          ))}
        </div>
      ) : profile.isError ? (
        <ErrorState
          title="Couldn’t load billing details"
          error={profile.error}
          onRetry={() => void profile.refetch()}
        />
      ) : (
        <>
          {profile.data === null && (
            <p className="text-sm text-muted-foreground">
              No billing details yet. Add them so your invoices show the right name and address.
            </p>
          )}
          {!canPay && <NoPayPermission />}
          <ProfileForm
            key={profile.data?.updated_at ?? 'new'}
            workspaceId={workspaceId}
            initial={profile.data}
            readOnly={!canPay}
          />
        </>
      )}
    </Section>
  )
}

function ProfileForm({
  workspaceId,
  initial,
  readOnly,
}: {
  workspaceId: string
  initial: BillingProfile | null
  readOnly: boolean
}) {
  const update = useUpdateProfile(workspaceId)
  const formRef = useRef<HTMLFormElement>(null)
  const form = useForm({
    defaultValues: toValues(initial),
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    validators: { onDynamic: billingProfileSchema },
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: ({ value }) =>
      update
        .mutateAsync(toInput(value))
        .then(() => toast.success('Billing details saved'))
        .catch(() => undefined),
  })
  const details = isApiError(update.error) ? (update.error.details ?? []) : []

  return (
    <form
      ref={formRef}
      noValidate
      className="flex max-w-[560px] flex-col gap-4"
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
      {FIELDS.map((f) => (
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
                  readOnly={readOnly}
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
      {!readOnly && (
        <form.Subscribe selector={(s) => s.isSubmitting}>
          {(submitting) => (
            <Button type="submit" size="lg" className="self-start" disabled={submitting}>
              {submitting ? 'Saving…' : 'Save billing details'}
            </Button>
          )}
        </form.Subscribe>
      )}
    </form>
  )
}
