import { Input } from '#/components/ui/input'
import { Textarea } from '#/components/ui/textarea'
import { AddRow, controlClass, Fld, Select } from '../kit/field'
import type { FormProps } from './automation-drawer'

export function KeywordForm({ settings, errors, onChange }: FormProps) {
  const rules = settings.keyword?.rules ?? []
  const save = (next: typeof rules) => onChange({ keyword: { rules: next } })
  const set = (i: number, patch: Partial<(typeof rules)[number]>) =>
    save(rules.map((r, n) => (n === i ? { ...r, ...patch } : r)))
  return (
    <div className="flex flex-col gap-3">
      {errors.rules && (
        <span className="text-xs font-semibold text-destructive">{errors.rules}</span>
      )}
      {rules.map((r, i) => (
        // biome-ignore lint/suspicious/noArrayIndexKey: rules have no identity until saved
        <div key={i} className="flex flex-col gap-2.5 rounded-xl border p-3.5">
          <div className="flex flex-wrap gap-2.5">
            <Fld
              label="If message contains"
              error={errors[`rules[${i}].keywords`]}
              className="flex-[2_1_180px]"
            >
              <Input
                unstyled
                className={controlClass}
                placeholder="e.g. price, cost"
                value={r.keywords}
                onChange={(e) => set(i, { keywords: e.target.value })}
              />
            </Fld>
            <Fld label="Match" className="flex-[1_1_120px]">
              <Select
                value={r.match}
                onValueChange={(match) => set(i, { match })}
                options={[
                  { value: 'any', label: 'Any word' },
                  { value: 'exact', label: 'Exact phrase' },
                ]}
              />
            </Fld>
          </div>
          <Fld label="Reply with" error={errors[`rules[${i}].reply`]}>
            <Textarea
              unstyled
              className={controlClass}
              rows={2}
              placeholder="Type the automatic reply"
              value={r.reply}
              onChange={(e) => set(i, { reply: e.target.value })}
            />
          </Fld>
        </div>
      ))}
      <AddRow onClick={() => save([...rules, { keywords: '', match: 'any', reply: '' }])}>
        + Add keyword rule
      </AddRow>
    </div>
  )
}
