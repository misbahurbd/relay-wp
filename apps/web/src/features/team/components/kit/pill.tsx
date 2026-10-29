import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'
import { TONE } from '../../schemas/team.schema'
import type { RoleColor } from '../../types'

/** Small rounded label: role names, states, badges. */
export function Pill({
  tone = 'gray',
  children,
  className,
  square,
}: {
  tone?: RoleColor
  children: ReactNode
  className?: string
  square?: boolean
}) {
  const t = TONE[tone]
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 py-0.5 text-xs font-bold whitespace-nowrap',
        square ? 'rounded-md px-2' : 'rounded-full px-2.5',
        t.soft,
        t.text,
        className,
      )}
    >
      {children}
    </span>
  )
}
