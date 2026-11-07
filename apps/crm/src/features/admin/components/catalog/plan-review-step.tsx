import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Label } from '#/components/ui/label'
import { Textarea } from '#/components/ui/textarea'
import type { PublishStep, StepState } from '../../api/plan-publish'
import type { Plan, Resource } from '../../types'
import { TextField } from '../kit/text-field'
import { PlanColumnPreview } from './plan-column-preview'
import {
  type Errors,
  limitsChanged,
  type PlanDraft,
  type PlanEditorMode,
  repeatedLines,
  withoutRepeats,
} from './plan-draft'
import { PlanPreviewCard } from './plan-preview-card'
import { previewCatalog } from './plan-preview-model'
import { PlanPublishList } from './plan-publish-list'

export function PlanReviewStep({
  mode,
  draft,
  resources,
  plans,
  errors,
  steps,
  labels,
  progress,
  partial,
  onChange,
}: {
  mode: PlanEditorMode
  draft: PlanDraft
  resources: Resource[]
  /** Every plan in the catalog, so the preview can compare this one with its neighbour. */
  plans: Plan[]
  errors: Errors
  steps: PublishStep[]
  labels: string[]
  progress: Record<string, StepState>
  /** Some calls already went through; the rest can be retried. */
  partial: boolean
  onChange: (patch: Partial<PlanDraft>) => void
}) {
  const free = draft.code === 'free'
  const { model, index } = previewCatalog(draft, mode, plans, resources)
  const repeats = repeatedLines(draft, resources)
  const edit = mode.kind === 'edit' ? mode.plan : undefined
  const changed = limitsChanged(mode, resources, draft)

  return (
    <div className="mx-auto grid w-full max-w-[860px] gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="flex min-w-0 flex-col gap-6">
        {!free && (
          <div className="flex flex-col gap-1.5">
            <Label htmlFor="plan-features">Extra highlights</Label>
            <Textarea
              id="plan-features"
              rows={6}
              value={draft.features}
              onChange={(e) => onChange({ features: e.target.value })}
              aria-invalid={errors.features ? true : undefined}
              aria-describedby="plan-features-note"
              placeholder={'Onboarding call\nDedicated success manager'}
            />
            <p
              id="plan-features-note"
              className={`text-xs ${errors.features ? 'text-destructive' : 'text-muted-foreground'}`}
            >
              {errors.features ??
                "Lines your limits can't say, like 'Onboarding call'. Limits are listed automatically."}
            </p>
            {repeats > 0 && (
              <Alert variant="warning">
                <AlertDescription>
                  <p>
                    {repeats === 1 ? '1 line repeats' : `${repeats} lines repeat`} your limits, so
                    the pricing card leaves {repeats === 1 ? 'it' : 'them'} out. Remove{' '}
                    {repeats === 1 ? 'it' : 'them'}?
                  </p>
                  <Button
                    type="button"
                    size="xs"
                    variant="outline"
                    className="mt-2"
                    onClick={() => onChange({ features: withoutRepeats(draft, resources) })}
                  >
                    Remove repeats
                  </Button>
                </AlertDescription>
              </Alert>
            )}
          </div>
        )}

        {free && (
          <p className="text-sm text-muted-foreground">
            The free plan has no pricing card. It is what a workspace gets with no paid plan.
          </p>
        )}

        {steps.length > 0 && (
          <section className="flex flex-col gap-2">
            <h3 className="text-sm font-extrabold">
              {partial ? 'Finishing up' : 'When you create this'}
            </h3>
            {partial && (
              <Alert variant="info">
                <AlertDescription>
                  <p>
                    The steps ticked below already went through and won't run twice. Retry to finish
                    the rest.
                  </p>
                </AlertDescription>
              </Alert>
            )}
            <PlanPublishList steps={steps} labels={labels} state={progress} />
            {!steps.some((s) => s.kind === 'price') && (
              <p className="text-xs text-muted-foreground">
                No prices yet, so nobody can buy this plan. Add them from the plan card later.
              </p>
            )}
          </section>
        )}

        {edit && (
          <section className="flex flex-col gap-3">
            {changed && (
              <Alert variant="warning">
                <AlertDescription>
                  <p>
                    The limits changed. This reaches{' '}
                    {edit.workspaces === 1
                      ? '1 workspace'
                      : `${edit.workspaces.toLocaleString()} workspaces`}{' '}
                    as soon as you save.
                  </p>
                </AlertDescription>
              </Alert>
            )}
            <TextField
              id="plan-reason"
              label={changed ? 'Why the limits change' : 'Reason (optional)'}
              value={draft.reason}
              onChange={(reason) => onChange({ reason })}
              maxLength={300}
              placeholder="Launch pricing, support fix, …"
              error={errors.reason}
              hint="Kept with the change in the limits audit log."
            />
          </section>
        )}
      </div>

      {!free && (
        <div className="min-w-0 order-first lg:order-last lg:sticky lg:top-0 lg:self-start">
          <p className="mb-2 text-xs font-bold tracking-wide text-muted-foreground uppercase">
            What customers see
          </p>
          <div className="flex flex-col gap-5">
            <PlanPreviewCard draft={draft} summary={model.plans[index]} />
            <PlanColumnPreview matrix={model.matrix} index={index} />
          </div>
        </div>
      )}
    </div>
  )
}
