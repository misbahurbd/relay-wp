import type { ComponentProps, ReactNode } from 'react'
import { cn } from '#/lib/utils'

export const inputClass =
  'h-11 w-full rounded-[10px] border border-input bg-muted px-3 text-base text-foreground placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60 aria-invalid:border-destructive md:text-sm'

/** A labelled input with its hint and error underneath. */
export function TextField({
  label,
  hint,
  error,
  className,
  id,
  ...props
}: ComponentProps<'input'> & { label: string; hint?: ReactNode; error?: string; id: string }) {
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      <label htmlFor={id} className="text-[13px] font-bold">
        {label}
      </label>
      <input
        id={id}
        aria-invalid={!!error}
        aria-describedby={error ? `${id}-error` : hint ? `${id}-hint` : undefined}
        className={inputClass}
        {...props}
      />
      {hint && !error && (
        <p id={`${id}-hint`} className="text-xs text-muted-foreground">
          {hint}
        </p>
      )}
      <ErrorText id={`${id}-error`}>{error}</ErrorText>
    </div>
  )
}

export function ErrorText({ id, children }: { id?: string; children?: ReactNode }) {
  if (!children) return null
  return (
    <p id={id} role="alert" className="text-[13px] text-destructive">
      {children}
    </p>
  )
}
