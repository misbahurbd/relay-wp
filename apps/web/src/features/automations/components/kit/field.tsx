import type { ComponentProps, ReactNode } from 'react'
import { cn } from '#/lib/utils'

export const controlClass =
  'min-h-11 w-full rounded-[10px] border border-input bg-muted px-3 py-2.5 text-base font-normal text-foreground outline-none focus-visible:ring-2 focus-visible:ring-ring md:text-sm'

/** A labelled control with its server error below it. */
export function Fld({
  label,
  error,
  className,
  children,
}: {
  label: ReactNode
  error?: string
  className?: string
  children: ReactNode
}) {
  return (
    // biome-ignore lint/a11y/noLabelWithoutControl: the control arrives as children
    <label className={cn('flex flex-col gap-1.5 text-[13px] font-bold', className)}>
      {label}
      {children}
      {error && <span className="text-xs font-semibold text-destructive">{error}</span>}
    </label>
  )
}

export function Select({ className, ...props }: ComponentProps<'select'>) {
  return <select className={cn(controlClass, className)} {...props} />
}

export function Check({
  checked,
  onChange,
  children,
}: {
  checked: boolean
  onChange: (checked: boolean) => void
  children: ReactNode
}) {
  return (
    <label className="flex items-center gap-2.5 font-semibold">
      <input
        type="checkbox"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
        className="size-[18px] accent-primary"
      />
      {children}
    </label>
  )
}

/** The dashed "+ Add" button the design uses for rows. */
export function AddRow(props: ComponentProps<'button'>) {
  return (
    <button
      type="button"
      className="h-11 rounded-[10px] border border-dashed bg-transparent font-bold text-brand-text hover:bg-muted"
      {...props}
    />
  )
}

export function ChatPreview({ text }: { text: string }) {
  return (
    <div className="flex flex-col gap-2 rounded-xl bg-chat p-3.5">
      <span className="text-xs font-bold text-muted-foreground">Preview</span>
      <div className="max-w-[85%] self-start whitespace-pre-wrap rounded-[4px_14px_14px_14px] bg-bubble-in px-2.5 py-2 text-[13px]">
        {text}
      </div>
    </div>
  )
}
