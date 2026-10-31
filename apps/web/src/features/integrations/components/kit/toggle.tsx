import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

/** The prototype's pill switch (aria-pressed button). */
export function Toggle({
  on,
  onChange,
  label,
  disabled,
}: {
  on: boolean
  onChange: (on: boolean) => void
  label: string
  disabled?: boolean
}) {
  return (
    <Button
      variant="unstyled"
      type="button"
      aria-pressed={on}
      aria-label={label}
      disabled={disabled}
      onClick={() => onChange(!on)}
      className={cn(
        'relative h-7 w-[46px] flex-none rounded-full transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        on ? 'bg-primary' : 'bg-input/60',
      )}
    >
      <span
        className={cn(
          'absolute top-1 size-5 rounded-full bg-white shadow-[0_1px_2px_rgba(0,0,0,.25)] transition-[left]',
          on ? 'left-[22px]' : 'left-1',
        )}
      />
    </Button>
  )
}
