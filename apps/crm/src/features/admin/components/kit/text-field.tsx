import type { ComponentProps, ReactNode } from 'react'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'

type Props = Omit<ComponentProps<typeof Input>, 'onChange'> & {
  id: string
  label: ReactNode
  hint?: ReactNode
  /** Shown below the field in place of the hint. */
  error?: string
  onChange: (value: string) => void
}

export function TextField({ id, label, hint, error, onChange, ...props }: Props) {
  const note = error ?? hint
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <Label htmlFor={id}>{label}</Label>
      <Input
        id={id}
        onChange={(e) => onChange(e.target.value)}
        aria-invalid={error ? true : undefined}
        aria-describedby={note ? `${id}-note` : undefined}
        {...props}
      />
      {note && (
        <p
          id={`${id}-note`}
          className={`text-xs ${error ? 'text-destructive' : 'text-muted-foreground'}`}
        >
          {note}
        </p>
      )}
    </div>
  )
}
