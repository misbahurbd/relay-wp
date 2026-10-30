import { CheckCheckIcon, CheckIcon, CircleAlertIcon, Clock3Icon } from 'lucide-react'
import type { Message } from '../../types'

/** Delivery state of an outbound message: clock, one or two ticks (blue once read), or an alert. */
export function Ticks({ status }: { status: Message['status'] }) {
  switch (status) {
    case 'pending':
      return <Clock3Icon role="img" aria-label="Sending" className="size-3.5 text-tick-sent" />
    case 'sent':
      return <CheckIcon role="img" aria-label="Sent" className="size-4 text-tick-sent" />
    case 'delivered':
      return <CheckCheckIcon role="img" aria-label="Delivered" className="size-4 text-tick-sent" />
    case 'read':
      return <CheckCheckIcon role="img" aria-label="Read" className="size-4 text-tick-read" />
    case 'failed':
      return (
        <CircleAlertIcon
          role="img"
          aria-label="Not delivered"
          className="size-4 text-destructive"
        />
      )
  }
}
