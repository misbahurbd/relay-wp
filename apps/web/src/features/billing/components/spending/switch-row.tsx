import { cn } from '#/lib/utils'

/** The design's on/off switch: a 48×28 track, green when on, with a white knob. */
export function Switch({
  label,
  checked,
  onChange,
}: {
  label: string
  checked: boolean
  onChange: (checked: boolean) => void
}) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={label}
      onClick={() => onChange(!checked)}
      className={cn(
        'box-border flex h-7 w-12 flex-none rounded-[14px] p-[3px] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring',
        checked ? 'justify-end bg-primary' : 'justify-start bg-border',
      )}
    >
      <span className="size-[22px] rounded-full bg-white shadow-[0_1px_2px_rgba(0,0,0,.2)]" />
    </button>
  )
}

/** A label and hint beside a Switch. Local state only on this page. */
export function SwitchRow({
  label,
  hint,
  checked,
  onChange,
  className,
}: {
  label: string
  hint: string
  checked: boolean
  onChange: (checked: boolean) => void
  className?: string
}) {
  return (
    <div className={cn('flex items-center gap-3', className)}>
      <span className="flex flex-1 flex-col leading-[1.35]">
        <span className="font-bold">{label}</span>
        <span className="text-xs text-muted-foreground">{hint}</span>
      </span>
      <Switch label={label} checked={checked} onChange={onChange} />
    </div>
  )
}
