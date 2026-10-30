import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'
import { TONE, type Tone } from '../../schemas/integrations.schema'

/** Small rounded label for states: Connected, Failed, Off. */
export function Pill({
  tone = 'gray',
  dot,
  children,
  className,
}: {
  tone?: Tone
  dot?: boolean
  children: ReactNode
  className?: string
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold whitespace-nowrap',
        TONE[tone],
        className,
      )}
    >
      {dot && <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />}
      {children}
    </span>
  )
}
