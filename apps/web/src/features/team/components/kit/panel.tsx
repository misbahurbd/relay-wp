import { Link } from '@tanstack/react-router'
import { ChevronLeftIcon } from 'lucide-react'
import type { ComponentProps, ReactNode } from 'react'
import { cn } from '#/lib/utils'

/** The prototype's .card: 16px radius, surface, border, soft shadow. */
export function Panel({ className, ...props }: ComponentProps<'section'>) {
  return (
    <section
      className={cn(
        'flex flex-col rounded-xl border bg-card px-5 py-5 shadow-card sm:px-[22px]',
        className,
      )}
      {...props}
    />
  )
}

export function PanelHeader({
  title,
  description,
  action,
  id,
}: {
  title: ReactNode
  description?: ReactNode
  action?: ReactNode
  id?: string
}) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3">
      <div className="flex flex-col gap-0.5">
        <h2 id={id} className="text-[17px] font-bold tracking-normal">
          {title}
        </h2>
        {description && <span className="text-[13px] text-muted-foreground">{description}</span>}
      </div>
      {action}
    </div>
  )
}

/** Uppercase section label (.h-sec). */
export function SectionLabel({ children, className }: { children: ReactNode; className?: string }) {
  return (
    <span
      className={cn(
        'text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase',
        className,
      )}
    >
      {children}
    </span>
  )
}

export function Breadcrumb({ current }: { current: string }) {
  return (
    <nav
      aria-label="Breadcrumb"
      className="flex items-center gap-2 text-[13px] font-semibold text-muted-foreground"
    >
      <Link
        to="/team"
        className="inline-flex items-center gap-1.5 text-brand-text no-underline hover:text-foreground"
      >
        <ChevronLeftIcon className="size-4" aria-hidden="true" />
        Team
      </Link>
      <span aria-hidden="true">/</span>
      <span aria-current="page" className="text-foreground">
        {current}
      </span>
    </nav>
  )
}
