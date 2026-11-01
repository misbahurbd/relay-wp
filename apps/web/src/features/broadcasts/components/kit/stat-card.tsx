import type { ReactNode } from 'react'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'

/** One number on a summary row: an icon, the value, and what it counts. */
export function StatCard({
  icon,
  tone,
  value,
  label,
  detail,
}: {
  icon: ReactNode
  tone: string
  value?: ReactNode
  label: string
  detail?: string
}) {
  return (
    <div className="flex items-center gap-3.5 rounded-xl border bg-card p-4 shadow-card">
      <span
        className={cn(
          'flex size-11 flex-none items-center justify-center rounded-lg [&_svg]:size-5',
          tone,
        )}
      >
        {icon}
      </span>
      <span className="flex min-w-0 flex-col">
        {value === undefined ? (
          <Skeleton className="h-7 w-10" />
        ) : (
          <span className="text-2xl font-extrabold tabular-nums">{value}</span>
        )}
        <span className="text-[13px] text-muted-foreground">
          {label}
          {detail && <span className="ml-1 font-semibold text-foreground">{detail}</span>}
        </span>
      </span>
    </div>
  )
}
