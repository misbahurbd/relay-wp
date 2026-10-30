import { useId } from 'react'
import { cn } from '#/lib/utils'

/** A labelled on/off switch with a hint line. Local state only on this page. */
export function SwitchRow({
  label,
  hint,
  checked,
  onChange,
}: {
  label: string
  hint?: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  const id = useId()
  return (
    <div className="flex items-center gap-3">
      <div className="flex min-w-0 flex-1 flex-col leading-snug">
        <span id={`${id}-label`} className="text-sm font-bold">
          {label}
        </span>
        {hint && (
          <span id={`${id}-hint`} className="text-xs text-muted-foreground">
            {hint}
          </span>
        )}
      </div>
      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-labelledby={`${id}-label`}
        aria-describedby={hint ? `${id}-hint` : undefined}
        onClick={() => onChange(!checked)}
        className={cn(
          'relative inline-flex h-7 w-12 shrink-0 items-center rounded-full border border-input p-0.5 transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
          checked ? 'justify-end bg-primary' : 'justify-start bg-muted',
        )}
      >
        <span
          aria-hidden="true"
          className={cn(
            'size-5 rounded-full shadow-sm',
            checked ? 'bg-primary-foreground' : 'bg-card',
          )}
        />
      </button>
    </div>
  )
}
