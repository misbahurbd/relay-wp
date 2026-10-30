import type { ComponentProps } from 'react'
import { cn } from '#/lib/utils'

/** Buttons in the prototype's sizes: lg 44px/12px radius, md 40px/10px, sm 34px. */
export function Btn({
  variant = 'primary',
  size = 'md',
  className,
  type = 'button',
  ...props
}: ComponentProps<'button'> & {
  variant?: 'primary' | 'outline' | 'danger' | 'danger-soft' | 'soft' | 'ghost'
  size?: 'lg' | 'md' | 'sm'
}) {
  return (
    <button
      type={type}
      className={cn(
        'inline-flex items-center justify-center gap-2 font-bold whitespace-nowrap disabled:cursor-not-allowed disabled:opacity-50',
        size === 'lg' && 'h-11 rounded-lg px-[18px]',
        size === 'md' && 'h-10 rounded-[10px] px-3.5 text-[13px]',
        size === 'sm' && 'h-[34px] rounded-[10px] px-3 text-[13px]',
        variant === 'primary' && 'bg-primary text-primary-foreground hover:bg-primary/90',
        variant === 'outline' && 'border bg-card text-foreground hover:bg-muted',
        variant === 'danger' &&
          'bg-destructive text-white hover:bg-destructive/90 dark:text-background',
        variant === 'danger-soft' && 'bg-destructive-soft text-destructive hover:opacity-80',
        variant === 'soft' && 'bg-brand-soft text-brand-text hover:opacity-80',
        variant === 'ghost' && 'bg-transparent text-brand-text hover:bg-muted',
        className,
      )}
      {...props}
    />
  )
}
