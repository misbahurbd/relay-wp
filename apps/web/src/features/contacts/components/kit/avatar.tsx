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

/** Initials in a coloured circle; the colour is stable per contact. */
export function ContactAvatar({
  name,
  seed,
  size = 'md',
}: {
  name: string
  seed: string
  size?: 'md' | 'lg'
}) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex flex-none items-center justify-center rounded-full font-extrabold',
        size === 'md' ? 'size-9 text-[13px]' : 'size-16 text-[22px]',
        toneFor(seed),
      )}
    >
      {initials(name)}
    </span>
  )
}
