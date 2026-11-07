import { useQuery } from '@tanstack/react-query'
import { Loader2Icon } from 'lucide-react'
import { type FormEvent, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from '#/components/ui/dialog'
import {
  planPublishApi,
  providerCapsQueryOptions,
  useRefreshAdmin,
  useUpdatePlan,
} from '../../api/admin.api'
import { type CreatedIds, runPublish, type StepState } from '../../api/plan-publish'
import type { Plan, Resource } from '../../types'
import { ApiErrorAlert } from '../kit/api-error'
import { FREE_PLAN, planEntries } from '../kit/limits'
import { formatPrice } from '../kit/money'
import { Stepper } from '../kit/stepper'
import { PlanBasicsStep } from './plan-basics-step'
import {
  CURRENCY,
  type Errors,
  initialDraft,
  limitsChanged,
  type PlanDraft,
  type PlanEditorMode,
  planBody,
  publishSteps,
  type StepId,
  sellProviders,
  sourceOf,
  stepErrors,
  stepLabel,
  stepsOf,
} from './plan-draft'
import { PlanIncludedStep } from './plan-included-step'
import { PlanPricingStep } from './plan-pricing-step'
import { PlanReviewStep } from './plan-review-step'
import { PlanTrialFields } from './plan-trial-fields'

const STEP_LABEL: Record<StepId, string> = {
  basics: 'Basics',
  included: "What's included",
  pricing: 'Pricing & trial',
  trial: 'Trial',
  review: 'Review',
}

function titleOf(mode: PlanEditorMode) {
  if (mode.kind === 'create') return 'Add a plan'
  if (mode.kind === 'version') return `New version of ${mode.from.name}`
  return `Edit ${mode.plan.name} v${mode.plan.version}`
}

function introOf(mode: PlanEditorMode, free: boolean) {
  if (mode.kind === 'create')
    return 'Name it, choose what it includes, set prices, then check how it looks.'
  if (mode.kind === 'version') {
    return 'The next version is for new buyers; everyone on this one keeps it. Deactivate the old version once its prices are replaced.'
  }
  return free
    ? 'What every workspace without a paid plan gets.'
    : 'Changes apply to this version in place. Prices never change once sold; publish a new version to sell different terms.'
}

/**
 * Add a plan, publish the next version of one, or edit one version in place. A new plan or version
 * goes through the steps and then creates the plan, its prices and their provider listings; an
 * edit lets staff open any step and save from it.
 */
export function PlanEditor({
  mode,
  resources,
  plans,
  providers,
  onClose,
}: {
  mode: PlanEditorMode
  resources: Resource[]
  plans: Plan[]
  /** Payment providers enabled on this server. */
  providers: string[]
  onClose: () => void
}) {
  const source = sourceOf(mode)
  const free = source?.code === FREE_PLAN
  const editing = mode.kind === 'edit' ? mode.plan : undefined
  const sellWith = sellProviders(providers)
  const caps = useQuery(providerCapsQueryOptions()).data
  const update = useUpdatePlan()
  const refresh = useRefreshAdmin()

  const [draft, setDraft] = useState(() => initialDraft(mode, resources, sellWith.length > 0))
  const [step, setStep] = useState<StepId>('basics')
  const [visited, setVisited] = useState<StepId[]>(['basics'])
  const [shown, setShown] = useState<StepId[]>([])
  const [progress, setProgress] = useState<Record<string, StepState>>({})
  const [ids, setIds] = useState<CreatedIds>({})
  const [running, setRunning] = useState(false)
  const [failure, setFailure] = useState<{ label: string } | null>(null)
  const body = useRef<HTMLDivElement>(null)
  const focusInvalid = useRef(false)

  const order = stepsOf(mode)
  const index = order.indexOf(step)
  const last = index === order.length - 1
  const ctx = { mode, resources }
  const errorsOf = (s: StepId) => stepErrors(s, draft, ctx)
  const valid = (s: StepId) => Object.keys(errorsOf(s)).length === 0
  const errors: Errors = shown.includes(step) ? errorsOf(step) : {}
  const pending = running || update.isPending

  const planCreated = !!ids.plan
  const steps = editing ? [] : publishSteps(draft, resources, draft.sell ? sellWith : [])
  const labels = steps.map((s) => stepLabel(s, draft, (m) => formatPrice(m, CURRENCY)))
  const doneCount = steps.filter((s) => ids[s.id]).length
  const createdPrices = (['month', 'year', 'lifetime'] as const).filter((c) => ids[`price:${c}`])

  const patch = (p: Partial<PlanDraft>) => setDraft((d) => ({ ...d, ...p }))

  useEffect(() => {
    if (!focusInvalid.current) return
    focusInvalid.current = false
    body.current?.querySelector<HTMLElement>('[aria-invalid="true"]')?.focus()
  })

  const goTo = (s: StepId) => {
    body.current?.scrollTo({ top: 0 })
    setStep(s)
    setVisited((v) => (v.includes(s) ? v : [...v, s]))
  }

  const stop = (at: StepId) => {
    setShown((s) => (s.includes(at) ? s : [...s, at]))
    focusInvalid.current = true
    goTo(at)
  }

  const firstInvalid = (upTo: number) => order.slice(0, upTo + 1).find((s) => !valid(s))

  const reachable = (i: number) => !!editing || order.slice(0, i).every(valid)

  const next = () => {
    const bad = firstInvalid(index)
    if (bad) return stop(bad)
    goTo(order[index + 1])
  }

  const save = () => {
    if (!editing) return
    const bad = firstInvalid(order.length - 1)
    if (bad) return stop(bad)
    update.mutate(
      {
        planId: editing.id,
        ...planBody(draft, resources),
        reason: draft.reason.trim(),
      },
      {
        onSuccess: () => {
          toast.success(`Saved ${editing.code} v${editing.version}`)
          onClose()
        },
      },
    )
  }

  const publish = async () => {
    const bad = firstInvalid(order.length - 1)
    if (bad) return stop(bad)
    setFailure(null)
    setRunning(true)
    const result = await runPublish(steps, ids, planPublishApi, (id, state) =>
      setProgress((p) => ({ ...p, [id]: state })),
    )
    setIds(result.ids)
    setRunning(false)
    void refresh()
    if (!result.ok) {
      const at = steps.findIndex((s) => s.id === result.failed)
      setFailure({ label: labels[at] })
      return
    }
    const prices = steps.filter((s) => s.kind === 'price').length
    toast.success(
      mode.kind === 'version'
        ? `Published ${draft.code} v${mode.from.version + 1}`
        : `Added ${draft.code}${prices ? ` with ${prices} price${prices === 1 ? '' : 's'}` : ''}`,
    )
    onClose()
  }

  const submit = (e: FormEvent) => {
    e.preventDefault()
    if (pending) return
    if (editing) save()
    else if (last) void publish()
    else next()
  }

  const items = order.map((s, i) => ({
    id: s,
    label: STEP_LABEL[s],
    done: !editing && s !== step && visited.includes(s) && valid(s),
    reachable: reachable(i),
  }))

  const lockedNote = planCreated && (step === 'basics' || step === 'included')
  const submitLabel = editing
    ? 'Save changes'
    : failure
      ? 'Retry the rest'
      : mode.kind === 'version'
        ? 'Publish version'
        : 'Create plan'

  return (
    <Dialog open onOpenChange={(open) => !open && !pending && onClose()}>
      <DialogContent
        onInteractOutside={(e) => e.preventDefault()}
        className="top-0 left-0 flex h-dvh max-h-dvh max-w-none translate-x-0 translate-y-0 flex-col gap-0 overflow-hidden rounded-none border-0 p-0 sm:top-[50%] sm:left-[50%] sm:h-[min(780px,calc(100dvh-48px))] sm:max-w-[min(940px,calc(100%-3rem))] sm:translate-x-[-50%] sm:translate-y-[-50%] sm:rounded-xl sm:border"
      >
        <form onSubmit={submit} noValidate className="flex min-h-0 flex-1 flex-col">
          <DialogHeader className="gap-3 border-b px-4 pt-5 pb-3 sm:px-6">
            <div className="flex flex-col gap-1 pr-8">
              <DialogTitle className="text-xl font-extrabold">{titleOf(mode)}</DialogTitle>
              <DialogDescription className="max-w-[70ch]">{introOf(mode, free)}</DialogDescription>
            </div>
            <Stepper
              label="Plan steps"
              steps={items}
              current={step}
              onSelect={(s) => goTo(s as StepId)}
            />
          </DialogHeader>

          <div
            ref={body}
            className="min-h-0 flex-1 overflow-x-hidden overflow-y-auto px-4 py-5 sm:px-6"
          >
            {lockedNote && (
              <Alert variant="info" className="mx-auto mb-5 max-w-[640px]">
                <AlertDescription>
                  <p>The plan is already created, so these can no longer change here.</p>
                </AlertDescription>
              </Alert>
            )}
            <fieldset disabled={planCreated} className="m-0 min-w-0 border-0 p-0">
              {step === 'basics' && (
                <PlanBasicsStep
                  mode={mode}
                  draft={draft}
                  errors={errors}
                  plans={plans}
                  onChange={patch}
                />
              )}
              {step === 'included' && (
                <PlanIncludedStep
                  resources={resources}
                  entries={draft.entries}
                  saved={planEntries(resources, source?.limits)}
                  errors={errors}
                  impact={
                    editing
                      ? { plan: editing, changed: limitsChanged(mode, resources, draft) }
                      : undefined
                  }
                  onChange={(entries) => patch({ entries })}
                />
              )}
            </fieldset>
            {step === 'pricing' && (
              <PlanPricingStep
                draft={draft}
                errors={errors}
                enabled={providers}
                sellWith={sellWith}
                caps={caps}
                created={createdPrices}
                trialLocked={planCreated}
                onChange={patch}
              />
            )}
            {step === 'trial' && (
              <div className="mx-auto w-full max-w-[560px]">
                <PlanTrialFields
                  draft={draft}
                  errors={errors}
                  enabled={providers}
                  caps={caps}
                  onChange={patch}
                />
              </div>
            )}
            {step === 'review' && (
              <PlanReviewStep
                mode={mode}
                draft={draft}
                resources={resources}
                errors={errors}
                steps={steps}
                labels={labels}
                progress={progress}
                partial={planCreated}
                onChange={patch}
              />
            )}
          </div>

          <footer className="flex flex-col gap-3 border-t bg-card px-4 py-3 sm:px-6">
            {editing && <ApiErrorAlert error={update.error} />}
            <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-3">
              <p className="min-w-0 text-[13px]" aria-live="polite">
                {running ? (
                  <span className="flex items-center gap-2 text-muted-foreground">
                    <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
                    {labels[steps.findIndex((s) => progress[s.id]?.status === 'running')] ??
                      'Working'}
                    …
                  </span>
                ) : failure ? (
                  <span className="text-destructive">
                    Stopped at: {failure.label}. {doneCount} of {steps.length} steps done.
                  </span>
                ) : (
                  <span className="text-muted-foreground">
                    Step {index + 1} of {order.length}
                  </span>
                )}
              </p>
              <div className="ml-auto flex flex-wrap justify-end gap-2">
                <Button type="button" variant="outline" disabled={pending} onClick={onClose}>
                  {planCreated ? 'Close' : 'Cancel'}
                </Button>
                {!editing && index > 0 && (
                  <Button
                    type="button"
                    variant="outline"
                    disabled={pending}
                    onClick={() => goTo(order[index - 1])}
                  >
                    Back
                  </Button>
                )}
                {editing || last ? (
                  <Button type="submit" disabled={pending}>
                    {pending ? 'Saving…' : submitLabel}
                  </Button>
                ) : (
                  <Button type="submit">Next</Button>
                )}
              </div>
            </div>
          </footer>
        </form>
      </DialogContent>
    </Dialog>
  )
}
