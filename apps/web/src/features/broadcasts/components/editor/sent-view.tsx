import { Link } from '@tanstack/react-router'
import { CheckIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import type { Broadcast } from '../../types'
import { formatCount, formatWhen } from '../kit/format'

/** After send: on its way, or scheduled, with the report one click away. */
export function SentView({
  broadcast: b,
  recipients,
  onAnother,
}: {
  broadcast: Broadcast
  recipients?: number
  onAnother: () => void
}) {
  const scheduled = b.status === 'scheduled'
  const who = recipients === undefined ? 'your audience' : `${formatCount(recipients)} contacts`
  return (
    <div className="flex flex-col items-center gap-3.5 px-3 py-10 text-center">
      <span className="flex size-[72px] items-center justify-center rounded-full bg-brand-soft text-brand-text">
        <CheckIcon className="size-[34px]" strokeWidth={2.5} aria-hidden="true" />
      </span>
      <h2 className="text-[22px] font-extrabold">
        {scheduled ? 'Broadcast scheduled' : 'Your broadcast is on its way'}
      </h2>
      <p className="max-w-[460px] text-muted-foreground">
        {scheduled
          ? `It will go out ${formatWhen(b.scheduled_at)} to ${who}. You can cancel it until then.`
          : `Messages to ${who} are being sent in batches. Follow delivery and reads live in the report.`}
      </p>
      <div className="mt-1.5 flex flex-wrap justify-center gap-2.5">
        <Button asChild>
          <Link to="/broadcasts/$broadcastId" params={{ broadcastId: b.id }}>
            {scheduled ? 'View broadcast' : 'View live report'}
          </Link>
        </Button>
        <Button variant="outline" onClick={onAnother}>
          Create another
        </Button>
      </div>
    </div>
  )
}
