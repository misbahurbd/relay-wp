import { type FormEvent, type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Label } from '#/components/ui/label'
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetFooter,
  SheetHeader,
  SheetTitle,
} from '#/components/ui/sheet'
import { Textarea } from '#/components/ui/textarea'
import { useCreatePlan, useUpdatePlan } from '../../api/admin.api'
import type { Limits, Plan, Resource } from '../../types'
import { ApiErrorAlert } from '../kit/api-error'
import { CheckField } from '../kit/check-field'
import { FREE_PLAN, limitsOf, planEntries, pricingLines } from '../kit/limits'
import { ResourceTable } from '../kit/resource-table'
import { TextField } from '../kit/text-field'

/** Add a plan, publish the next version of one, or edit one version in place. */
export type PlanSheetMode =
  | { kind: 'create' }
  | { kind: 'version'; from: Plan }
  | { kind: 'edit'; plan: Plan }

function sameLimits(resources: Resource[], a: Limits, b: Limits) {
  return resources.every((r) => (a[r.key] ?? 0) === (b[r.key] ?? 0))
}

function Section({
  title,
  description,
  children,
}: {
  title: string
  description?: string
  children: ReactNode
}) {
  return (
    <section className="flex flex-col gap-4 border-t pt-5 first:border-t-0 first:pt-0">
      <div className="flex flex-col gap-1">
        <h3 className="text-base font-extrabold">{title}</h3>
        {description && <p className="text-sm text-muted-foreground">{description}</p>}
      </div>
      {children}
    </section>
  )
}

export function PlanSheet({
  mode,
  resources,
  onClose,
}: {
  mode: PlanSheetMode
  resources: Resource[]
  onClose: () => void
}) {
  const source =
    mode.kind === 'create' ? undefined : mode.kind === 'version' ? mode.from : mode.plan
  const editing = mode.kind === 'edit' ? mode.plan : undefined
  const free = source?.code === FREE_PLAN
  const create = useCreatePlan()
  const update = useUpdatePlan()
  const pending = create.isPending || update.isPending

  const [code, setCode] = useState(source?.code ?? '')
  const [name, setName] = useState(source?.name ?? '')
  const [sortOrder, setSortOrder] = useState(String(source?.sort_order ?? 0))
  const [entries, setEntries] = useState(() => planEntries(resources, source?.limits))
  const [description, setDescription] = useState(source?.description ?? '')
  const [features, setFeatures] = useState((source?.features ?? []).join('\n'))
  const [recommended, setRecommended] = useState(source?.recommended ?? false)
  const [trialDays, setTrialDays] = useState(String(source?.trial_days ?? 0))
  const [requiresCard, setRequiresCard] = useState(source?.trial_requires_card ?? false)
  const [reason, setReason] = useState('')

  const limits = limitsOf(entries, false)
  const limitsChanged = !!editing && !sameLimits(resources, editing.limits, limits)
  const reach =
    editing?.workspaces === 1 ? '1 workspace' : `${editing?.workspaces.toLocaleString()} workspaces`

  const title =
    mode.kind === 'create'
      ? 'Add a plan'
      : mode.kind === 'version'
        ? `New version of ${mode.from.name}`
        : `Edit ${mode.plan.name} v${mode.plan.version}`
  const intro =
    mode.kind === 'create'
      ? 'A code already in use becomes its next version. Add prices after.'
      : mode.kind === 'version'
        ? 'The next version is for new buyers; everyone on this one keeps it. Deactivate the old version once its prices are replaced.'
        : free
          ? 'What every workspace without a paid plan gets.'
          : 'Changes apply to this version in place. Prices never change once sold; publish a new version to sell different terms.'

  const submit = (e: FormEvent) => {
    e.preventDefault()
    const details = {
      name: name.trim(),
      description: description.trim(),
      features: features
        .split('\n')
        .map((f) => f.trim())
        .filter(Boolean),
      recommended,
      trial_days: Number(trialDays) || 0,
      trial_requires_card: requiresCard,
      limits,
      sort_order: Number(sortOrder) || 0,
    }
    const done = (msg: string) => () => {
      toast.success(msg)
      onClose()
    }
    if (editing) {
      update.mutate(
        { planId: editing.id, ...details, reason: reason.trim() },
        { onSuccess: done(`Saved ${editing.code} v${editing.version}`) },
      )
      return
    }
    create.mutate(
      { code: code.trim(), ...details },
      {
        onSuccess: done(
          mode.kind === 'version' ? `Published ${code} v${mode.from.version + 1}` : `Added ${code}`,
        ),
      },
    )
  }

  return (
    <Sheet open onOpenChange={(o) => !o && onClose()}>
      <SheetContent className="w-full gap-0 overflow-y-auto sm:max-w-[720px]">
        <form onSubmit={submit} className="flex min-h-full flex-col">
          <SheetHeader className="px-6 pt-6">
            <SheetTitle className="text-xl font-extrabold">{title}</SheetTitle>
            <SheetDescription>{intro}</SheetDescription>
          </SheetHeader>

          <div className="flex flex-col gap-6 px-6 pb-6">
            <Section title="Plan">
              <div className="grid gap-4 sm:grid-cols-3">
                <TextField
                  id="plan-code"
                  label="Code"
                  value={code}
                  onChange={setCode}
                  disabled={mode.kind !== 'create'}
                  placeholder="growth"
                  hint="Lowercase, digits and _"
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
                  id="plan-sort"
                  label="Sort order"
                  type="number"
                  min={0}
                  value={sortOrder}
                  onChange={setSortOrder}
                  hint="Lower shows first"
                />
              </div>
            </Section>

            <Section
              title="What's included"
              description="A resource left out is not part of the plan. Limits marked “Not enforced yet” only show on the pricing table until their feature ships."
            >
              {editing && (
                <Alert variant={limitsChanged ? 'warning' : 'info'}>
                  <AlertDescription>
                    {free
                      ? `Changes reach ${reach} without a paid plan as soon as you save.`
                      : `Changes reach ${reach} on this version as soon as you save. Lowering a limit never removes anything; it only stops new additions.`}
                  </AlertDescription>
                </Alert>
              )}
              <ResourceTable
                idPrefix="plan-resource"
                resources={resources}
                entries={entries}
                onChange={setEntries}
              />
            </Section>

            {!free && (
              <Section
                title="Pricing table"
                description="What the plan card says. Fill the list from what's included, then edit it."
              >
                <TextField
                  id="plan-description"
                  label="Description"
                  value={description}
                  onChange={setDescription}
                  maxLength={200}
                  placeholder="For growing teams running regular campaigns."
                  hint="One line under the plan name"
                />
                <div className="flex flex-col gap-1.5">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <Label htmlFor="plan-features">Features</Label>
                    <Button
                      type="button"
                      size="xs"
                      variant="outline"
                      onClick={() => setFeatures(pricingLines(resources, limits).join('\n'))}
                    >
                      Fill from what's included
                    </Button>
                  </div>
                  <Textarea
                    id="plan-features"
                    value={features}
                    onChange={(e) => setFeatures(e.target.value)}
                    rows={7}
                    placeholder={'10,000 contacts\n5 team seats\nEverything in Starter'}
                    aria-describedby="plan-features-hint"
                  />
                  <p id="plan-features-hint" className="text-xs text-muted-foreground">
                    One per line, up to 20, each with a tick on the pricing table.
                  </p>
                </div>
                <CheckField
                  id="plan-recommended"
                  label="Recommended plan (gets the Recommended badge)"
                  checked={recommended}
                  onChange={setRecommended}
                />
              </Section>
            )}

            {!free && (
              <Section title="Trial">
                <div className="grid gap-4 sm:grid-cols-2">
                  <TextField
                    id="plan-trial"
                    label="Trial days"
                    type="number"
                    min={0}
                    max={90}
                    value={trialDays}
                    onChange={setTrialDays}
                    hint="0 turns the trial off; TRIAL_PLAN names the plan new workspaces try"
                  />
                </div>
                <CheckField
                  id="plan-card"
                  label="The trial needs a card (it starts through checkout)"
                  checked={requiresCard}
                  onChange={setRequiresCard}
                />
              </Section>
            )}

            {editing && (
              <Section title="Reason">
                <TextField
                  id="plan-reason"
                  label={limitsChanged ? 'Why the limits change' : 'Reason (optional)'}
                  value={reason}
                  onChange={setReason}
                  maxLength={300}
                  placeholder="Launch pricing, support fix, …"
                  hint="Kept with the change in the limits audit log"
                  required={limitsChanged}
                />
              </Section>
            )}
          </div>

          <SheetFooter className="sticky bottom-0 gap-3 border-t bg-background px-6 py-4">
            <ApiErrorAlert error={editing ? update.error : create.error} />
            <div className="flex justify-end gap-2">
              <Button type="button" variant="outline" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" disabled={pending}>
                {pending
                  ? 'Saving…'
                  : mode.kind === 'create'
                    ? 'Add plan'
                    : mode.kind === 'version'
                      ? 'Publish version'
                      : 'Save changes'}
              </Button>
            </div>
          </SheetFooter>
        </form>
      </SheetContent>
    </Sheet>
  )
}
