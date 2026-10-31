import { PlusIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { cn } from '#/lib/utils'
import { blankCondition, plainWords, presets } from '../../schemas/segments.schema'
import type { SegmentGroup, SegmentRules, Tag } from '../../types'
import { ConditionRow } from './condition-row'

export type SegmentDraft = { name: string; description: string; rules: SegmentRules }

/** Name, quick starts, groups of conditions and the rules in plain words. */
export function SegmentBuilder({
  title,
  draft,
  tags,
  readOnly,
  dirty,
  errors,
  actions,
  onChange,
}: {
  title: string
  draft: SegmentDraft
  tags: Tag[]
  readOnly: boolean
  dirty: boolean
  errors: Record<string, string>
  actions: ReactNode
  onChange: (d: SegmentDraft) => void
}) {
  const groups = draft.rules.groups
  const setGroups = (g: SegmentGroup[]) => onChange({ ...draft, rules: { groups: g } })
  const setGroup = (i: number, g: SegmentGroup) =>
    setGroups(groups.map((x, n) => (n === i ? g : x)))
  const words = plainWords(draft.rules, tags)

  return (
    <section
      aria-label="Segment builder"
      className="flex min-w-0 flex-col gap-5 rounded-2xl border bg-card p-5 shadow-card sm:p-6"
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="text-[19px] font-extrabold">{title}</h2>
        {dirty && (
          <span className="rounded-full bg-warning-soft px-2.5 py-0.5 text-xs font-bold text-warning">
            Unsaved changes
          </span>
        )}
      </div>
      <div className="grid gap-3 sm:grid-cols-2">
        <Label className="flex flex-col items-stretch gap-1.5">
          Segment name
          <Input
            value={draft.name}
            maxLength={80}
            disabled={readOnly}
            onChange={(e) => onChange({ ...draft, name: e.target.value })}
          />
          {errors.name && <span className="text-xs text-destructive">{errors.name}</span>}
        </Label>
        <Label className="flex flex-col items-stretch gap-1.5">
          <span>
            Description{' '}
            <span className="font-normal text-muted-foreground">(optional, for your team)</span>
          </span>
          <Input
            value={draft.description}
            maxLength={200}
            disabled={readOnly}
            placeholder="Who is this for?"
            onChange={(e) => onChange({ ...draft, description: e.target.value })}
          />
        </Label>
      </div>

      {!readOnly && (
        <div className="flex flex-wrap items-center gap-2">
          <span className="text-[13px] font-bold text-muted-foreground">Quick start:</span>
          {presets(tags).map((p) => (
            <Button
              key={p.name}
              variant="outline"
              size="sm"
              className="rounded-full"
              onClick={() => onChange({ name: p.name, description: p.description, rules: p.rules })}
            >
              {p.name}
            </Button>
          ))}
        </div>
      )}

      <div className="flex flex-col gap-3">
        {groups.map((g, gi) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: groups are positional
          <div key={gi} className="flex flex-col gap-3">
            {gi > 0 && (
              <div className="flex items-center gap-3" aria-hidden="true">
                <span className="h-px flex-1 bg-border" />
                <span className="text-xs font-extrabold text-muted-foreground">OR</span>
                <span className="h-px flex-1 bg-border" />
              </div>
            )}
            <fieldset
              aria-label={`Group ${gi + 1}`}
              className="flex flex-col gap-2.5 rounded-[14px] border bg-muted/40 p-4"
            >
              <div className="flex flex-wrap items-center gap-2 text-sm font-semibold">
                Match
                <span className="inline-flex rounded-lg border bg-card p-0.5">
                  {(['all', 'any'] as const).map((m) => (
                    <Button
                      key={m}
                      variant="unstyled"
                      type="button"
                      aria-pressed={g.match === m}
                      disabled={readOnly}
                      onClick={() => setGroup(gi, { ...g, match: m })}
                      className={cn(
                        'h-8 rounded-md px-3 text-xs font-extrabold',
                        g.match === m
                          ? 'bg-primary text-primary-foreground'
                          : 'text-muted-foreground',
                      )}
                    >
                      {m.toUpperCase()}
                    </Button>
                  ))}
                </span>
                of the following
                {groups.length > 1 && !readOnly && (
                  <Button
                    variant="ghost"
                    size="sm"
                    className="ml-auto text-destructive"
                    onClick={() => setGroups(groups.filter((_, n) => n !== gi))}
                  >
                    Remove group
                  </Button>
                )}
              </div>
              {g.conditions.map((c, ci) => (
                <ConditionRow
                  // biome-ignore lint/suspicious/noArrayIndexKey: conditions are positional
                  key={ci}
                  condition={c}
                  joiner={ci === 0 ? 'Where' : g.match === 'all' ? 'and' : 'or'}
                  tags={tags}
                  readOnly={readOnly}
                  error={errors[`rules.groups[${gi}].conditions[${ci}]`]}
                  onChange={(next) =>
                    setGroup(gi, {
                      ...g,
                      conditions: g.conditions.map((x, n) => (n === ci ? next : x)),
                    })
                  }
                  onRemove={() =>
                    setGroup(gi, { ...g, conditions: g.conditions.filter((_, n) => n !== ci) })
                  }
                />
              ))}
              {!readOnly && g.conditions.length < 10 && (
                <Button
                  variant="ghost"
                  size="sm"
                  className="self-start text-brand-text"
                  onClick={() =>
                    setGroup(gi, {
                      ...g,
                      conditions: [...g.conditions, blankCondition('tag', tags)],
                    })
                  }
                >
                  <PlusIcon aria-hidden="true" />
                  Add condition
                </Button>
              )}
            </fieldset>
          </div>
        ))}
        {!readOnly && groups.length < 5 && (
          <Button
            variant="outline"
            className="self-start border-dashed"
            onClick={() =>
              setGroups([...groups, { match: 'all', conditions: [blankCondition('tag', tags)] }])
            }
          >
            <PlusIcon aria-hidden="true" />
            Add OR group
          </Button>
        )}
        {errors['rules.groups'] && (
          <span className="text-xs text-destructive">{errors['rules.groups']}</span>
        )}
      </div>

      {words && (
        <p className="rounded-xl bg-info-soft px-3.5 py-3 text-[13px]">
          <b>In plain words:</b> contacts who {words}.
        </p>
      )}

      <div className="flex flex-wrap items-center gap-2.5 border-t pt-4">{actions}</div>
    </section>
  )
}
