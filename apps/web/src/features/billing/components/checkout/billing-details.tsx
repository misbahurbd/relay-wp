import { Button } from '#/components/ui/button'
import type { BillingProfile } from '../../types'
import { Section, SectionHeader } from '../kit/section'
import { ProfileFields, type ProfileFormState } from '../settings/profile-form'

function countryName(code: string) {
  try {
    return new Intl.DisplayNames(undefined, { type: 'region' }).of(code.toUpperCase()) ?? code
  } catch {
    return code
  }
}

/**
 * Who the invoice is for, on the checkout itself. A saved profile shows as three lines with an
 * Edit link; with none, the form is open. The page's "Continue to payment" saves it, so the
 * form has no button of its own.
 */
export function BillingDetails({
  profile,
  form,
  editing,
  locked,
  onEdit,
  onCancel,
}: {
  profile: BillingProfile | null
  form: ProfileFormState
  editing: boolean
  /** Payment has started: the details are what the checkout was made with. */
  locked: boolean
  onEdit: () => void
  onCancel: () => void
}) {
  const open = !locked && (editing || profile === null)
  return (
    <Section aria-labelledby="billing-heading">
      <SectionHeader
        id="billing-heading"
        title="Billing details"
        description={
          open ? 'Who the invoice is made out to. Only name and email are needed.' : undefined
        }
        action={
          profile && !locked && !open ? (
            <Button variant="link" size="sm" onClick={onEdit}>
              Edit
            </Button>
          ) : undefined
        }
      />
      {open ? (
        <ProfileFields profile={form} collapsible>
          {profile && (
            <Button type="button" variant="outline" className="self-start" onClick={onCancel}>
              Cancel
            </Button>
          )}
        </ProfileFields>
      ) : (
        profile && (
          <div className="flex flex-col gap-0.5 text-sm">
            <p className="font-bold">{profile.legal_name}</p>
            <p className="text-muted-foreground">{profile.email}</p>
            {profile.country && (
              <p className="text-muted-foreground">{countryName(profile.country)}</p>
            )}
          </div>
        )
      )}
    </Section>
  )
}
