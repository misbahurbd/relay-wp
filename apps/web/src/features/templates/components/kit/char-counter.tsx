import { cn } from '#/lib/utils'
import { charCount } from '../../schemas/templates.schema'

/** "12/60" under a text field; turns red past the limit. */
export function CharCounter({ id, value, max }: { id?: string; value: string; max: number }) {
  const n = charCount(value)
  return (
    <span
      id={id}
      className={cn(
        'text-xs tabular-nums',
        n > max ? 'font-bold text-destructive' : 'text-muted-foreground',
      )}
    >
      <span className="sr-only">Characters used: </span>
      {n}/{max}
    </span>
  )
}
