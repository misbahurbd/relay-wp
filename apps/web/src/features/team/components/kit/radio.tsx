import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'

export function RadioDot({ checked }: { checked: boolean }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'mt-0.5 box-border size-[18px] flex-none rounded-full bg-card',
        checked ? 'border-[6px] border-primary' : 'border-2 border-input',
      )}
    />
  )
}

/**
 * A native radio drawn as anything: the input is visually hidden and the label carries the look,
 * so arrow keys, forms and screen readers behave as with any radio group.
 */
export function RadioOption({
  name,
  checked,
  onSelect,
  label,
  disabled,
  className,
  children,
}: {
  name: string
  checked: boolean
  onSelect: () => void
  label?: string
  disabled?: boolean
  className?: string
  children: ReactNode
}) {
  return (
    <label
      className={cn(
        'cursor-pointer has-[:disabled]:cursor-default has-[:focus-visible]:outline-2 has-[:focus-visible]:outline-offset-2 has-[:focus-visible]:outline-ring',
        className,
      )}
    >
      <input
        type="radio"
        className="sr-only"
        name={name}
        checked={checked}
        disabled={disabled}
        aria-label={label}
        onChange={onSelect}
      />
      {children}
    </label>
  )
}

/** A selectable card used as a radio (role, scope, reassign choices). */
export function RadioCard({
  name,
  checked,
  onSelect,
  children,
  className,
  disabled,
}: {
  name: string
  checked: boolean
  onSelect: () => void
  children: ReactNode
  className?: string
  disabled?: boolean
}) {
  return (
    <RadioOption
      name={name}
      checked={checked}
      onSelect={onSelect}
      disabled={disabled}
      className={cn(
        'flex items-start gap-3 rounded-lg border-2 px-3.5 py-3 text-left text-foreground has-[:disabled]:opacity-50',
        checked ? 'border-primary bg-brand-soft' : 'border-border bg-card hover:bg-muted',
        className,
      )}
    >
      <RadioDot checked={checked} />
      {children}
    </RadioOption>
  )
}
