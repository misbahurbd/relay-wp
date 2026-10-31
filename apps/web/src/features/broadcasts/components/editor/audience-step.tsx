import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CheckIcon, PlusIcon, UsersIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '#/components/ui/button'
import { Checkbox } from '#/components/ui/checkbox'
import { Label } from '#/components/ui/label'
import { Skeleton } from '#/components/ui/skeleton'
import { contactTagsQueryOptions, segmentsQueryOptions } from '#/features/contacts'
import { cn } from '#/lib/utils'
import type { Audience } from '../../types'
import { formatCount } from '../kit/format'

/** A pressed-or-not pill, like the design's tag chips. */
function Chip({
  on,
  onToggle,
  disabled,
  children,
}: {
  on: boolean
  onToggle: () => void
  disabled?: boolean
  children: ReactNode
}) {
  return (
    <Button
      variant="unstyled"
      type="button"
      aria-pressed={on}
      disabled={disabled}
      onClick={onToggle}
      className={cn(
        'inline-flex h-10 items-center gap-2 rounded-full border px-3.5 text-[13px] font-bold',
        on
          ? 'border-primary bg-primary text-primary-foreground'
          : 'border-border bg-card text-foreground hover:border-primary/60',
      )}
    >
      {on && <CheckIcon className="size-3.5" strokeWidth={3} aria-hidden="true" />}
      {children}
    </Button>
  )
}

export function StepHeading({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="flex flex-col gap-1">
      <h2 className="text-[19px] font-extrabold">{title}</h2>
      <p className="text-muted-foreground">{children}</p>
    </div>
  )
}

/** Step 1: everyone, or tags (more sources join here), plus the recent-marketing guard. */
export function AudienceStep({
  workspaceId,
  audience,
  readOnly,
  error,
  extra,
  hideTagIds = [],
  onChange,
}: {
  workspaceId: string
  audience: Audience
  readOnly: boolean
  error?: string
  /** Segments and uploaded lists, when available. */
  extra?: ReactNode
  /** Tags shown elsewhere, such as an uploaded list's. */
  hideTagIds?: string[]
  onChange: (a: Audience) => void
}) {
  const tags = useQuery(contactTagsQueryOptions(workspaceId))
  // A segment made in another tab ("New segment") shows up on coming back.
  const segments = useQuery({
    ...segmentsQueryOptions(workspaceId),
    refetchOnWindowFocus: 'always',
  })
  const toggleSegment = (id: string) =>
    onChange({
      ...audience,
      all: false,
      segment_ids: audience.segment_ids.includes(id)
        ? audience.segment_ids.filter((s) => s !== id)
        : [...audience.segment_ids, id],
    })
  const toggleTag = (id: string) =>
    onChange({
      ...audience,
      all: false,
      tag_ids: audience.tag_ids.includes(id)
        ? audience.tag_ids.filter((t) => t !== id)
        : [...audience.tag_ids, id],
    })

  return (
    <>
      <StepHeading title="Who should receive this?">
        Pick everyone, or tags, segments and lists. People who appear in more than one are counted
        once. Only opted-in contacts get it.
      </StepHeading>

      <Button
        variant="unstyled"
        type="button"
        aria-pressed={audience.all}
        disabled={readOnly}
        onClick={() => onChange({ ...audience, all: !audience.all })}
        className={cn(
          'flex h-auto items-start justify-start gap-3 rounded-[14px] border-[1.5px] p-3.5 text-left',
          audience.all
            ? 'border-primary bg-brand-soft'
            : 'border-border bg-card hover:border-primary/60',
        )}
      >
        <span
          className={cn(
            'mt-0.5 flex size-5 flex-none items-center justify-center rounded-md border-2',
            audience.all ? 'border-primary bg-primary text-primary-foreground' : 'border-border',
          )}
        >
          {audience.all && <CheckIcon className="size-3" strokeWidth={3.5} aria-hidden="true" />}
        </span>
        <span className="flex flex-col leading-snug">
          <span className="flex items-center gap-2 font-bold">
            <UsersIcon className="size-4 text-brand-text" aria-hidden="true" />
            Everyone who opted in
          </span>
          <span className="text-xs text-muted-foreground">
            All contacts with marketing opt-in. Tags and lists are ignored.
          </span>
        </span>
      </Button>

      <div className={cn('flex flex-col gap-2.5', audience.all && 'opacity-50')}>
        <h3 className="text-sm font-bold">Tags</h3>
        {tags.isPending ? (
          <Skeleton className="h-10 w-full" />
        ) : (tags.data ?? []).length === 0 ? (
          <p className="text-[13px] text-muted-foreground">
            No tags yet. Tag contacts on the Contacts page, or upload a list below.
          </p>
        ) : (
          <div className="flex flex-wrap gap-2">
            {(tags.data ?? [])
              .filter((t) => !hideTagIds.includes(t.id))
              .map((t) => (
                <Chip
                  key={t.id}
                  on={!audience.all && audience.tag_ids.includes(t.id)}
                  disabled={readOnly || audience.all}
                  onToggle={() => toggleTag(t.id)}
                >
                  {t.name}
                  <span className="font-semibold opacity-75">{formatCount(t.contact_count)}</span>
                </Chip>
              ))}
          </div>
        )}
      </div>

      <div className={cn('flex flex-col gap-2.5', audience.all && 'opacity-50')}>
        <div className="flex items-center justify-between gap-2">
          <h3 className="text-sm font-bold">Segments</h3>
          {(segments.data ?? []).length > 0 && (
            <Link
              to="/contacts/segments"
              target="_blank"
              className="text-[13px] font-bold text-brand-text"
            >
              Manage segments
              <span className="sr-only"> (opens in a new tab)</span>
            </Link>
          )}
        </div>
        {segments.isPending ? (
          <Skeleton className="h-20 w-full" />
        ) : (segments.data ?? []).length === 0 ? (
          <p className="text-[13px] text-muted-foreground">
            No segments yet.{' '}
            <Link
              to="/contacts/segments"
              search={{ new: true }}
              target="_blank"
              className="font-bold text-brand-text"
            >
              Create one
            </Link>{' '}
            from rules like “wrote to us in the last 30 days”. It opens in a new tab; this list
            updates when you come back.
          </p>
        ) : (
          <div className="grid gap-2.5 sm:grid-cols-2 xl:grid-cols-3">
            {(segments.data ?? []).map((g) => {
              const on = !audience.all && audience.segment_ids.includes(g.id)
              return (
                <Button
                  key={g.id}
                  variant="unstyled"
                  type="button"
                  aria-pressed={on}
                  disabled={readOnly || audience.all}
                  onClick={() => toggleSegment(g.id)}
                  className={cn(
                    'flex h-auto items-start justify-start gap-3 rounded-[14px] border-[1.5px] p-3.5 text-left whitespace-normal',
                    on
                      ? 'border-primary bg-brand-soft'
                      : 'border-border bg-card hover:border-primary/60',
                  )}
                >
                  <span
                    className={cn(
                      'mt-0.5 flex size-5 flex-none items-center justify-center rounded-md border-2',
                      on ? 'border-primary bg-primary text-primary-foreground' : 'border-border',
                    )}
                  >
                    {on && <CheckIcon className="size-3" strokeWidth={3.5} aria-hidden="true" />}
                  </span>
                  <span className="flex flex-col leading-snug">
                    <span className="font-bold">{g.name}</span>
                    {g.description && (
                      <span className="text-xs text-muted-foreground">{g.description}</span>
                    )}
                    <span className="mt-1 text-[13px] font-bold">
                      {formatCount(g.count.opted_in)} opted-in contacts
                    </span>
                  </span>
                </Button>
              )
            })}
            {!readOnly && !audience.all && (
              <Link
                to="/contacts/segments"
                search={{ new: true }}
                target="_blank"
                className="flex min-h-[88px] items-center justify-center gap-2 rounded-[14px] border-[1.5px] border-dashed p-3.5 text-sm font-bold text-brand-text no-underline hover:border-primary/60 hover:bg-muted"
              >
                <PlusIcon className="size-4" aria-hidden="true" />
                New segment
                <span className="sr-only"> (opens in a new tab)</span>
              </Link>
            )}
          </div>
        )}
      </div>

      {extra && (
        <div className={cn('flex flex-col gap-5', audience.all && 'opacity-50')}>{extra}</div>
      )}

      {error && <p className="text-xs font-semibold text-destructive">{error}</p>}

      <Label className="flex cursor-pointer items-center gap-2.5 rounded-xl bg-muted px-3.5 py-3 font-normal">
        <Checkbox
          checked={audience.skip_recent_marketing}
          disabled={readOnly}
          onCheckedChange={(v) => onChange({ ...audience, skip_recent_marketing: v === true })}
        />
        <span className="flex flex-col leading-snug">
          <span className="font-semibold">
            Skip contacts who got a marketing message in the last 24 hours
          </span>
          <span className="text-xs text-muted-foreground">Helps protect your quality rating</span>
        </span>
      </Label>
    </>
  )
}
