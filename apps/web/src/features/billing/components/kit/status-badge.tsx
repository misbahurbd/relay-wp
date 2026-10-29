import { Badge } from '#/components/ui/badge'
import type { Tone } from '../../schemas/billing.schema'

/** A status in words and colour, never colour alone. */
export function StatusBadge({ status }: { status: { label: string; tone: Tone } }) {
  return <Badge variant={status.tone}>{status.label}</Badge>
}
