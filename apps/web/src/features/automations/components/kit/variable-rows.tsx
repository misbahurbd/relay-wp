import { Input } from '#/components/ui/input'
import type { TemplateVariable } from '../../types'
import { controlClass, Select } from './field'

/** Maps each {{n}} of the chosen template to a source. */
export function VariableRows({
  variables,
  sources,
  onChange,
  errors,
}: {
  variables: TemplateVariable[]
  sources: { value: string; label: string }[]
  onChange: (variables: TemplateVariable[]) => void
  errors: Record<string, string>
}) {
  if (variables.length === 0) return null
  const set = (i: number, patch: Partial<TemplateVariable>) =>
    onChange(variables.map((v, n) => (n === i ? { ...v, ...patch } : v)))
  return (
    <div className="flex flex-col gap-2">
      <span className="text-[13px] font-bold">Fill template variables</span>
      {variables.map((v, i) => {
        const key = `{{${i + 1}}}`
        const error = errors[`variables[${i}]`]
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: a template's variables are positional
          <div key={i} className="flex flex-col gap-1">
            <div className="flex items-center gap-2.5">
              <span className="w-[52px] flex-none rounded-lg bg-info-soft py-1.5 text-center text-xs font-extrabold text-info">
                {key}
              </span>
              <Select
                aria-label={`Value for ${key}`}
                value={v.source}
                onChange={(e) => set(i, { source: e.target.value })}
              >
                <option value="">Choose a value</option>
                {sources.map((s) => (
                  <option key={s.value} value={s.value}>
                    {s.label}
                  </option>
                ))}
              </Select>
            </div>
            {v.source === 'custom' && (
              <Input
                unstyled
                className={controlClass}
                aria-label={`Custom text for ${key}`}
                value={v.custom}
                onChange={(e) => set(i, { custom: e.target.value })}
              />
            )}
            {error && <span className="text-xs font-semibold text-destructive">{error}</span>}
          </div>
        )
      })}
    </div>
  )
}

/** Keeps the rows in step with the chosen template's variable count. */
export function fitVariables(variables: TemplateVariable[], count: number): TemplateVariable[] {
  return Array.from({ length: count }, (_, i) => variables[i] ?? { source: '', custom: '' })
}
