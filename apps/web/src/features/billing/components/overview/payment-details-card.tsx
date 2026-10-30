import { useQuery } from '@tanstack/react-query'
import { CreditCardIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { useOpenCardPortal } from '../../api/billing.mutations'
import { paymentMethodQueryOptions, profileQueryOptions } from '../../api/billing.queries'
import { PROVIDER_LABEL } from '../../schemas/billing.schema'
import type { BillingProfile, PaymentMethod } from '../../types'
import { cardBrand, cardExpiry } from '../kit/card'
import { Modal } from '../kit/modal'
import { Section, SectionHeader } from '../kit/section'
import { ErrorState } from '../kit/states'
import { ProfileForm } from '../settings/profile-form'

/** The card the provider charges (read from the provider, never stored) and who invoices go to. */
export function PaymentDetailsCard({
  workspaceId,
  canPay,
  className,
}: {
  workspaceId: string
  canPay: boolean
  className?: string
}) {
  const method = useQuery(paymentMethodQueryOptions(workspaceId))
  const profile = useQuery(profileQueryOptions(workspaceId))
  const portal = useOpenCardPortal(workspaceId)
  const [editing, setEditing] = useState(false)
  const canManage = canPay && !!method.data?.can_manage
  const openPortal = () => portal.mutate()

  return (
    <Section aria-labelledby="payment-heading" className={className}>
      <SectionHeader
        id="payment-heading"
        title="Payment method"
        action={
          canManage && (
            <Button variant="outline" size="sm" disabled={portal.isPending} onClick={openPortal}>
              {portal.isPending ? 'Opening…' : 'Update'}
            </Button>
          )
        }
      />

      {method.isPending ? (
        <Skeleton className="h-[104px] rounded-lg" aria-label="Loading your card" />
      ) : method.isError ? (
        <ErrorState
          title="Couldn’t load your card"
          error={method.error}
          onRetry={() => void method.refetch()}
        />
      ) : (
        <CardOnFile method={method.data} holder={profile.data?.legal_name} />
      )}

      {portal.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(portal.error)}</AlertDescription>
        </Alert>
      )}

      <dl className="flex flex-col gap-1 text-[13px]">
        {profile.isPending ? (
          <Skeleton className="h-24" aria-label="Loading billing details" />
        ) : profile.isError ? (
          <ErrorState
            title="Couldn’t load billing details"
            error={profile.error}
            onRetry={() => void profile.refetch()}
          />
        ) : (
          <ProfileSummary profile={profile.data} />
        )}
      </dl>

      <div className="flex flex-wrap gap-x-5 gap-y-2 text-[13px] font-bold">
        {canManage && (
          <button
            type="button"
            className="text-brand-text hover:underline disabled:opacity-50"
            disabled={portal.isPending}
            onClick={openPortal}
          >
            + Add backup payment method
          </button>
        )}
        {canPay && profile.isSuccess && (
          <button
            type="button"
            className="text-brand-text hover:underline"
            onClick={() => setEditing(true)}
          >
            {profile.data ? 'Edit billing details' : 'Add billing details'}
          </button>
        )}
      </div>

      {editing && profile.isSuccess && (
        <Modal
          onClose={() => setEditing(false)}
          title="Billing details"
          description="Who invoices are made out to. Past invoices keep the details they were issued with."
        >
          <ProfileForm
            workspaceId={workspaceId}
            initial={profile.data}
            onSaved={() => setEditing(false)}
          />
        </Modal>
      )}
    </Section>
  )
}

function CardOnFile({ method, holder }: { method: PaymentMethod; holder?: string }) {
  const { card, provider } = method
  if (!card) {
    return (
      <p className="flex items-start gap-2 rounded-xl border border-dashed p-4 text-[13px] text-muted-foreground">
        <CreditCardIcon className="mt-0.5 size-4 flex-none" aria-hidden="true" />
        {!provider
          ? 'No card on file yet. It’s saved when you first pay for a plan.'
          : !method.card_details
            ? `Your card is kept by ${PROVIDER_LABEL[provider]}, which doesn’t share its details. Use Update to see or change it.`
            : `We couldn’t read your card from ${PROVIDER_LABEL[provider]} right now. Try again later.`}
      </p>
    )
  }
  return (
    <div className="flex min-h-[150px] flex-col justify-between gap-[22px] rounded-[14px] bg-card-face p-[18px] text-card-face-foreground">
      <div className="flex items-center justify-between">
        <span className="h-[26px] w-9 rounded-[5px] bg-card-chip opacity-85" aria-hidden="true" />
        <span className="text-xs font-extrabold tracking-[0.08em] uppercase">
          {cardBrand(card.brand)}
        </span>
      </div>
      <p className="text-lg font-bold tracking-[0.12em] tabular-nums">
        <span aria-hidden="true">•••• •••• •••• </span>
        <span className="sr-only">Card ending </span>
        {card.last4}
      </p>
      <div className="flex justify-between gap-3 text-xs">
        <span className="truncate">{holder}</span>
        <span className="flex-none">Expires {cardExpiry(card)}</span>
      </div>
    </div>
  )
}

function ProfileSummary({ profile }: { profile: BillingProfile | null }) {
  if (!profile) {
    return (
      <p className="text-muted-foreground">
        No billing details yet. Add them so invoices show the right name and address.
      </p>
    )
  }
  const address = [profile.city, profile.state, profile.country].filter(Boolean).join(', ')
  return (
    <>
      <Row label="Billing email">{profile.email}</Row>
      <Row label="Billing address">{address || 'Not added'}</Row>
      <Row label="Tax ID">
        {profile.tax_id
          ? `${profile.tax_id}${profile.tax_id_type ? ` (${profile.tax_id_type})` : ''}`
          : 'Not added'}
      </Row>
    </>
  )
}

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex flex-wrap justify-between gap-x-3 gap-y-0.5 border-b py-2 last:border-b-0">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="min-w-0 text-right font-semibold break-words">{children}</dd>
    </div>
  )
}
