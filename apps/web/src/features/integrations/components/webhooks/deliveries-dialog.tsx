import { useInfiniteQuery } from '@tanstack/react-query'
import { ChevronRightIcon, RotateCwIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useRedeliver } from '../../api/integrations.mutations'
import { deliveriesQueryOptions } from '../../api/integrations.queries'
import type { Delivery, Endpoint } from '../../types'
import { Btn } from '../kit/btn'
import { formatDateTime } from '../kit/format'
import { Modal } from '../kit/modal'
import { Pill } from '../kit/pill'

/** The endpoint's delivery log: newest first, each expandable to its body and error. */
export function DeliveriesDialog({
  workspaceId,
  endpoint,
  onClose,
}: {
  workspaceId: string
  endpoint: Endpoint
  onClose: () => void
}) {
  const log = useInfiniteQuery(deliveriesQueryOptions(workspaceId, endpoint.id))
  const redeliver = useRedeliver(workspaceId)
  const [openId, setOpenId] = useState<string>()
  const rows = log.data?.pages.flatMap((p) => p.deliveries) ?? []

  async function resend(d: Delivery) {
    try {
      await redeliver.mutateAsync({ endpointId: endpoint.id, deliveryId: d.id })
      toast.success('Queued to send again')
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <Modal
      onClose={onClose}
      title="Deliveries"
      description={<span className="break-all font-mono">{endpoint.url}</span>}
      width="sm:max-w-[720px]"
    >
      {log.isPending ? (
        <Skeleton className="h-40 rounded-[10px]" />
      ) : log.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {errorMessage(log.error)}
        </p>
      ) : rows.length === 0 ? (
        <p className="rounded-[10px] border border-dashed p-6 text-center text-[13px] text-muted-foreground">
          Nothing sent yet. Use Send test event to try the endpoint. The log keeps 30 days.
        </p>
      ) : (
        <ul className="flex flex-col divide-y rounded-[10px] border">
          {rows.map((d) => {
            const open = openId === d.id
            return (
              <li key={d.id} className="flex flex-col">
                <Button
                  variant="unstyled"
                  type="button"
                  aria-expanded={open}
                  onClick={() => setOpenId(open ? undefined : d.id)}
                  className="flex items-center gap-3 p-3 text-left hover:bg-muted"
                >
                  <ChevronRightIcon
                    className={cn(
                      'size-4 flex-none text-muted-foreground transition-transform',
                      open && 'rotate-90',
                    )}
                    aria-hidden="true"
                  />
                  <DeliveryPill d={d} />
                  <code className="min-w-0 flex-1 truncate font-mono text-[13px]">
                    {d.event_type}
                  </code>
                  <span className="hidden text-xs text-muted-foreground sm:inline">
                    {d.response_ms != null && `${d.response_ms} ms · `}
                    {formatDateTime(d.created_at)}
                  </span>
                </Button>
                {open && (
                  <div className="flex flex-col gap-2 px-3 pb-3 pl-10">
                    <span className="text-xs text-muted-foreground sm:hidden">
                      {formatDateTime(d.created_at)}
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {d.attempts} {d.attempts === 1 ? 'attempt' : 'attempts'}
                      {d.error && ` · ${d.error}`}
                    </span>
                    <pre className="max-h-64 overflow-auto rounded-[10px] bg-muted p-3 font-mono text-xs">
                      {JSON.stringify(d.payload, null, 2)}
                    </pre>
                    {d.status !== 'pending' && (
                      <Btn
                        size="sm"
                        variant="outline"
                        className="self-start"
                        onClick={() => resend(d)}
                        disabled={redeliver.isPending}
                      >
                        <RotateCwIcon className="size-3.5" aria-hidden="true" />
                        Send again
                      </Btn>
                    )}
                  </div>
                )}
              </li>
            )
          })}
        </ul>
      )}
      {log.hasNextPage && (
        <Btn
          variant="outline"
          className="self-center"
          onClick={() => log.fetchNextPage()}
          disabled={log.isFetchingNextPage}
        >
          {log.isFetchingNextPage ? 'Loading…' : 'Load older'}
        </Btn>
      )}
    </Modal>
  )
}

export function DeliveryPill({ d }: { d: { status: string; response_status: number | null } }) {
  const code = d.response_status ? ` ${d.response_status}` : ''
  if (d.status === 'succeeded') return <Pill tone="green">{`OK${code}`}</Pill>
  if (d.status === 'failed') return <Pill tone="red">{`Failed${code}`}</Pill>
  return <Pill tone="amber">{code ? `Retrying${code}` : 'Sending'}</Pill>
}
