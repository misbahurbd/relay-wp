import type { ReactNode } from 'react'
import { Checkbox } from '#/components/ui/checkbox'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'
import { SOURCE_LABEL } from '../../schemas/contacts.schema'
import type { Contact } from '../../types'
import { ContactAvatar } from '../kit/avatar'
import { lastMessageLabel } from '../kit/format'
import { OptInPill } from '../kit/opt-in-pill'
import { TagChip } from '../kit/tag-chip'

const th = 'px-2.5 py-3 text-left text-xs font-bold text-muted-foreground'
const td = 'px-2.5 py-2.5 align-middle'
const MAX_TAGS = 3

/**
 * The contacts table. Each row's name is a button that opens the panel (Enter or Space);
 * a click anywhere else on the row does the same, except on the checkbox.
 */
export function ContactsTable({
  rows,
  loading,
  selectable,
  selectedIds,
  openId,
  onToggle,
  onToggleAll,
  onOpen,
  empty,
  footer,
}: {
  rows: Contact[] | undefined
  loading: boolean
  selectable: boolean
  selectedIds: Set<string>
  openId: string | null
  onToggle: (c: Contact) => void
  onToggleAll: (checked: boolean) => void
  onOpen: (id: string) => void
  empty: ReactNode
  footer: ReactNode
}) {
  const count = rows?.filter((r) => selectedIds.has(r.id)).length ?? 0
  const all = !!rows?.length && count === rows.length
  const cols = selectable ? 7 : 6

  return (
    <div className="overflow-hidden rounded-xl border bg-card shadow-card">
      <div className="overflow-x-auto">
        <table className="w-full min-w-[860px] border-collapse text-sm" aria-busy={loading}>
          <thead>
            <tr className="bg-muted">
              {selectable && (
                <th scope="col" className="w-11 py-3 pl-4">
                  <Checkbox
                    aria-label="Select all contacts on this page"
                    checked={all ? true : count > 0 ? 'indeterminate' : false}
                    disabled={!rows?.length}
                    onCheckedChange={(v) => onToggleAll(v === true)}
                  />
                </th>
              )}
              <th scope="col" className={cn(th, !selectable && 'pl-4')}>
                Name
              </th>
              <th scope="col" className={th}>
                Phone
              </th>
              <th scope="col" className={th}>
                Tags
              </th>
              <th scope="col" className={th}>
                Opt-in status
              </th>
              <th scope="col" className={th}>
                Last message
              </th>
              <th scope="col" className={cn(th, 'pr-4')}>
                Source
              </th>
            </tr>
          </thead>
          <tbody>
            {loading && !rows ? (
              <SkeletonRows selectable={selectable} />
            ) : !rows?.length ? (
              <tr>
                <td colSpan={cols} className="p-0">
                  {empty}
                </td>
              </tr>
            ) : (
              rows.map((c) => {
                const checked = selectedIds.has(c.id)
                const open = c.id === openId
                return (
                  <tr
                    key={c.id}
                    onClick={(e) => {
                      if ((e.target as HTMLElement).closest('button,[role=checkbox],a')) return
                      onOpen(c.id)
                    }}
                    className={cn(
                      'cursor-pointer border-t',
                      open ? 'bg-brand-soft' : checked ? 'bg-muted' : 'hover:bg-muted',
                    )}
                  >
                    {selectable && (
                      <td className="py-2.5 pl-4">
                        <Checkbox
                          aria-label={`Select ${c.name}`}
                          checked={checked}
                          onCheckedChange={() => onToggle(c)}
                        />
                      </td>
                    )}
                    <td className={cn(td, !selectable && 'pl-4')}>
                      <button
                        type="button"
                        onClick={() => onOpen(c.id)}
                        aria-current={open ? 'true' : undefined}
                        className="group flex items-center gap-2.5 rounded-md text-left"
                      >
                        <ContactAvatar name={c.name} seed={c.id} />
                        <span className="flex min-w-0 flex-col leading-tight">
                          <span className="font-bold group-hover:underline">{c.name}</span>
                          {c.email && (
                            <span className="truncate text-xs text-muted-foreground">
                              {c.email}
                            </span>
                          )}
                        </span>
                      </button>
                    </td>
                    <td className={cn(td, 'whitespace-nowrap tabular-nums')}>{c.phone_display}</td>
                    <td className={td}>
                      <div className="flex flex-wrap gap-1">
                        {c.tags.slice(0, MAX_TAGS).map((t) => (
                          <TagChip key={t.id} name={t.name} />
                        ))}
                        {c.tags.length > MAX_TAGS && (
                          <span className="px-1 text-xs font-semibold text-muted-foreground">
                            +{c.tags.length - MAX_TAGS}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className={td}>
                      <OptInPill value={c.opt_in} />
                    </td>
                    <td className={cn(td, 'whitespace-nowrap text-muted-foreground')}>
                      {lastMessageLabel(c.last_message_at)}
                    </td>
                    <td className={cn(td, 'pr-4 whitespace-nowrap text-muted-foreground')}>
                      {SOURCE_LABEL[c.source]}
                    </td>
                  </tr>
                )
              })
            )}
          </tbody>
        </table>
      </div>
      {footer}
    </div>
  )
}

function SkeletonRows({ selectable }: { selectable: boolean }) {
  return Array.from({ length: 8 }, (_, i) => (
    // biome-ignore lint/suspicious/noArrayIndexKey: static placeholders
    <tr key={i} className="border-t">
      {selectable && (
        <td className="py-2.5 pl-4">
          <Skeleton className="size-4.5" />
        </td>
      )}
      <td className={td}>
        <div className="flex items-center gap-2.5">
          <Skeleton className="size-9 rounded-full" />
          <div className="flex flex-col gap-1.5">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-3 w-40" />
          </div>
        </div>
      </td>
      <td className={td}>
        <Skeleton className="h-4 w-28" />
      </td>
      <td className={td}>
        <Skeleton className="h-5 w-24" />
      </td>
      <td className={td}>
        <Skeleton className="h-5 w-20 rounded-full" />
      </td>
      <td className={td}>
        <Skeleton className="h-4 w-24" />
      </td>
      <td className={cn(td, 'pr-4')}>
        <Skeleton className="h-4 w-16" />
      </td>
    </tr>
  ))
}
