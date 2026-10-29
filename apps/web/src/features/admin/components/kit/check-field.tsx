import type { ReactNode } from 'react'
import { Checkbox } from '#/components/ui/checkbox'
import { Label } from '#/components/ui/label'

export function CheckField({
  id,
  label,
  checked,
  onChange,
}: {
  id: string
  label: ReactNode
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <div className="flex items-center gap-2">
      <Checkbox id={id} checked={checked} onCheckedChange={(c) => onChange(c === true)} />
      <Label htmlFor={id}>{label}</Label>
    </div>
  )
}
