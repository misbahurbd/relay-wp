import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage, isApiError } from '#/lib/api'
import { broadcastQueryOptions } from '../api/broadcasts.queries'
import { BroadcastEditor } from './editor/broadcast-editor'
import { BroadcastReport } from './report/broadcast-report'

/** One broadcast: the editor while it is a draft or scheduled, the report once it started. */
export function BroadcastDetail({
  workspaceId,
  broadcastId,
}: {
  workspaceId: string
  broadcastId: string
}) {
  const query = useQuery(broadcastQueryOptions(workspaceId, broadcastId))
  if (query.isPending) {
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
        <Skeleton className="h-10 w-72" />
        <Skeleton className="h-64 w-full" />
      </main>
    )
  }
  if (query.isError) {
    const missing = isApiError(query.error) && query.error.code === 'broadcast_not_found'
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col items-start gap-3 p-4 sm:p-7">
        <Link to="/broadcasts" className="text-[13px] font-bold text-muted-foreground">
          ← Broadcasts
        </Link>
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">
          {missing ? 'Broadcast not found' : 'Couldn’t load this broadcast'}
        </h1>
        <p className="text-muted-foreground">
          {missing
            ? 'It may have been deleted, or it belongs to another workspace.'
            : errorMessage(query.error)}
        </p>
        {!missing && (
          <Button variant="outline" onClick={() => query.refetch()}>
            Try again
          </Button>
        )}
      </main>
    )
  }
  const b = query.data
  if (b.status === 'draft' || b.status === 'scheduled') {
    // Keyed by status, so sending or scheduling starts the next view from fresh data.
    return <BroadcastEditor key={`${b.id}-${b.status}`} workspaceId={workspaceId} broadcast={b} />
  }
  return <BroadcastReport workspaceId={workspaceId} broadcast={b} />
}
