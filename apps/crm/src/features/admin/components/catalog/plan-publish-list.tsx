import { CheckIcon, CircleIcon, Loader2Icon, XIcon } from 'lucide-react'
import type { PublishStep, StepState } from '../../api/plan-publish'
import { ApiErrorAlert } from '../kit/api-error'

/** Each call a create makes, with how far it got. Doubles as the list of what Create will do. */
export function PlanPublishList({
  steps,
  labels,
  state,
}: {
  steps: PublishStep[]
  labels: string[]
  state: Record<string, StepState>
}) {
  return (
    <ol className="flex flex-col divide-y overflow-hidden rounded-lg border bg-card">
      {steps.map((step, i) => {
        const s = state[step.id]?.status ?? 'pending'
        return (
          <li
            key={step.id}
            ref={s === 'failed' ? (el) => el?.scrollIntoView({ block: 'nearest' }) : undefined}
            className="flex flex-col gap-2 px-3.5 py-2.5"
          >
            <div className="flex items-center gap-2.5 text-sm">
              <span className="grid size-5 shrink-0 place-content-center" aria-hidden="true">
                {s === 'done' && <CheckIcon className="size-4 text-brand" />}
                {s === 'running' && (
                  <Loader2Icon className="size-4 animate-spin text-muted-foreground" />
                )}
                {s === 'failed' && <XIcon className="size-4 text-destructive" />}
                {s === 'pending' && <CircleIcon className="size-3.5 text-muted-foreground" />}
              </span>
              <span className={s === 'pending' ? 'text-muted-foreground' : 'font-semibold'}>
                {labels[i]}
              </span>
              <span className="sr-only">
                {s === 'done'
                  ? '(done)'
                  : s === 'running'
                    ? '(in progress)'
                    : s === 'failed'
                      ? '(failed)'
                      : '(waiting)'}
              </span>
            </div>
            {s === 'failed' && <ApiErrorAlert error={state[step.id]?.error} />}
          </li>
        )
      })}
    </ol>
  )
}
