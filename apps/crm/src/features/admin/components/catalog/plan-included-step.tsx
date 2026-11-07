import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { TooltipProvider } from '#/components/ui/tooltip'
import type { Plan, Resource } from '../../types'
import { includeEverything, isIncluded, type ResourceEntries } from '../kit/limits'
import { type Errors, includedCount } from './plan-draft'
import { PlanResourceRow } from './plan-resource-row'

/** Limits first, then features, as the catalog lists them. */
const GROUPS = [
  { title: 'Limits', hint: 'How much of it the plan allows', access: false },
  { title: 'Features', hint: 'Whether the plan can use it at all', access: true },
]

export function PlanIncludedStep({
  resources,
  entries,
  saved,
  errors,
  impact,
  onChange,
}: {
  resources: Resource[]
  entries: ResourceEntries
  /** What Reset goes back to: the saved plan, or the starting point of a new one. */
  saved: ResourceEntries
  errors: Errors
  /** Editing a live version: who a change reaches. */
  impact?: { plan: Plan; changed: boolean }
  onChange: (entries: ResourceEntries) => void
}) {
  const total = resources.length
  const count = includedCount(resources, entries)
  const reach =
    impact?.plan.workspaces === 1
      ? '1 workspace'
      : `${impact?.plan.workspaces.toLocaleString()} workspaces`

  return (
    <TooltipProvider delayDuration={150}>
      <div className="mx-auto flex w-full max-w-[760px] flex-col gap-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div className="flex flex-col gap-0.5">
            <p className="text-sm font-extrabold" aria-live="polite">
              {count} of {total} included
            </p>
            <p className="text-xs text-muted-foreground">
              A plan only has what you switch on. Anything left out is not part of it.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <Button
              type="button"
              size="sm"
              variant="outline"
              onClick={() => onChange(includeEverything(resources))}
            >
              Include everything (unlimited)
            </Button>
            <Button type="button" size="sm" variant="ghost" onClick={() => onChange(saved)}>
              Reset
            </Button>
          </div>
        </div>

        {impact && (
          <Alert variant={impact.changed ? 'warning' : 'info'}>
            <AlertDescription>
              <p>
                {impact.plan.code === 'free'
                  ? `Changes reach ${reach} without a paid plan as soon as you save.`
                  : `Changes reach ${reach} on this version as soon as you save. Lowering a limit never removes anything; it only stops new additions.`}
              </p>
            </AlertDescription>
          </Alert>
        )}

        {GROUPS.map((g) => {
          const rows = resources.filter((r) => (r.kind === 'access') === g.access)
          if (rows.length === 0) return null
          const here = rows.filter((r) => isIncluded(entries[r.key])).length
          return (
            <section key={g.title} className="flex flex-col gap-2">
              <div className="flex flex-wrap items-baseline justify-between gap-x-3">
                <h3 className="text-sm font-extrabold">{g.title}</h3>
                <p className="text-xs text-muted-foreground">
                  {g.hint} · {here} of {rows.length} included
                </p>
              </div>
              <ul className="flex flex-col divide-y overflow-hidden rounded-lg border bg-card">
                {rows.map((r) => (
                  <PlanResourceRow
                    key={r.key}
                    resource={r}
                    entry={entries[r.key] ?? { mode: 'off' }}
                    error={errors[`cap:${r.key}`]}
                    onChange={(e) => onChange({ ...entries, [r.key]: e })}
                  />
                ))}
              </ul>
            </section>
          )
        })}
      </div>
    </TooltipProvider>
  )
}
