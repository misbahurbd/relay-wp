import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

/** The design's pill switch (aria-pressed button). Small is the drawer's day switch. */
export function Toggle({
  on,
  onChange,
  label,
  disabled,
  title,
  small,
}: {
  on: boolean
  onChange: (on: boolean) => void
  label: string
  disabled?: boolean
  title?: string
  small?: boolean
}) {
  return (
    <Button
      variant="unstyled"
      type="button"
      aria-pressed={on}
      aria-label={label}
      disabled={disabled}
      title={title}
      onClick={() => onChange(!on)}
      className={cn(
        'flex flex-none rounded-full p-[3px] transition-colors disabled:cursor-not-allowed disabled:opacity-50',
        small ? 'h-6 w-10' : 'h-7 w-12',
        on ? 'justify-end bg-primary' : 'justify-start bg-input',
      )}
    >
      <span
        className={cn(
          'rounded-full bg-white shadow-[0_1px_3px_rgba(0,0,0,.25)]',
          small ? 'size-[18px]' : 'size-[22px]',
        )}
      />
    </Button>
  )
}
