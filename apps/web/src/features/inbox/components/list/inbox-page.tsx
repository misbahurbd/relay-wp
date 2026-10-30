import { InboxIcon } from 'lucide-react'
import { useCallback, useRef } from 'react'
import { cn } from '#/lib/utils'
import type { ListFilter } from '../../api/inbox.keys'
import type { InboxSearch } from '../../schemas/inbox.schema'
import { ThreadView } from '../thread/thread-view'
import { ConversationList } from './conversation-list'

/**
 * The Team Inbox: chats on the left, the open chat in the middle, customer details on the right.
 * Below 768px it shows one column at a time (the list, or the chat with a back button).
 */
export function InboxPage({
  workspaceId,
  me,
  search,
  onSearchChange,
}: {
  workspaceId: string
  me: { id: string; name: string }
  search: InboxSearch
  onSearchChange: (next: InboxSearch) => void
}) {
  const filter: ListFilter = {
    tab: search.tab ?? 'open',
    q: search.q,
    sort: search.sort ?? 'newest',
  }
  const selected = search.c

  // Stable callbacks: the list's search box calls onFilterChange from an effect.
  const searchRef = useRef(search)
  searchRef.current = search
  const setFilter = useCallback(
    (next: ListFilter) =>
      onSearchChange({
        ...searchRef.current,
        tab: next.tab === 'open' ? undefined : next.tab,
        q: next.q || undefined,
        sort: next.sort === 'newest' ? undefined : next.sort,
      }),
    [onSearchChange],
  )
  const select = (c: string | undefined) => onSearchChange({ ...searchRef.current, c })

  return (
    <div className="flex h-[calc(100dvh-4rem)] min-h-0 overflow-hidden">
      <ConversationList
        workspaceId={workspaceId}
        meId={me.id}
        filter={filter}
        selectedId={selected}
        onSelect={select}
        onFilterChange={setFilter}
        className={cn('w-full md:w-[320px] md:flex-none lg:w-[340px]', selected && 'max-md:hidden')}
      />
      {selected ? (
        <ThreadView
          key={selected}
          workspaceId={workspaceId}
          conversationId={selected}
          me={me}
          onBack={() => select(undefined)}
        />
      ) : (
        <div className="hidden flex-1 flex-col items-center justify-center gap-2 bg-chat p-6 text-center md:flex">
          <InboxIcon className="size-8 text-muted-foreground" aria-hidden="true" />
          <p className="font-bold">Pick a chat to start</p>
          <p className="max-w-xs text-[13px] text-muted-foreground">
            New WhatsApp messages appear on the left as they arrive.
          </p>
        </div>
      )}
    </div>
  )
}
