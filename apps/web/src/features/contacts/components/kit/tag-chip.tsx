import { XIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'

/** A tag label; with onRemove it gets a remove button. */
export function TagChip({
  name,
  onRemove,
  round,
}: {
  name: string
  onRemove?: () => void
  round?: boolean
}) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 bg-neutral-soft py-0.5 text-xs font-semibold whitespace-nowrap text-foreground',
        round ? 'rounded-full px-2.5 py-1' : 'rounded-md px-2',
      )}
    >
      {name}
      {onRemove && (
        <Button
          variant="unstyled"
          type="button"
          aria-label={`Remove tag ${name}`}
          onClick={onRemove}
          className="-mr-1 flex size-5 items-center justify-center rounded-full text-muted-foreground hover:bg-accent hover:text-foreground"
        >
          <XIcon className="size-3" aria-hidden="true" />
        </Button>
      )}
    </span>
  )
}
