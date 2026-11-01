import { Badge } from '#/components/ui/badge'
import { Input } from '#/components/ui/input'
import { OptionSelect } from '#/components/ui/option-select'
import type { Resource } from '../../types'
import type { ResourceEntries, ResourceEntry } from './limits'

type Mode = ResourceEntry['mode']

function modes(r: Resource, followsPlan: boolean): { value: Mode; label: string }[] {
  const out: { value: Mode; label: string }[] = followsPlan
    ? [{ value: 'plan', label: "Plan's" }]
    : []
  if (!r.required) out.push({ value: 'off', label: 'Not included' })
  if (r.kind === 'access') return [...out, { value: 'on', label: 'Included' }]
  return [
    ...out,
    { value: 'unlimited', label: 'Unlimited' },
    { value: 'cap', label: 'Limited to…' },
  ]
}

/** What a plan, or one workspace on top of its plan, includes: limits first, then features. */
export function ResourceTable({
  idPrefix,
  resources,
  entries,
  onChange,
  followsPlan = false,
}: {
  idPrefix: string
  resources: Resource[]
  entries: ResourceEntries
  onChange: (entries: ResourceEntries) => void
  /** Overrides: each row can follow the plan. */
  followsPlan?: boolean
}) {
  const groups = [
    { title: 'Limits', rows: resources.filter((r) => r.kind !== 'access') },
    { title: 'Features', rows: resources.filter((r) => r.kind === 'access') },
  ]
  const set = (key: string, e: ResourceEntry) => onChange({ ...entries, [key]: e })

  return (
    <div className="flex flex-col gap-4">
      {groups.map((g) => (
        <fieldset key={g.title} className="flex flex-col gap-2">
          <legend className="mb-2 text-sm font-bold">{g.title}</legend>
          <ul className="flex flex-col divide-y rounded-lg border">
            {g.rows.map((r) => {
              const id = `${idPrefix}-${r.key}`
              const e = entries[r.key] ?? { mode: 'off' }
              return (
                <li
                  key={r.key}
                  className="flex flex-wrap items-center justify-between gap-3 px-3 py-2.5"
                >
                  <div className="flex min-w-0 flex-col gap-0.5">
                    <label htmlFor={id} className="text-sm font-bold">
                      {r.label}
                    </label>
                    <div className="flex flex-wrap items-center gap-1.5 text-xs text-muted-foreground">
                      {r.kind === 'monthly' && <span>Per month</span>}
                      {r.required && <span>Every plan has it</span>}
                      {!r.enforced && <Badge variant="neutral">Not enforced yet</Badge>}
                    </div>
                  </div>
                  <div className="flex items-center gap-2">
                    <OptionSelect
                      id={id}
                      size="sm"
                      value={e.mode}
                      onValueChange={(v) => {
                        const mode = v as Mode
                        set(
                          r.key,
                          mode === 'cap' ? { mode, cap: e.mode === 'cap' ? e.cap : '' } : { mode },
                        )
                      }}
                      options={modes(r, followsPlan)}
                      className="h-10 w-auto min-w-[140px] rounded-md bg-card"
                    />
                    {e.mode === 'cap' && (
                      <Input
                        type="number"
                        min={r.required ? 1 : 0}
                        step={1}
                        required
                        aria-label={`${r.label} limit`}
                        value={e.cap}
                        onChange={(ev) => set(r.key, { mode: 'cap', cap: ev.target.value })}
                        className="h-10 w-28"
                      />
                    )}
                  </div>
                </li>
              )
            })}
          </ul>
        </fieldset>
      ))}
    </div>
  )
}
