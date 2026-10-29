import { CheckIcon, Loader2Icon, PlusIcon, SearchIcon } from 'lucide-react'
import { type ReactNode, useId, useState } from 'react'
import { Popover, PopoverContent, PopoverTrigger } from '#/components/ui/popover'
import { cn } from '#/lib/utils'
import { tagNameSchema } from '../../schemas/contacts.schema'
import type { Tag, TagRef } from '../../types'

/**
 * A searchable tag list in a popover. Picking calls onPick; when nothing matches, the typed
 * name can be created (if onCreate is given). Single-pick closes the popover.
 */
export function TagPicker({
  tags,
  selectedIds = [],
  onPick,
  onCreate,
  multi,
  trigger,
  label = 'Tags',
  align = 'start',
}: {
  tags: Tag[]
  selectedIds?: string[]
  onPick: (tag: TagRef) => void
  onCreate?: (name: string) => Promise<TagRef>
  multi?: boolean
  trigger: ReactNode
  label?: string
  align?: 'start' | 'end'
}) {
  const [open, setOpen] = useState(false)
  const [q, setQ] = useState('')
  const [creating, setCreating] = useState(false)
  const [error, setError] = useState<string>()
  const inputId = useId()
  const term = q.trim().toLowerCase()
  const shown = tags.filter((t) => t.name.toLowerCase().includes(term))
  const exact = tags.some((t) => t.name.toLowerCase() === term)

  const pick = (tag: TagRef) => {
    onPick(tag)
    if (!multi) setOpen(false)
    setQ('')
  }

  const create = async () => {
    if (!onCreate) return
    const parsed = tagNameSchema.safeParse(q)
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message)
      return
    }
    setCreating(true)
    setError(undefined)
    try {
      pick(await onCreate(parsed.data))
    } catch {
      setError('Couldn’t create the tag. Try again.')
    } finally {
      setCreating(false)
    }
  }

  return (
    <Popover
      open={open}
      onOpenChange={(o) => {
        setOpen(o)
        if (!o) {
          setQ('')
          setError(undefined)
        }
      }}
    >
      <PopoverTrigger asChild>{trigger}</PopoverTrigger>
      <PopoverContent align={align} className="flex w-64 flex-col gap-2 p-2">
        <label
          htmlFor={inputId}
          className="flex h-9 items-center gap-2 rounded-md border border-input bg-card px-2.5 text-muted-foreground focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring"
        >
          <SearchIcon className="size-4" aria-hidden="true" />
          <span className="sr-only">{`Search ${label.toLowerCase()}`}</span>
          <input
            id={inputId}
            value={q}
            onChange={(e) => {
              setQ(e.target.value)
              setError(undefined)
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter') {
                e.preventDefault()
                const first = shown[0]
                if (first && (exact || !onCreate)) pick(first)
                else if (term && onCreate) void create()
              }
            }}
            placeholder={onCreate ? 'Find or create a tag' : 'Find a tag'}
            className="min-w-0 flex-1 bg-transparent text-base text-foreground outline-none placeholder:text-muted-foreground md:text-sm"
          />
        </label>
        <ul className="flex max-h-60 flex-col overflow-y-auto" aria-label={label}>
          {shown.map((t) => {
            const on = selectedIds.includes(t.id)
            return (
              <li key={t.id}>
                <button
                  type="button"
                  aria-pressed={multi ? on : undefined}
                  onClick={() => pick({ id: t.id, name: t.name })}
                  className="flex min-h-9 w-full items-center gap-2 rounded-md px-2 text-left text-sm hover:bg-accent"
                >
                  <CheckIcon
                    aria-hidden="true"
                    className={cn('size-4 text-brand-text', on ? 'opacity-100' : 'opacity-0')}
                  />
                  <span className="flex-1 truncate">{t.name}</span>
                  <span className="text-xs text-muted-foreground tabular-nums">
                    {t.contact_count.toLocaleString()}
                  </span>
                </button>
              </li>
            )
          })}
          {shown.length === 0 && !onCreate && (
            <li className="px-2 py-2 text-sm text-muted-foreground">No tags match.</li>
          )}
        </ul>
        {onCreate && term && !exact && (
          <button
            type="button"
            onClick={() => void create()}
            disabled={creating}
            className="flex min-h-9 items-center gap-2 rounded-md px-2 text-left text-sm font-semibold text-brand-text hover:bg-accent disabled:opacity-60"
          >
            {creating ? (
              <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
            ) : (
              <PlusIcon className="size-4" aria-hidden="true" />
            )}
            <span className="truncate">Create “{q.trim()}”</span>
          </button>
        )}
        {error && (
          <p role="alert" className="px-2 text-[13px] text-destructive">
            {error}
          </p>
        )}
      </PopoverContent>
    </Popover>
  )
}
