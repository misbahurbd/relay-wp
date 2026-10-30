import { cn } from '#/lib/utils'
import { TONE, type Tone } from '../../schemas/integrations.schema'

/** The app's letters on a tinted square, standing in for its logo. */
export function AppTile({
  mono,
  tone,
  className,
}: {
  mono: string
  tone: Tone
  className?: string
}) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        'flex size-11 flex-none items-center justify-center rounded-xl text-sm font-extrabold tracking-[-0.02em]',
        TONE[tone],
        className,
      )}
    >
      {mono}
    </span>
  )
}
