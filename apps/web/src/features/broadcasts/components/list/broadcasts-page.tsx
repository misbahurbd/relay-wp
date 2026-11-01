import { useInfiniteQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { PlusIcon, SearchIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { useAccess } from '#/features/team'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { broadcastsQueryOptions } from '../../api/broadcasts.queries'
import { STATUS_TABS } from '../../schemas/broadcasts.schema'
import type { BroadcastFilter } from '../../types'
import { BroadcastTable } from './broadcast-table'

type Props = {
  workspaceId: string
  filter: BroadcastFilter
  onFilter: (next: BroadcastFilter) => void
}

const newLink =
  'inline-flex h-11 items-center gap-2 rounded-lg bg-primary px-[18px] font-bold text-primary-foreground no-underline hover:bg-primary/90 hover:text-primary-foreground'

export function BroadcastsPage({ workspaceId, filter, onFilter }: Props) {
  const { can } = useAccess(workspaceId)
  const list = useInfiniteQuery(broadcastsQueryOptions(workspaceId, filter))
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  const filtered = !!(filter.status || filter.q)
  const [q, setQ] = useState(filter.q ?? '')

  // Search as the user types, a moment after they stop.
  useEffect(() => {
    const t = setTimeout(() => {
      if ((filter.q ?? '') !== q.trim()) onFilter({ ...filter, q: q.trim() || undefined })
    }, 300)
    return () => clearTimeout(t)
  }, [q, filter, onFilter])

  const empty = filtered ? (
    <div className="flex flex-col items-center gap-3">
      <p className="text-muted-foreground">No broadcasts match these filters.</p>
      <Button
        variant="outline"
        size="sm"
        onClick={() => {
          setQ('')
          onFilter({})
        }}
      >
        Clear filters
      </Button>
    </div>
  ) : (
    <div className="flex flex-col items-center gap-2">
      <p className="font-bold">No broadcasts yet</p>
      <p className="max-w-prose text-muted-foreground">
        Send an approved template to everyone who opted in, or only to customers with a tag.
      </p>
      {can('broadcasts.create') && (
        <Link to="/broadcasts/new" className={cn(newLink, 'mt-1 h-10 px-4')}>
          <PlusIcon className="size-4" aria-hidden="true" />
          New broadcast
        </Link>
      )}
    </div>
  )

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Broadcasts</h1>
          <p className="max-w-prose text-[15px] text-muted-foreground">
            Send one approved template to many customers at once, now or later. Only contacts who
            opted in get it.
          </p>
        </div>
        {can('broadcasts.create') && (
          <Link to="/broadcasts/new" className={newLink}>
            <PlusIcon className="size-[18px]" aria-hidden="true" />
            New broadcast
          </Link>
        )}
      </div>

      <section
        aria-label="Broadcasts"
        className="flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:p-6"
      >
        <div className="flex flex-wrap items-center justify-between gap-3">
          <div role="tablist" aria-label="Status" className="flex flex-wrap gap-1.5">
            {STATUS_TABS.map((t) => {
              const active = filter.status === t.value
              return (
                <Button
                  key={t.label}
                  role="tab"
                  aria-selected={active}
                  variant={active ? 'secondary' : 'ghost'}
                  size="sm"
                  onClick={() => onFilter({ ...filter, status: t.value })}
                >
                  {t.label}
                </Button>
              )
            })}
          </div>
          <div className="relative w-full sm:w-64">
            <SearchIcon
              className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
              aria-hidden="true"
            />
            <Input
              type="search"
              aria-label="Search broadcasts"
              placeholder="Search by name"
              className="pl-9"
              value={q}
              onChange={(e) => setQ(e.target.value)}
            />
          </div>
        </div>

        {list.isError ? (
          <Alert variant="danger">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              {errorMessage(list.error, 'Couldn’t load broadcasts.')}
              <Button variant="outline" size="sm" onClick={() => list.refetch()}>
                Try again
              </Button>
            </AlertDescription>
          </Alert>
        ) : (
          <BroadcastTable items={items} loading={list.isPending} empty={empty} />
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
    </main>
  )
}
