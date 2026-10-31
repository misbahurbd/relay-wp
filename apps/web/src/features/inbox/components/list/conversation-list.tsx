import { useInfiniteQuery } from '@tanstack/react-query'
import { Loader2Icon, SearchIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { NativeSelect } from '#/components/ui/native-select'
import { Skeleton } from '#/components/ui/skeleton'
import { useDebouncedValue } from '#/hooks/use-debounced-value'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import type { ListFilter } from '../../api/inbox.keys'
import { conversationsQueryOptions } from '../../api/inbox.queries'
import { useLiveConnected } from '../../hooks/use-live-status'
import { useNow } from '../../hooks/use-now'
import { INBOX_TABS, TAB_LABEL } from '../../schemas/inbox.schema'
import type { InboxSort, InboxTab } from '../../types'
import { ConversationRow } from './conversation-row'

/** The left column (F2): sort, search, tabs with counts, and the chats, loaded as you scroll. */
export function ConversationList({
  workspaceId,
  meId,
  filter,
  selectedId,
  onSelect,
  onFilterChange,
  className,
}: {
  workspaceId: string
  meId: string
  filter: ListFilter
  selectedId: string | undefined
  onSelect: (id: string) => void
  onFilterChange: (next: ListFilter) => void
  className?: string
}) {
  const live = useLiveConnected()
  const now = useNow()
  const query = useInfiniteQuery(conversationsQueryOptions(workspaceId, filter, !live))
  const counts = query.data?.pages[0]?.counts
  const items = query.data?.pages.flatMap((p) => p.items) ?? []

  // The search box updates the URL once typing pauses.
  const [q, setQ] = useState(filter.q ?? '')
  const debounced = useDebouncedValue(q.trim(), 300)
  const filterRef = useRef(filter)
  filterRef.current = filter
  useEffect(() => {
    if (debounced !== (filterRef.current.q ?? '')) {
      onFilterChange({ ...filterRef.current, q: debounced || undefined })
    }
  }, [debounced, onFilterChange])

  // Load the next page when the end of the list scrolls into view.
  const sentinel = useRef<HTMLDivElement>(null)
  const { hasNextPage, isFetchingNextPage, fetchNextPage } = query
  useEffect(() => {
    const el = sentinel.current
    if (!el || !hasNextPage) return
    const io = new IntersectionObserver((entries) => {
      if (entries[0]?.isIntersecting && !isFetchingNextPage) void fetchNextPage()
    })
    io.observe(el)
    return () => io.disconnect()
  }, [hasNextPage, isFetchingNextPage, fetchNextPage])

  return (
    <section
      aria-label="Conversations"
      className={cn('flex min-h-0 flex-col border-r bg-card', className)}
    >
      <div className="flex flex-col gap-3 px-4 pt-4 pb-3">
        <div className="flex items-center justify-between gap-2">
          <h1 className="text-xl font-extrabold tracking-tight">Inbox</h1>
          <label className="inline-flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
            Sort
            <NativeSelect
              unstyled
              value={filter.sort}
              onChange={(e) => onFilterChange({ ...filter, sort: e.target.value as InboxSort })}
              className="h-9 rounded-md border border-input bg-card px-2 text-xs font-bold text-foreground"
            >
              <option value="newest">Newest</option>
              <option value="waiting">Waiting longest</option>
            </NativeSelect>
          </label>
        </div>
        <label className="flex h-10 items-center gap-2 rounded-md border border-input bg-muted px-3 text-muted-foreground">
          <SearchIcon className="size-4" aria-hidden="true" />
          <span className="sr-only">Search chats</span>
          <Input
            unstyled
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search name, phone or message"
            className="min-w-0 flex-1 bg-transparent text-base text-foreground outline-none md:text-[13px]"
          />
        </label>
        <div role="tablist" aria-label="Chat filters" className="flex flex-wrap gap-1.5">
          {INBOX_TABS.map((tab) => (
            <TabButton
              key={tab}
              tab={tab}
              count={counts?.[tab]}
              selected={filter.tab === tab}
              onSelect={() => onFilterChange({ ...filter, tab })}
            />
          ))}
        </div>
      </div>

      <div className="flex min-h-0 flex-1 flex-col gap-0.5 overflow-y-auto px-2 pb-3">
        {query.isPending ? (
          <ListSkeleton />
        ) : query.isError ? (
          <Alert variant="danger" className="m-2">
            <AlertDescription>
              {errorMessage(query.error, 'We couldn’t load your chats.')}{' '}
              <Button
                variant="unstyled"
                type="button"
                className="font-bold underline"
                onClick={() => query.refetch()}
              >
                Try again
              </Button>
            </AlertDescription>
          </Alert>
        ) : items.length === 0 ? (
          <p className="mx-3 my-6 text-center text-muted-foreground">No chats match this view.</p>
        ) : (
          items.map((c) => (
            <ConversationRow
              key={c.id}
              conversation={c}
              meId={meId}
              now={now}
              selected={c.id === selectedId}
              onSelect={() => onSelect(c.id)}
            />
          ))
        )}
        <div ref={sentinel} aria-hidden="true" className="h-px" />
        {isFetchingNextPage && (
          <Loader2Icon
            className="mx-auto my-2 size-5 animate-spin text-muted-foreground"
            aria-label="Loading more chats"
          />
        )}
      </div>
    </section>
  )
}

function TabButton({
  tab,
  count,
  selected,
  onSelect,
}: {
  tab: InboxTab
  count: number | undefined
  selected: boolean
  onSelect: () => void
}) {
  return (
    <Button
      variant="unstyled"
      type="button"
      role="tab"
      aria-selected={selected}
      onClick={onSelect}
      className={cn(
        'inline-flex h-9 items-center gap-1.5 rounded-full border px-3 text-[13px] font-bold',
        selected
          ? 'border-primary bg-primary text-primary-foreground'
          : 'border-border bg-card hover:bg-muted',
      )}
    >
      {TAB_LABEL[tab]}
      {count !== undefined && <span className="text-xs opacity-85">{count}</span>}
    </Button>
  )
}

function ListSkeleton() {
  return (
    <div role="status" aria-label="Loading chats" className="flex flex-col gap-1">
      {[0, 1, 2, 3, 4, 5].map((i) => (
        <div key={i} className="flex gap-3 px-2.5 py-3">
          <Skeleton className="size-11 rounded-full" />
          <div className="flex flex-1 flex-col gap-2">
            <Skeleton className="h-4 w-2/3" />
            <Skeleton className="h-3 w-full" />
          </div>
        </div>
      ))}
    </div>
  )
}
