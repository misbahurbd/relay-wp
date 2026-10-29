import { useInfiniteQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { CheckCircle2Icon, ClockIcon, PlusIcon, RefreshCwIcon, XCircleIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { useAccess } from '#/features/team'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useDeleteTemplate, useSyncTemplates } from '../../api/templates.mutations'
import { templatesQueryOptions } from '../../api/templates.queries'
import type { Template, TemplateFilter } from '../../types'
import { ActionError } from '../kit/action-error'
import { DeleteTemplateDialog } from '../kit/delete-template-dialog'
import { TemplateFilters } from './template-filters'
import { TemplateTable } from './template-table'

type Props = {
  workspaceId: string
  filter: TemplateFilter
  onFilter: (next: TemplateFilter) => void
}

export function TemplatesPage({ workspaceId, filter, onFilter }: Props) {
  const { can } = useAccess(workspaceId)
  const list = useInfiniteQuery(templatesQueryOptions(workspaceId, filter))
  const sync = useSyncTemplates(workspaceId)
  const remove = useDeleteTemplate(workspaceId)
  const [deleting, setDeleting] = useState<Template>()

  const counts = list.data?.pages[0]?.counts
  const items = list.data?.pages.flatMap((p) => p.items) ?? []
  const filtered = !!(filter.status || filter.category || filter.q)

  const empty = filtered ? (
    <div className="flex flex-col items-center gap-3">
      <p className="text-muted-foreground">No templates match these filters.</p>
      <Button variant="outline" size="sm" onClick={() => onFilter({})}>
        Clear filters
      </Button>
    </div>
  ) : (
    <div className="flex flex-col items-center gap-2">
      <p className="font-bold">No templates yet</p>
      <p className="max-w-prose text-muted-foreground">
        Create your first template, or pull the ones you already have on Meta.
      </p>
      {can('templates.create') && (
        <Link
          to="/templates/new"
          className="mt-1 inline-flex h-10 items-center gap-2 rounded-lg bg-primary px-4 font-bold text-primary-foreground no-underline hover:bg-primary/90 hover:text-primary-foreground"
        >
          <PlusIcon className="size-4" aria-hidden="true" />
          Create Template
        </Link>
      )}
    </div>
  )

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Message templates</h1>
          <p className="max-w-prose text-[15px] text-muted-foreground">
            Templates are pre-approved messages from Meta. You need one to start a chat or send a
            broadcast outside the 24-hour reply window.
          </p>
        </div>
        {can('templates.create') && (
          <div className="flex flex-wrap gap-2.5">
            <Button
              variant="outline"
              disabled={sync.isPending}
              onClick={() =>
                sync.mutate(undefined, {
                  onSuccess: (n) =>
                    toast.success(
                      n === 1 ? '1 template synced from Meta' : `${n} templates synced from Meta`,
                    ),
                })
              }
            >
              <RefreshCwIcon className={cn(sync.isPending && 'animate-spin')} aria-hidden="true" />
              {sync.isPending ? 'Syncing…' : 'Sync from Meta'}
            </Button>
            <Link
              to="/templates/new"
              className="inline-flex h-11 items-center gap-2 rounded-lg bg-primary px-[18px] font-bold text-primary-foreground no-underline hover:bg-primary/90 hover:text-primary-foreground"
            >
              <PlusIcon className="size-[18px]" aria-hidden="true" />
              Create Template
            </Link>
          </div>
        )}
      </div>

      {sync.isError && <ActionError error={sync.error} />}

      <section aria-label="Template summary" className="grid gap-3 sm:grid-cols-3">
        <SummaryCard
          icon={<CheckCircle2Icon aria-hidden="true" />}
          tone="bg-brand-soft text-brand-text"
          value={counts?.approved}
          label="Approved"
        />
        <SummaryCard
          icon={<ClockIcon aria-hidden="true" />}
          tone="bg-warning-soft text-warning"
          value={counts?.pending}
          label="In review by Meta"
        />
        <SummaryCard
          icon={<XCircleIcon aria-hidden="true" />}
          tone="bg-destructive-soft text-destructive"
          value={counts?.rejected}
          label="Rejected, needs edits"
        />
      </section>

      <section
        aria-label="Templates"
        className="flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:p-6"
      >
        <TemplateFilters filter={filter} counts={counts} onFilter={onFilter} />

        {list.isError ? (
          <Alert variant="danger">
            <AlertDescription className="flex flex-wrap items-center gap-3">
              {errorMessage(list.error, 'Couldn’t load templates.')}
              <Button variant="outline" size="sm" onClick={() => list.refetch()}>
                Try again
              </Button>
            </AlertDescription>
          </Alert>
        ) : (
          <TemplateTable
            items={items}
            loading={list.isPending}
            canEdit={can('templates.create')}
            canDelete={can('templates.delete')}
            empty={empty}
            onDelete={setDeleting}
          />
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

      {deleting && (
        <DeleteTemplateDialog
          template={deleting}
          pending={remove.isPending}
          error={remove.error}
          onClose={() => {
            remove.reset()
            setDeleting(undefined)
          }}
          onConfirm={() =>
            remove.mutate(deleting.id, {
              onSuccess: () => {
                toast.success(`${deleting.name} deleted`)
                setDeleting(undefined)
              },
            })
          }
        />
      )}
    </main>
  )
}

function SummaryCard({
  icon,
  tone,
  value,
  label,
}: {
  icon: ReactNode
  tone: string
  value?: number
  label: string
}) {
  return (
    <div className="flex items-center gap-3.5 rounded-xl border bg-card p-4 shadow-card">
      <span
        className={cn(
          'flex size-11 flex-none items-center justify-center rounded-lg [&_svg]:size-5',
          tone,
        )}
      >
        {icon}
      </span>
      <span className="flex flex-col">
        {value === undefined ? (
          <Skeleton className="h-7 w-10" />
        ) : (
          <span className="text-2xl font-extrabold tabular-nums">{value}</span>
        )}
        <span className="text-[13px] text-muted-foreground">{label}</span>
      </span>
    </div>
  )
}
