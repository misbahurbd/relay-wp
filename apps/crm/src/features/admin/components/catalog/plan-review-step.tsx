import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Label } from '#/components/ui/label'
import { Textarea } from '#/components/ui/textarea'
import type { PublishStep, StepState } from '../../api/plan-publish'
import type { Resource } from '../../types'
import { pricingLines } from '../kit/limits'
import { TextField } from '../kit/text-field'
import {
  draftLimits,
  type Errors,
  featureLines,
  limitsChanged,
  type PlanDraft,
  type PlanEditorMode,
} from './plan-draft'
import { PlanPreviewCard } from './plan-preview-card'
import { PlanPublishList } from './plan-publish-list'

export function PlanReviewStep({
  mode,
  draft,
  resources,
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
  errors: Errors
  steps: PublishStep[]
  labels: string[]
  progress: Record<string, StepState>
  /** Some calls already went through; the rest can be retried. */
  partial: boolean
  onChange: (patch: Partial<PlanDraft>) => void
}) {
  const free = draft.code === 'free'
  const auto = pricingLines(resources, draftLimits(draft))
  const lines = featureLines(draft, resources)
  const edit = mode.kind === 'edit' ? mode.plan : undefined
  const changed = limitsChanged(mode, resources, draft)

  return (
    <div className="mx-auto grid w-full max-w-[860px] gap-6 lg:grid-cols-[minmax(0,1fr)_320px]">
      <div className="flex min-w-0 flex-col gap-6">
        {!free && (
          <div className="flex flex-col gap-1.5">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Label htmlFor="plan-features">Features on the pricing card</Label>
              {draft.featuresEdited && (
                <Button
                  type="button"
                  size="xs"
                  variant="outline"
                  onClick={() => onChange({ featuresEdited: false })}
                >
                  Fill from what's included
                </Button>
              )}
            </div>
            <Textarea
              id="plan-features"
              rows={Math.min(Math.max(lines.length + 1, 6), 12)}
              value={draft.featuresEdited ? draft.features : auto.join('\n')}
              onChange={(e) => onChange({ features: e.target.value, featuresEdited: true })}
              aria-invalid={errors.features ? true : undefined}
              aria-describedby="plan-features-note"
              placeholder={'10,000 contacts\n5 team seats\nEverything in Starter'}
            />
            <p
              id="plan-features-note"
              className={`text-xs ${errors.features ? 'text-destructive' : 'text-muted-foreground'}`}
            >
              {errors.features ??
                (draft.featuresEdited
                  ? 'One per line, up to 20. Your own wording; it no longer follows what is included.'
                  : "Filled from what's included. Change any line to write your own.")}
            </p>
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
          <PlanPreviewCard draft={draft} features={lines} />
        </div>
      )}
    </div>
  )
}
