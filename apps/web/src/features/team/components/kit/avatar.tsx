import type { ReactNode } from 'react'
import { cn } from '#/lib/utils'
import { TONE, toneFor } from '../../schemas/team.schema'
import type { RoleColor } from '../../types'

export function initials(name: string) {
  return name
    .split(/[\s@.]+/)
    .filter(Boolean)
    .slice(0, 2)
    .map((w) => w[0]?.toUpperCase())
    .join('')
}

export function PersonAvatar({
  name,
  seed,
  size = 38,
  tone,
  className,
  children,
}: {
  name: string
  seed?: string
  size?: number
  tone?: RoleColor
  className?: string
  children?: ReactNode
}) {
  const t = TONE[tone ?? toneFor(seed ?? name)]
  return (
    <span
      aria-hidden="true"
      style={{ width: size, height: size, fontSize: size >= 60 ? 22 : size >= 38 ? 13 : 11 }}
      className={cn(
        'relative flex flex-none items-center justify-center rounded-full font-extrabold',
        t.soft,
        t.text,
        className,
      )}
    >
      {initials(name)}
      {children}
    </span>
  )
}
