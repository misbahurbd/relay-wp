import type { ReactNode } from 'react'
import { Label } from '#/components/ui/label'
import { RadioGroupItem } from '#/components/ui/radio-group'
import { cn } from '#/lib/utils'

/** A radio choice drawn as a card: a template, "Send now", "Schedule for later". */
export function OptionCard({
  id,
  value,
  selected,
  icon,
  children,
}: {
  id: string
  value: string
  selected: boolean
  icon?: ReactNode
  children: ReactNode
}) {
  return (
    <Label
      htmlFor={id}
      className={cn(
        'flex cursor-pointer items-start gap-3 rounded-[14px] border-[1.5px] p-3.5 font-normal leading-snug',
        selected ? 'border-primary bg-brand-soft' : 'border-border bg-card hover:border-primary/60',
      )}
    >
      <RadioGroupItem id={id} value={value} className={icon ? 'sr-only' : 'mt-0.5'} />
      {icon && (
        <span className="text-brand-text [&_svg]:size-[22px]" aria-hidden="true">
          {icon}
        </span>
      )}
      <span className="flex min-w-0 flex-1 flex-col gap-0.5">{children}</span>
    </Label>
  )
}
