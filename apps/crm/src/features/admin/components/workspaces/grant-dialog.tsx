import { useState } from 'react'
import { toast } from 'sonner'
import { useGrantPlan } from '../../api/admin.api'
import type { AdminWorkspace, Plan } from '../../types'
import { FormDialog } from '../kit/form-dialog'
import { SelectField } from '../kit/select-field'
import { TextField } from '../kit/text-field'

export function GrantDialog({
  workspace,
  plans,
  onClose,
}: {
  workspace: AdminWorkspace
  plans: Plan[]
  onClose: () => void
}) {
  const grant = useGrantPlan()
  const [planId, setPlanId] = useState(plans[0]?.id ?? '')
  const [endsAt, setEndsAt] = useState('')
  const [reason, setReason] = useState('')

  const submit = () =>
    grant.mutate(
      {
        workspace_id: workspace.id,
        plan_id: planId,
        ends_at: endsAt ? new Date(endsAt).toISOString() : undefined,
        reason: reason.trim(),
      },
      {
        onSuccess: () => {
          toast.success(`${workspace.name} is on the new plan`)
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title={`Change ${workspace.name}'s plan`}
      description="A manual grant: no payment, no invoice. It replaces a trial or an earlier manual grant."
      submitLabel="Grant plan"
      pending={grant.isPending}
      disabled={plans.length === 0}
      error={grant.error}
      onSubmit={submit}
      onClose={onClose}
    >
      {plans.length === 0 ? (
        <p className="text-sm text-muted-foreground">Add an active plan first.</p>
      ) : (
        <SelectField
          id="g-plan"
          label="Plan"
          value={planId}
          onChange={setPlanId}
          options={plans.map((p) => ({
            value: p.id,
            label: `${p.name} (${p.code} v${p.version})`,
          }))}
        />
      )}
      <TextField
        id="g-ends"
        label="Ends (optional)"
        type="datetime-local"
        value={endsAt}
        onChange={setEndsAt}
        hint="Empty: until staff ends it"
      />
      <TextField
        id="g-reason"
        label="Reason"
        value={reason}
        onChange={setReason}
        placeholder="Partner deal, support fix, …"
        hint="Kept with the grant, for the record"
        required
      />
    </FormDialog>
  )
}
