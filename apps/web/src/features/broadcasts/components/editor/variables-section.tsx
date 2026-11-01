import { Input } from '#/components/ui/input'
import { SOURCES } from '../../schemas/broadcasts.schema'
import type { BroadcastForm, Variable, VariableSource } from '../../types'
import { controlClass, Select } from '../kit/field'

/** Each {{n}} of the template: a token, where its value comes from, the value, a fallback. */
export function VariablesSection({
  form,
  readOnly,
  errors,
  onChange,
}: {
  form: BroadcastForm
  readOnly: boolean
  errors: Record<string, string>
  onChange: (next: Pick<BroadcastForm, 'header' | 'body'>) => void
}) {
  const rows: { token: string; field: string; v: Variable }[] = [
    ...(form.header ? [{ token: 'H1', field: 'variables.header', v: form.header }] : []),
    ...form.body.map((v, i) => ({ token: `{{${i + 1}}}`, field: `variables.body[${i}]`, v })),
  ]
  if (rows.length === 0) {
    return <p className="text-[13px] text-muted-foreground">This template has no blanks to fill.</p>
  }
  const set = (field: string, patch: Partial<Variable>) => {
    if (field === 'variables.header' && form.header) {
      onChange({ header: { ...form.header, ...patch }, body: form.body })
      return
    }
    const i = Number(field.match(/\[(\d+)\]/)?.[1])
    onChange({
      header: form.header,
      body: form.body.map((v, n) => (n === i ? { ...v, ...patch } : v)),
    })
  }

  return (
    <div className="flex flex-col gap-3">
      {rows.map(({ token, field, v }) => {
        const contactField = v.source === 'name' || v.source === 'attribute'
        const error = errors[field] ?? errors[`${field}.value`] ?? errors[`${field}.fallback`]
        return (
          <div key={field} className="grid grid-cols-[60px_minmax(0,1fr)] items-start gap-2.5">
            <span className="mt-[22px] flex h-8 items-center justify-center rounded-lg bg-info-soft text-[13px] font-extrabold text-info">
              {token}
            </span>
            <div className="flex flex-col gap-1.5">
              <div className="grid gap-2 sm:grid-cols-[170px_minmax(0,1fr)]">
                {/* biome-ignore lint/a11y/noLabelWithoutControl: the control is inside */}
                <label className="flex flex-col gap-1 text-xs font-semibold text-muted-foreground">
                  Value from
                  <Select
                    value={v.source}
                    disabled={readOnly}
                    onValueChange={(source) =>
                      set(field, { source: source as VariableSource, value: '' })
                    }
                    options={SOURCES}
                  />
                </label>
                {v.source === 'text' && (
                  <label className="flex flex-col gap-1 text-xs font-semibold text-muted-foreground">
                    Text
                    <Input
                      unstyled
                      className={controlClass}
                      value={v.value}
                      disabled={readOnly}
                      maxLength={1024}
                      onChange={(e) => set(field, { value: e.target.value })}
                    />
                  </label>
                )}
                {v.source === 'phone' && (
                  <span className="self-end pb-3 text-xs text-muted-foreground">
                    Every contact has one, so no fallback is needed.
                  </span>
                )}
                {contactField && (
                  <div className="grid gap-2 sm:grid-cols-2">
                    {v.source === 'attribute' && (
                      <label className="flex flex-col gap-1 text-xs font-semibold text-muted-foreground">
                        Field key
                        <Input
                          unstyled
                          className={controlClass}
                          value={v.value}
                          disabled={readOnly}
                          placeholder="city"
                          onChange={(e) => set(field, { value: e.target.value })}
                        />
                      </label>
                    )}
                    <label
                      className={
                        v.source === 'name'
                          ? 'col-span-full flex flex-col gap-1 text-xs font-semibold text-muted-foreground'
                          : 'flex flex-col gap-1 text-xs font-semibold text-muted-foreground'
                      }
                    >
                      If empty, use
                      <Input
                        unstyled
                        className={controlClass}
                        value={v.fallback}
                        disabled={readOnly}
                        placeholder={v.source === 'name' ? 'there' : ''}
                        onChange={(e) => set(field, { fallback: e.target.value })}
                      />
                    </label>
                  </div>
                )}
              </div>
              {error && <span className="text-xs font-semibold text-destructive">{error}</span>}
            </div>
          </div>
        )
      })}
    </div>
  )
}
