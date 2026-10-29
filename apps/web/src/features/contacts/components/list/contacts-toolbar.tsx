import { SearchIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { cn } from '#/lib/utils'
import { useDebouncedValue } from '../../hooks/use-debounced-value'
import { LAST_MESSAGE_LABEL, LAST_MESSAGES, OPT_IN_FILTERS } from '../../schemas/contacts.schema'
import type { ContactFilter, LastMessage, OptIn, Tag } from '../../types'

const ANY = 'any'

/** Search, opt-in chips and the Tag and Last message selects. Every change resets paging. */
export function ContactsToolbar({
  filter,
  tags,
  onChange,
}: {
  filter: ContactFilter
  tags: Tag[]
  onChange: (next: ContactFilter) => void
}) {
  const [q, setQ] = useState(filter.q ?? '')
  const debounced = useDebouncedValue(q.trim(), 300)

  // biome-ignore lint/correctness/useExhaustiveDependencies: only the typed text drives this
  useEffect(() => {
    if (debounced !== (filter.q ?? '')) onChange({ ...filter, q: debounced || undefined })
  }, [debounced])

  // Back/forward or "Clear filters" changes q from outside.
  useEffect(() => {
    setQ((cur) => (cur.trim() === (filter.q ?? '') ? cur : (filter.q ?? '')))
  }, [filter.q])

  return (
    <div className="flex flex-wrap items-center gap-2.5">
      <label className="flex h-10 max-w-[380px] min-w-0 flex-[1_1_260px] items-center gap-2 rounded-[10px] border border-input bg-card px-3 text-muted-foreground focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring">
        <SearchIcon className="size-4 flex-none" aria-hidden="true" />
        <span className="sr-only">Search contacts</span>
        <input
          type="search"
          value={q}
          onChange={(e) => setQ(e.target.value)}
          placeholder="Search by name, phone or email"
          className="min-w-0 flex-1 bg-transparent text-base text-foreground outline-none placeholder:text-muted-foreground md:text-[13px]"
        />
      </label>

      <fieldset className="inline-flex rounded-[10px] border bg-card p-[3px]">
        <legend className="sr-only">Opt-in status</legend>
        {OPT_IN_FILTERS.map((f) => {
          const on = filter.opt_in === f.value
          return (
            <button
              key={f.label}
              type="button"
              aria-pressed={on}
              onClick={() => onChange({ ...filter, opt_in: f.value as OptIn | undefined })}
              className={cn(
                'h-8 rounded-lg px-3 text-[13px] font-bold',
                on
                  ? 'bg-brand-soft text-brand-text'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {f.label}
            </button>
          )
        })}
      </fieldset>

      <Select
        value={filter.tag_id ?? ANY}
        onValueChange={(v) => onChange({ ...filter, tag_id: v === ANY ? undefined : v })}
      >
        <SelectTrigger size="sm" aria-label="Tag" className="h-10 min-w-[130px] rounded-[10px]">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ANY}>All tags</SelectItem>
          {tags.map((t) => (
            <SelectItem key={t.id} value={t.id}>
              {t.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>

      <Select
        value={filter.last_message ?? ANY}
        onValueChange={(v) =>
          onChange({ ...filter, last_message: v === ANY ? undefined : (v as LastMessage) })
        }
      >
        <SelectTrigger
          size="sm"
          aria-label="Last message"
          className="h-10 min-w-[140px] rounded-[10px]"
        >
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={ANY}>Any time</SelectItem>
          {LAST_MESSAGES.map((v) => (
            <SelectItem key={v} value={v}>
              {LAST_MESSAGE_LABEL[v]}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </div>
  )
}
