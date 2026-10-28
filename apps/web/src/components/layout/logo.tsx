import { Link } from '@tanstack/react-router'
import { cn } from '#/lib/utils'

export function LogoMark({ className }: { className?: string }) {
  return (
    <span
      className={cn(
        'flex size-9 shrink-0 items-center justify-center rounded-[11px] bg-primary text-primary-foreground',
        className,
      )}
      aria-hidden="true"
    >
      <svg
        viewBox="0 0 24 24"
        aria-hidden="true"
        className="size-5"
        fill="none"
        stroke="currentColor"
        strokeWidth="2"
      >
        <path d="M7.9 20A9 9 0 1 0 4 16.1L2 22Z" strokeLinejoin="round" />
        <path d="M8 12h2l1.5-3 2 6 1.5-3h1" strokeLinecap="round" strokeLinejoin="round" />
      </svg>
    </span>
  )
}

export function Logo({ to = '/' }: { to?: string }) {
  return (
    <Link to={to} className="flex items-center gap-2.5 rounded-lg text-foreground no-underline">
      <LogoMark />
      <span className="text-lg font-extrabold tracking-tight">Relay</span>
    </Link>
  )
}
