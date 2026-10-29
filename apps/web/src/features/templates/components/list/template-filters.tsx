import { SearchIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Input } from '#/components/ui/input'
import { cn } from '#/lib/utils'
import { CATEGORIES, STATUS_TABS } from '../../schemas/templates.schema'
import type { TemplateCounts, TemplateFilter } from '../../types'

const COUNT_KEY = { all: 'all', approved: 'approved', pending: 'pending', rejected: 'rejected' }

/** Status segments with counts, category chips and a search box (filters live in the URL). */
export function TemplateFilters({
  filter,
  counts,
  onFilter,
}: {
  filter: TemplateFilter
  counts?: TemplateCounts
  onFilter: (next: TemplateFilter) => void
}) {
  const [q, setQ] = useState(filter.q ?? '')

  // Follow the URL when it changes from outside (Back, "Clear filters").
  useEffect(() => {
    setQ((prev) => (prev.trim() === (filter.q ?? '') ? prev : (filter.q ?? '')))
  }, [filter.q])

  // Search as the user types, without a request per keystroke.
  useEffect(() => {
    const next = q.trim() || undefined
    if (next === filter.q) return
    const t = setTimeout(() => onFilter({ ...filter, q: next }), 300)
    return () => clearTimeout(t)
  }, [q, filter, onFilter])

  return (
    <div className="flex flex-col gap-3 lg:flex-row lg:flex-wrap lg:items-center">
      <fieldset className="flex w-fit max-w-full flex-wrap gap-1 rounded-lg bg-muted p-1">
        <legend className="sr-only">Filter by status</legend>
        {STATUS_TABS.map((t) => {
          const on = filter.status === t.value
          const count = counts?.[COUNT_KEY[t.value ?? 'all'] as keyof TemplateCounts]
          return (
            <button
              key={t.label}
              type="button"
              aria-pressed={on}
              onClick={() => onFilter({ ...filter, status: t.value })}
              className={cn(
                'inline-flex h-9 items-center gap-1.5 rounded-md px-3 text-sm font-bold',
                on
                  ? 'bg-card text-foreground shadow-card'
                  : 'text-muted-foreground hover:text-foreground',
              )}
            >
              {t.label}
              {count !== undefined && (
                <span className="rounded-full bg-secondary px-1.5 text-xs tabular-nums">
                  {count}
                </span>
              )}
            </button>
          )
        })}
      </fieldset>

      <fieldset className="flex min-w-0 flex-wrap gap-2">
        <legend className="sr-only">Filter by category</legend>
        {[{ value: undefined, label: 'All categories' }, ...CATEGORIES].map((c) => {
          const on = filter.category === c.value
          return (
            <button
              key={c.label}
              type="button"
              aria-pressed={on}
              onClick={() => onFilter({ ...filter, category: c.value })}
              className={cn(
                'h-9 rounded-full border px-3.5 text-[13px] font-bold',
                on
                  ? 'border-primary bg-brand-soft text-brand-text'
                  : 'border-border bg-card text-foreground hover:bg-muted',
              )}
            >
              {c.label}
            </button>
          )
        })}
      </fieldset>

      <div className="relative w-full lg:ml-auto lg:w-64">
        <label htmlFor="template-search" className="sr-only">
          Search templates
        </label>
        <SearchIcon
          className="pointer-events-none absolute top-1/2 left-3 size-4 -translate-y-1/2 text-muted-foreground"
          aria-hidden="true"
        />
        <Input
          id="template-search"
          type="search"
          placeholder="Search templates"
          className="h-10 pl-9"
          value={q}
          onChange={(e) => setQ(e.target.value)}
        />
      </div>
    </div>
  )
}
