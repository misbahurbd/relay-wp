import { cn } from '#/lib/utils'

const TONES = [
  'bg-brand-soft text-brand-text',
  'bg-info-soft text-info',
  'bg-warning-soft text-warning',
  'bg-destructive-soft text-destructive',
  'bg-neutral-soft text-neutral',
] as const

export function initials(name: string) {
  return (
    name
      .split(/\s+/)
      .filter(Boolean)
      .slice(0, 2)
      .map((w) => w[0]?.toUpperCase())
      .join('') || '?'
  )
}

function toneFor(seed: string) {
  let h = 0
  for (const c of seed) h = (h * 31 + c.charCodeAt(0)) >>> 0
  return TONES[h % TONES.length]
}

const SIZE = {
  xs: 'size-6 text-[10px]',
  sm: 'size-10 text-sm',
  md: 'size-11 text-sm',
  lg: 'size-16 text-[22px]',
}

/** Initials in a coloured circle; the colour is stable per person. Decorative: the name is beside it. */
export function Avatar({
  name,
  seed,
  size = 'md',
  className,
}: {
  name: string
  seed: string
  size?: keyof typeof SIZE
  className?: string
}) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex flex-none items-center justify-center rounded-full font-extrabold',
        SIZE[size],
        toneFor(seed),
        className,
      )}
    >
      {initials(name)}
    </span>
  )
}
