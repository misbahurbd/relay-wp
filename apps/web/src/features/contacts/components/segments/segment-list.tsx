import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'
import type { Segment } from '../../types'
import { formatNumber } from '../kit/format'

/** Saved segments as cards, each with who it reaches and what you can do with it. */
export function SegmentList({
  segments,
  loading,
  selectedId,
  canEdit,
  canBroadcast,
  deleting,
  onSelect,
  onDuplicate,
  onDelete,
}: {
  segments: Segment[]
  loading: boolean
  selectedId?: string
  canEdit: boolean
  canBroadcast: boolean
  deleting: boolean
  onSelect: (s: Segment) => void
  onDuplicate: (s: Segment) => void
  onDelete: (s: Segment) => void
}) {
  const [confirm, setConfirm] = useState<string>()
  return (
    <section aria-label="Saved segments" className="flex flex-col gap-3">
      {loading && [0, 1, 2].map((i) => <Skeleton key={i} className="h-36 w-full rounded-2xl" />)}
      {!loading && segments.length === 0 && (
        <p className="rounded-2xl border bg-card p-5 text-[13px] text-muted-foreground">
          No segments yet. Start from a quick start on the right, or build your own rules.
        </p>
      )}
      {segments.map((s) => (
        <article
          key={s.id}
          className={cn(
            'flex flex-col gap-2 rounded-2xl border bg-card p-4 shadow-card',
            s.id === selectedId && 'border-primary',
          )}
        >
          <div className="flex items-start justify-between gap-2">
            <Button
              variant="unstyled"
              type="button"
              aria-pressed={s.id === selectedId}
              className="h-auto p-0 text-left text-[15px] font-bold hover:underline"
              onClick={() => onSelect(s)}
            >
              {s.name}
            </Button>
            <Badge variant="info">Auto-updates</Badge>
          </div>
          {s.description && <p className="text-[13px] text-muted-foreground">{s.description}</p>}
          <div className="flex items-baseline gap-1.5">
            <span className="text-2xl font-extrabold tabular-nums">
              {formatNumber(s.count.opted_in)}
            </span>
            <span className="text-[13px] text-muted-foreground">
              opted in · {formatNumber(s.count.matched)} match
            </span>
          </div>
          {confirm === s.id ? (
            <div
              role="alert"
              className="flex flex-col gap-2 rounded-xl bg-destructive-soft p-3 text-[13px]"
            >
              <span>
                Delete this segment? Contacts stay; scheduled broadcasts using it will need a new
                audience.
              </span>
              <div className="flex gap-2">
                <Button size="sm" variant="outline" onClick={() => setConfirm(undefined)}>
                  Keep
                </Button>
                <Button
                  size="sm"
                  variant="destructive"
                  disabled={deleting}
                  onClick={() => onDelete(s)}
                >
                  Delete
                </Button>
              </div>
            </div>
          ) : (
            <div className="flex flex-wrap gap-1.5">
              <Button size="sm" variant="outline" onClick={() => onSelect(s)}>
                {canEdit ? 'Edit' : 'View'}
              </Button>
              {canEdit && (
                <Button size="sm" variant="outline" onClick={() => onDuplicate(s)}>
                  Duplicate
                </Button>
              )}
              {canBroadcast && (
                <Button size="sm" variant="outline" asChild>
                  <Link to="/broadcasts/new" search={{ segment: s.id }}>
                    Use in broadcast
                  </Link>
                </Button>
              )}
              {canEdit && (
                <Button
                  size="sm"
                  variant="ghost"
                  className="text-destructive"
                  onClick={() => setConfirm(s.id)}
                >
                  Delete
                </Button>
              )}
            </div>
          )}
        </article>
      ))}
      <aside className="flex flex-col gap-2 rounded-2xl bg-muted p-4 text-[13px]">
        <span className="font-bold">Segments, tags or lists?</span>
        <span>
          <b>Segments</b> update on their own: anyone who matches the rules joins, anyone who stops
          matching leaves.
        </span>
        <span>
          <b>Tags</b> are labels you or an automation add, like VIP or Wholesale.
        </span>
        <span>
          <b>Uploaded lists</b> come from a file you upload for a broadcast; their contacts get a
          list tag.
        </span>
      </aside>
    </section>
  )
}
