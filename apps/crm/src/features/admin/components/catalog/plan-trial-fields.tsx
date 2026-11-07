import { Label } from '#/components/ui/label'
import { Switch } from '#/components/ui/switch'
import type { ProviderCaps } from '../../types'
import { TextField } from '../kit/text-field'
import { cardTrialSupport, type Errors, MAX_TRIAL_DAYS, type PlanDraft } from './plan-draft'

/** Trial length, and whether starting it takes a card at checkout. */
export function PlanTrialFields({
  draft,
  errors,
  enabled,
  caps,
  disabled = false,
  onChange,
}: {
  draft: PlanDraft
  errors: Errors
  enabled: string[]
  caps: ProviderCaps[] | undefined
  disabled?: boolean
  onChange: (patch: Partial<PlanDraft>) => void
}) {
  const support = cardTrialSupport(enabled, caps)
  const days = Number(draft.trialDays) || 0
  const cardOff = disabled || (!draft.requiresCard && (!support.allowed || days === 0))

  let note = 'The trial starts through checkout, so the customer enters a card first.'
  if (!support.allowed) {
    const names = support.without.join(' and ')
    note = `${names} can't start a trial at checkout, so ${support.without.length > 1 ? 'they charge' : 'it charges'} the first period straight away. A card trial isn't offered on this server.`
  } else if (days === 0) {
    note = 'Set the trial days first.'
  } else if (support.without.length > 0) {
    note = `Not offered on ${support.without.join(', ')}: customers there pay the first period at checkout.`
  }

  return (
    <div className="flex flex-col gap-4">
      <TextField
        id="plan-trial"
        label="Trial days"
        type="number"
        min={0}
        max={MAX_TRIAL_DAYS}
        value={draft.trialDays}
        onChange={(trialDays) => onChange({ trialDays })}
        disabled={disabled}
        className="w-32"
        error={errors.trialDays}
        hint="0 turns the trial off. TRIAL_PLAN names the plan new workspaces try."
      />
      <div className="flex items-start justify-between gap-4 rounded-lg border bg-card p-4">
        <div className="flex min-w-0 flex-col gap-0.5">
          <Label htmlFor="plan-card">Trial needs a card</Label>
          <p id="plan-card-note" className="text-xs text-muted-foreground">
            {note}
          </p>
        </div>
        <Switch
          id="plan-card"
          checked={draft.requiresCard}
          onCheckedChange={(requiresCard) => onChange({ requiresCard })}
          disabled={cardOff}
          aria-describedby="plan-card-note"
        />
      </div>
    </div>
  )
}
