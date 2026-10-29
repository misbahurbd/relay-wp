import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'

/** One numbered card of the template form: "3. Body · Required", with optional tools. */
export function FormSection({
  id,
  title,
  aside,
  tools,
  className,
  children,
}: {
  id: string
  title: string
  aside?: string
  tools?: ReactNode
  className?: string
  children: ReactNode
}) {
  return (
    <section
      aria-labelledby={id}
      className={cn(
        'flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:p-6',
        className,
      )}
    >
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex items-baseline gap-2">
          <h2 id={id} className="text-base font-extrabold">
            {title}
          </h2>
          {aside && <span className="text-[13px] text-muted-foreground">{aside}</span>}
        </div>
        {tools}
      </div>
      {children}
    </section>
  )
}
