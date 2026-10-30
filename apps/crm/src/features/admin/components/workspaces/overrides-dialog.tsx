import { useState } from 'react'
import { toast } from 'sonner'
import { useSetGrantOverrides } from '../../api/admin.api'
import type { AdminWorkspace, Resource } from '../../types'
import { FormDialog } from '../kit/form-dialog'
import { limitsOf, overrideEntries } from '../kit/limits'
import { ResourceTable } from '../kit/resource-table'
import { TextField } from '../kit/text-field'

/** Sets what one workspace gets on top of its plan. Its own limits end when its plan changes. */
export function OverridesDialog({
  workspace,
  resources,
  onClose,
}: {
  workspace: AdminWorkspace
  resources: Resource[]
  onClose: () => void
}) {
  const set = useSetGrantOverrides()
  const [entries, setEntries] = useState(() =>
    overrideEntries(resources, workspace.grant?.overrides ?? {}),
  )
  const [reason, setReason] = useState('')

  const submit = () =>
    set.mutate(
      { workspace_id: workspace.id, overrides: limitsOf(entries, true), reason: reason.trim() },
      {
        onSuccess: () => {
          toast.success(`Updated ${workspace.name}'s limits`)
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title={`${workspace.name}'s own limits`}
      description={`On top of ${workspace.grant?.plan_name ?? 'its plan'}, for this workspace only. “Plan's” follows the plan, including later edits to it. Its own limits end when the workspace changes plan.`}
      submitLabel="Save limits"
      wide
      pending={set.isPending}
      error={set.error}
      onSubmit={submit}
      onClose={onClose}
    >
      <ResourceTable
        idPrefix="override"
        resources={resources}
        entries={entries}
        onChange={setEntries}
        followsPlan
      />
      <TextField
        id="override-reason"
        label="Reason"
        value={reason}
        onChange={setReason}
        placeholder="Bigger team for the pilot, …"
        hint="Kept with the change in the limits audit log"
        required
      />
    </FormDialog>
  )
}
