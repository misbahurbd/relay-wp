import { useState } from 'react'
import { toast } from 'sonner'
import { useCreatePlan } from '../../api/admin.api'
import type { Plan } from '../../types'
import { CheckField } from '../kit/check-field'
import { FormDialog } from '../kit/form-dialog'
import { TextField } from '../kit/text-field'

/** Adds a plan. Given `from`, it publishes that plan's next version, prefilled with its settings. */
export function PlanDialog({ from, onClose }: { from?: Plan; onClose: () => void }) {
  const create = useCreatePlan()
  const [code, setCode] = useState(from?.code ?? '')
  const [name, setName] = useState(from?.name ?? '')
  const [trialDays, setTrialDays] = useState(String(from?.trial_days ?? 0))
  const [requiresCard, setRequiresCard] = useState(from?.trial_requires_card ?? false)
  const [members, setMembers] = useState(String(from?.limits.members ?? 5))
  const [numbers, setNumbers] = useState(String(from?.limits.whatsapp_numbers ?? 1))
  const [sortOrder, setSortOrder] = useState(String(from?.sort_order ?? 0))

  const submit = () =>
    create.mutate(
      {
        code: code.trim(),
        name: name.trim(),
        trial_days: Number(trialDays) || 0,
        trial_requires_card: requiresCard,
        limits: { members: Number(members), whatsapp_numbers: Number(numbers) },
        sort_order: Number(sortOrder) || 0,
      },
      {
        onSuccess: () => {
          toast.success(from ? `Published ${code} v${from.version + 1}` : `Added ${code}`)
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title={from ? `New version of ${from.name}` : 'Add a plan'}
      description={
        from
          ? 'Plans are never edited once sold. This adds the next version; deactivate the old one when its prices are replaced.'
          : 'A code already in use becomes its next version. Add prices after.'
      }
      submitLabel={from ? 'Publish version' : 'Add plan'}
      pending={create.isPending}
      error={create.error}
      onSubmit={submit}
      onClose={onClose}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField
          id="plan-code"
          label="Code"
          value={code}
          onChange={setCode}
          disabled={!!from}
          placeholder="growth"
          hint="Lowercase, digits and _. TRIAL_PLAN names one."
          required
        />
        <TextField
          id="plan-name"
          label="Name"
          value={name}
          onChange={setName}
          placeholder="Growth"
          required
        />
        <TextField
          id="plan-members"
          label="Team members"
          type="number"
          min={-1}
          value={members}
          onChange={setMembers}
          hint="-1 is unlimited"
        />
        <TextField
          id="plan-numbers"
          label="WhatsApp numbers"
          type="number"
          min={-1}
          value={numbers}
          onChange={setNumbers}
          hint="-1 is unlimited"
        />
        <TextField
          id="plan-trial"
          label="Trial days"
          type="number"
          min={0}
          max={90}
          value={trialDays}
          onChange={setTrialDays}
          hint="0 turns the trial off"
        />
        <TextField
          id="plan-sort"
          label="Sort order"
          type="number"
          min={0}
          value={sortOrder}
          onChange={setSortOrder}
          hint="Lower shows first on the pricing page"
        />
      </div>
      <CheckField
        id="plan-card"
        label="The trial needs a card (it starts through checkout)"
        checked={requiresCard}
        onChange={setRequiresCard}
      />
    </FormDialog>
  )
}
