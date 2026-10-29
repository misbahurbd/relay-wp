import { useQuery } from '@tanstack/react-query'
import { DownloadIcon, Loader2Icon, TagIcon, Trash2Icon, UserRoundIcon } from 'lucide-react'
import { type ComponentProps, useState } from 'react'
import { toast } from 'sonner'
import { Popover, PopoverContent, PopoverTrigger } from '#/components/ui/popover'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { downloadSelectedContacts } from '../../api/contacts.downloads'
import { useBulkContacts } from '../../api/contacts.mutations'
import { contactAgentsQueryOptions } from '../../api/contacts.queries'
import { useTags } from '../../hooks/use-tags'
import type { BulkInput, Contact } from '../../types'
import { ConfirmDialog } from '../kit/confirm-dialog'
import { plural } from '../kit/format'
import { TagPicker } from '../kit/tag-picker'

// React 19 passes ref as a prop, so Popover triggers can wrap this.
function BarButton({ className, ...props }: ComponentProps<'button'>) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-9 items-center gap-1.5 rounded-lg bg-primary-foreground/15 px-3 text-[13px] font-bold text-inherit hover:bg-primary-foreground/25 disabled:opacity-60 [&_svg]:size-4',
        className,
      )}
      {...props}
    />
  )
}

/** Actions on the selected rows. "Send broadcast" waits for Broadcasts (AD-11). */
export function BulkBar({
  workspaceId,
  selected,
  can,
  onClear,
  onDeleted,
}: {
  workspaceId: string
  selected: Contact[]
  can: { edit: boolean; export: boolean; delete: boolean }
  onClear: () => void
  onDeleted: (ids: string[]) => void
}) {
  const bulk = useBulkContacts(workspaceId)
  const { tags, create } = useTags(workspaceId)
  const [confirmDelete, setConfirmDelete] = useState(false)
  const [assignOpen, setAssignOpen] = useState(false)
  const agents = useQuery({ ...contactAgentsQueryOptions(workspaceId), enabled: assignOpen })
  const ids = selected.map((c) => c.id)

  const run = (body: Omit<BulkInput, 'contact_ids'>, done: (n: number) => string) =>
    bulk.mutate(
      { ...body, contact_ids: ids },
      {
        onSuccess: (n) => {
          toast.success(done(n))
          if (body.action === 'delete') {
            setConfirmDelete(false)
            onDeleted(ids)
            onClear()
          }
        },
        onError: (e) => {
          if (body.action !== 'delete') toast.error(errorMessage(e))
        },
      },
    )

  return (
    <section
      aria-label="Bulk actions"
      className="flex flex-wrap items-center gap-2 rounded-lg bg-primary py-2 pr-2.5 pl-4 text-primary-foreground"
    >
      <span className="mr-2 font-extrabold" aria-live="polite">
        {selected.length} selected
      </span>
      {bulk.isPending && <Loader2Icon className="size-4 animate-spin" aria-label="Working" />}
      {can.edit && (
        <TagPicker
          tags={tags}
          onCreate={create}
          label="Tag to add"
          onPick={(tag) =>
            run(
              { action: 'add_tag', tag_id: tag.id },
              (n) => `Tagged ${plural(n, 'contact')} “${tag.name}”`,
            )
          }
          trigger={
            <BarButton disabled={bulk.isPending}>
              <TagIcon aria-hidden="true" />
              Add tag
            </BarButton>
          }
        />
      )}
      {can.edit && (
        <Popover open={assignOpen} onOpenChange={setAssignOpen}>
          <PopoverTrigger asChild>
            <BarButton disabled={bulk.isPending}>
              <UserRoundIcon aria-hidden="true" />
              Assign agent
            </BarButton>
          </PopoverTrigger>
          <PopoverContent align="start" className="w-60 p-1.5">
            {agents.isPending ? (
              <p className="flex items-center gap-2 px-2 py-2 text-sm text-muted-foreground">
                <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
                Loading team…
              </p>
            ) : agents.isError ? (
              <p className="px-2 py-2 text-sm text-destructive">{errorMessage(agents.error)}</p>
            ) : (
              <ul aria-label="Agents" className="flex max-h-64 flex-col overflow-y-auto">
                {agents.data.map((a) => (
                  <li key={a.id}>
                    <button
                      type="button"
                      className="flex min-h-9 w-full items-center rounded-md px-2 text-left text-sm hover:bg-accent"
                      onClick={() => {
                        setAssignOpen(false)
                        run(
                          { action: 'assign', user_id: a.id },
                          (n) => `Assigned ${plural(n, 'contact')} to ${a.name}`,
                        )
                      }}
                    >
                      <span className="truncate">{a.name}</span>
                    </button>
                  </li>
                ))}
                <li className="mt-1 border-t pt-1">
                  <button
                    type="button"
                    className="flex min-h-9 w-full items-center rounded-md px-2 text-left text-sm text-muted-foreground hover:bg-accent"
                    onClick={() => {
                      setAssignOpen(false)
                      run({ action: 'assign' }, (n) => `Unassigned ${plural(n, 'contact')}`)
                    }}
                  >
                    Unassign
                  </button>
                </li>
              </ul>
            )}
          </PopoverContent>
        </Popover>
      )}
      {can.export && (
        <BarButton onClick={() => downloadSelectedContacts(selected)}>
          <DownloadIcon aria-hidden="true" />
          Export
        </BarButton>
      )}
      {can.delete && (
        <BarButton disabled={bulk.isPending} onClick={() => setConfirmDelete(true)}>
          <Trash2Icon aria-hidden="true" />
          Delete
        </BarButton>
      )}
      <button
        type="button"
        onClick={onClear}
        className="ml-auto h-9 rounded-lg px-3 text-[13px] font-bold underline"
      >
        Clear
      </button>
      <ConfirmDialog
        open={confirmDelete}
        onClose={() => {
          setConfirmDelete(false)
          bulk.reset()
        }}
        title={`Delete ${plural(selected.length, 'contact')}?`}
        description="They’re removed with their tags and custom fields. This can’t be undone."
        confirmLabel="Delete"
        pending={bulk.isPending}
        error={bulk.variables?.action === 'delete' ? bulk.error : null}
        onConfirm={() => run({ action: 'delete' }, (n) => `Deleted ${plural(n, 'contact')}`)}
      />
    </section>
  )
}
