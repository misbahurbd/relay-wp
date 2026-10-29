import type { ComponentProps, ReactNode } from 'react'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'

type Props = Omit<ComponentProps<typeof Input>, 'onChange'> & {
  id: string
  label: ReactNode
  hint?: ReactNode
  onChange: (value: string) => void
}

export function TextField({ id, label, hint, onChange, ...props }: Props) {
  return (
    <div className="flex flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input id={id} onChange={(e) => onChange(e.target.value)} {...props} />
      {hint && <p className="text-xs text-muted-foreground">{hint}</p>}
    </div>
  )
}
