import type { ReactNode } from 'react'
import { Label } from '#/components/ui/label'
import { OptionSelect } from '#/components/ui/option-select'

type Props<T extends string> = {
  id: string
  label: ReactNode
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
  hint?: ReactNode
}

/** A labelled select box (OptionSelect: no native selects, CLAUDE.md). */
export function SelectField<T extends string>({
  id,
  label,
  value,
  options,
  onChange,
  hint,
}: Props<T>) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <OptionSelect
        id={id}
        value={value}
        onValueChange={(v) => onChange(v as T)}
        options={options}
        className="h-11 rounded-md bg-card"
      />
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}
