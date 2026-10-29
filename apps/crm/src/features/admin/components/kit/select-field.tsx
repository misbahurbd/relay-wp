import type { ReactNode } from 'react'
import { Label } from '#/components/ui/label'

type Props<T extends string> = {
  id: string
  label: ReactNode
  value: T
  options: { value: T; label: string }[]
  onChange: (value: T) => void
  hint?: ReactNode
}

/** A native select: enough for staff tools, and it works everywhere. */
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
      <select
        id={id}
        value={value}
        onChange={(e) => onChange(e.target.value as T)}
        className="h-11 rounded-md border border-input bg-card px-3 text-sm outline-none focus-visible:border-ring focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"
      >
        {options.map((o) => (
          <option key={o.value} value={o.value}>
            {o.label}
          </option>
        ))}
      </select>
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}
