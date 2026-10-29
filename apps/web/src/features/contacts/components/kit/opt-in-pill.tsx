import { cn } from '#/lib/utils'
import { OPT_IN_LABEL } from '../../schemas/contacts.schema'
import type { OptIn } from '../../types'

const TONE: Record<OptIn, string> = {
  in: 'bg-brand-soft text-brand-text',
  out: 'bg-destructive-soft text-destructive',
  pending: 'bg-warning-soft text-warning',
}

/** Opt-in status as a dot plus text, so it doesn't rely on colour. */
export function OptInPill({ value, className }: { value: OptIn; className?: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold whitespace-nowrap',
        TONE[value],
        className,
      )}
    >
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {OPT_IN_LABEL[value]}
    </span>
  )
}
