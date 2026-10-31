import { useInfiniteQuery } from '@tanstack/react-query'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { recipientsQueryOptions } from '../../api/broadcasts.queries'
import { FAILURE_LABEL } from '../../schemas/broadcasts.schema'
import type { Recipient, RecipientFilter } from '../../types'
import { formatWhen } from '../kit/format'

const FILTERS: { value?: RecipientFilter; label: string }[] = [
  { label: 'All' },
  { value: 'delivered', label: 'Delivered' },
  { value: 'read', label: 'Read' },
  { value: 'failed', label: 'Not delivered' },
]

/** What happened to one recipient, as a pill. */
function Outcome({ r }: { r: Recipient }) {
  if (r.state === 'pending') return <Badge variant="neutral">Waiting</Badge>
  if (r.state === 'skipped') return <Badge variant="neutral">Skipped</Badge>
  if (r.state === 'failed' || r.message_status === 'failed')
    return <Badge variant="danger">Failed</Badge>
  switch (r.message_status) {
    case 'read':
      return <Badge variant="success">Read</Badge>
    case 'delivered':
      return <Badge variant="info">Delivered</Badge>
    case 'sent':
      return <Badge variant="neutral">Sent</Badge>
    default:
      return <Badge variant="warning">Sending</Badge>
  }
}

export function RecipientsTable({
  workspaceId,
  broadcastId,
  filter,
  onFilter,
}: {
  workspaceId: string
  broadcastId: string
  filter?: RecipientFilter
  onFilter: (f?: RecipientFilter) => void
}) {
  const list = useInfiniteQuery(recipientsQueryOptions(workspaceId, broadcastId, filter))
  const rows = list.data?.pages.flatMap((p) => p.items) ?? []
  return (
    <section
      aria-labelledby="bc-recipients"
      className="flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:p-6"
    >
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 id="bc-recipients" className="text-base font-extrabold">
          Recipients
        </h2>
        <div role="tablist" aria-label="Filter recipients" className="flex flex-wrap gap-1.5">
          {FILTERS.map((f) => (
            <Button
              key={f.label}
              role="tab"
              aria-selected={filter === f.value}
              variant={filter === f.value ? 'secondary' : 'ghost'}
              size="sm"
              onClick={() => onFilter(f.value)}
            >
              {f.label}
            </Button>
          ))}
        </div>
      </div>
      {list.isError ? (
        <Alert variant="danger">
          <AlertDescription className="flex flex-wrap items-center gap-3">
            {errorMessage(list.error, 'Couldn’t load recipients.')}
            <Button variant="outline" size="sm" onClick={() => list.refetch()}>
              Try again
            </Button>
          </AlertDescription>
        </Alert>
      ) : (
        <div className="-mx-5 overflow-x-auto px-5 sm:-mx-6 sm:px-6">
          <table
            className="w-full min-w-[680px] border-collapse text-sm"
            aria-busy={list.isPending}
          >
            <thead>
              <tr className="text-left text-xs text-muted-foreground">
                <th scope="col" className="py-2 pr-2 font-semibold">
                  Contact
                </th>
                <th scope="col" className="p-2 font-semibold">
                  Status
                </th>
                <th scope="col" className="p-2 font-semibold">
                  Updated
                </th>
                <th scope="col" className="py-2 pl-2 font-semibold">
                  Note
                </th>
              </tr>
            </thead>
            <tbody>
              {list.isPending &&
                [0, 1, 2].map((i) => (
                  <tr key={i} className="border-t">
                    <td colSpan={4} className="py-3">
                      <Skeleton className="h-9 w-full" />
                    </td>
                  </tr>
                ))}
              {rows.map((r) => (
                <tr key={r.contact_id} className="border-t align-top">
                  <td className="py-3 pr-2">
                    <span className="flex flex-col">
                      <span className="font-bold">{r.name || r.phone}</span>
                      {r.name && (
                        <span className="text-[13px] text-muted-foreground">{r.phone}</span>
                      )}
                    </span>
                  </td>
                  <td className="p-2 py-3">
                    <span className="flex flex-wrap items-center gap-1.5">
                      <Outcome r={r} />
                      {r.replied && <Badge variant="success">Replied</Badge>}
                    </span>
                  </td>
                  <td className="p-2 py-3 whitespace-nowrap text-muted-foreground">
                    {formatWhen(r.status_at ?? r.queued_at)}
                  </td>
                  <td className="py-3 pl-2 text-[13px] text-muted-foreground">
                    {r.error ?? (r.reason ? (FAILURE_LABEL[r.reason] ?? r.reason) : '')}
                  </td>
                </tr>
              ))}
              {!list.isPending && rows.length === 0 && (
                <tr className="border-t">
                  <td colSpan={4} className="py-10 text-center text-muted-foreground">
                    {filter ? 'Nobody here yet.' : 'Recipients appear once sending starts.'}
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      )}
      {list.hasNextPage && (
        <Button
          variant="outline"
          size="sm"
          className="self-center"
          disabled={list.isFetchingNextPage}
          onClick={() => list.fetchNextPage()}
        >
          {list.isFetchingNextPage ? 'Loading…' : 'Load more'}
        </Button>
      )}
    </section>
  )
}
