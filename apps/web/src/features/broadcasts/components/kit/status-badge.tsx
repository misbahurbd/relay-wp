import { Badge } from '#/components/ui/badge'
import { STATUS_LABEL } from '../../schemas/broadcasts.schema'
import type { BroadcastStatus } from '../../types'

const VARIANT = {
  draft: 'neutral',
  scheduled: 'info',
  sending: 'warning',
  completed: 'success',
  cancelled: 'neutral',
  failed: 'danger',
} as const satisfies Record<BroadcastStatus, string>

/** The broadcast's status as a pill with a dot; the word carries the meaning, not the colour. */
export function StatusBadge({ status }: { status: BroadcastStatus }) {
  return (
    <Badge variant={VARIANT[status]}>
      <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
      {STATUS_LABEL[status]}
    </Badge>
  )
}
