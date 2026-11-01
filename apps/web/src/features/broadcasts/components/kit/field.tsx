import type { ComponentProps, ReactNode } from 'react'
import { OptionSelect } from '#/components/ui/option-select'
import { cn } from '#/lib/utils'

export const controlClass =
  'min-h-11 w-full rounded-[10px] border border-input bg-muted px-3 py-2.5 text-base font-normal text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring md:text-sm'

/** A labelled control with its server error below it. */
export function Fld({
  label,
  hint,
  error,
  className,
  children,
}: {
  label: ReactNode
  hint?: ReactNode
  error?: string
  className?: string
  children: ReactNode
}) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: the control arrives as children
    <label className={cn('flex flex-col gap-1.5 text-[13px] font-bold', className)}>
      {label}
      {children}
      {hint && <span className="text-xs font-normal text-muted-foreground">{hint}</span>}
      {error && <span className="text-xs font-semibold text-destructive">{error}</span>}
    </label>
  )
}

/** The kit's select box: OptionSelect in the form's look. */
export function Select({ className, ...props }: ComponentProps<typeof OptionSelect>) {
  return (
    <OptionSelect
      className={cn('h-auto min-h-11 rounded-[10px] bg-muted text-base md:text-sm', className)}
      {...props}
    />
  )
}
