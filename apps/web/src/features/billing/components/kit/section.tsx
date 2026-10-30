import type { ComponentProps, ReactNode } from 'react'
import { cn } from '#/lib/utils'

/** A billing card: 16px radius, surface, border, soft shadow. */
export function Section({ className, ...props }: ComponentProps<'section'>) {
  return (
    <section
      className={cn(
        'flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:p-[22px]',
        className,
      )}
      {...props}
    />
  )
}

export function SectionHeader({
  id,
  title,
  description,
  action,
}: {
  id?: string
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex min-w-0 flex-col gap-0.5">
        <h2 id={id} className="text-[17px] font-bold">
          {title}
        </h2>
        {description && <p className="text-[13px] text-muted-foreground">{description}</p>}
      </div>
      {action}
    </div>
  )
}
