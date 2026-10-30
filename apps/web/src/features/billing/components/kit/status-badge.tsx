import { Badge } from '#/components/ui/badge'
import type { Tone } from '../../schemas/billing.schema'

/** A status in words and colour, never colour alone: a pill with a dot. */
export function StatusBadge({ status }: { status: { label: string; tone: Tone } }) {
  return (
    <Badge variant={status.tone} className="gap-1.5 rounded-full px-2.5 font-bold">
      <span className="size-1.5 rounded-full bg-current" aria-hidden="true" />
      {status.label}
    </Badge>
  )
}
