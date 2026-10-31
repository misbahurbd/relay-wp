import { useQuery } from '@tanstack/react-query'
import { DownloadIcon, Loader2Icon, PlusIcon, UploadIcon, UsersIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { useAccess } from '#/features/team'
import { errorMessage } from '#/lib/api'
import { downloadContactsExport } from '../../api/contacts.downloads'
import {
  contactsListQueryOptions,
  contactsSummaryQueryOptions,
  contactTagsQueryOptions,
  PAGE_SIZE,
} from '../../api/contacts.queries'
import type { ContactsSearch } from '../../schemas/contacts.schema'
import type { Contact, ContactFilter } from '../../types'
import { ContactDialog } from '../form/contact-dialog'
import { ImportWizard } from '../import/import-wizard'
import { formatNumber } from '../kit/format'
import { ContactPanel } from '../panel/contact-panel'
import { BulkBar } from './bulk-bar'
import { ContactsTable } from './contacts-table'
import { ContactsToolbar } from './contacts-toolbar'

/** The Contacts screen: header, filters, table with bulk actions, and the contact panel. */
export function ContactsPage({
  workspaceId,
  search,
  onSearchChange,
}: {
  workspaceId: string
  search: ContactsSearch
  onSearchChange: (next: ContactsSearch) => void
}) {
  const { cursor, ...filter } = search
  const filterKey = JSON.stringify(filter)
  const pageKey = `${filterKey}|${cursor ?? ''}`
  const { can } = useAccess(workspaceId)
  const perms = {
    edit: can('contacts.edit'),
    import: can('contacts.import'),
    export: can('contacts.export'),
    delete: can('contacts.delete'),
  }

  const summary = useQuery(contactsSummaryQueryOptions(workspaceId))
  const list = useQuery(contactsListQueryOptions(workspaceId, filter, cursor))
  const { data: tags = [] } = useQuery(contactTagsQueryOptions(workspaceId))

  // Cursors of the pages before this one, so Previous can go back. Reset by a new filter.
  const [nav, setNav] = useState({ key: filterKey, stack: [] as (string | undefined)[] })
  const stack = nav.key === filterKey ? nav.stack : []
  // Selection belongs to the page on screen.
  const [sel, setSel] = useState({ key: pageKey, rows: new Map<string, Contact>() })
  const selected = sel.key === pageKey ? sel.rows : new Map<string, Contact>()
  const setSelected = (rows: Map<string, Contact>) => setSel({ key: pageKey, rows })

  const [openId, setOpenId] = useState<string | null>(null)
  const [adding, setAdding] = useState(false)
  const [importing, setImporting] = useState(false)
  const [exporting, setExporting] = useState(false)

  const setFilter = (next: ContactFilter) => onSearchChange({ ...next, cursor: undefined })
  const hasFilter = Object.values(filter).some((v) => v !== undefined)

  const page = list.data
  const offset = stack.length > 0 || !cursor ? stack.length * PAGE_SIZE : null
  const rows = page?.items

  const exportAll = async () => {
    setExporting(true)
    try {
      await downloadContactsExport(workspaceId, filter)
    } catch (e) {
      toast.error(errorMessage(e, 'The export failed. Please try again.'))
    } finally {
      setExporting(false)
    }
  }

  const emptyState = hasFilter ? (
    <EmptyState
      title="No contacts match these filters"
      body="Try another search, or clear the filters to see everyone."
      actions={
        <Button variant="outline" onClick={() => setFilter({})}>
          Clear filters
        </Button>
      }
    />
  ) : (
    <EmptyState
      title="No contacts yet"
      body="Import a CSV or Excel file, or add someone by hand. Only message people who agreed to hear from you on WhatsApp."
      actions={
        <>
          {perms.import && (
            <Button onClick={() => setImporting(true)}>
              <UploadIcon aria-hidden="true" />
              Import contacts
            </Button>
          )}
          {perms.edit && (
            <Button variant="outline" onClick={() => setAdding(true)}>
              <PlusIcon aria-hidden="true" />
              Add contact
            </Button>
          )}
        </>
      }
    />
  )

  return (
    <div className="flex min-h-0 flex-1 flex-col xl:flex-row xl:items-start">
      <main className="flex min-w-0 flex-1 flex-col gap-4 p-4 sm:px-7 sm:py-6">
        <div className="flex flex-wrap items-end justify-between gap-3.5">
          <div className="flex flex-col gap-1">
            <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Contacts</h1>
            {summary.data ? (
              <p className="text-muted-foreground">
                {formatNumber(summary.data.total)}{' '}
                {summary.data.total === 1 ? 'contact' : 'contacts'} ·{' '}
                {formatNumber(summary.data.opted_in)} opted in to WhatsApp marketing
              </p>
            ) : summary.isPending ? (
              <Skeleton className="h-5 w-72" />
            ) : null}
          </div>
          <div className="flex flex-wrap gap-2.5">
            {perms.export && (
              <Button variant="outline" onClick={exportAll} disabled={exporting}>
                {exporting ? (
                  <Loader2Icon className="animate-spin" aria-hidden="true" />
                ) : (
                  <DownloadIcon aria-hidden="true" />
                )}
                Export
              </Button>
            )}
            {perms.import && (
              <Button
                variant="outline"
                className="border-primary text-brand-text"
                onClick={() => setImporting(true)}
              >
                <UploadIcon aria-hidden="true" />
                Import CSV/Excel
              </Button>
            )}
            {perms.edit && (
              <Button onClick={() => setAdding(true)}>
                <PlusIcon aria-hidden="true" />
                Add contact
              </Button>
            )}
          </div>
        </div>

        <ContactsToolbar filter={filter} tags={tags} onChange={setFilter} />

        {selected.size > 0 && (
          <BulkBar
            workspaceId={workspaceId}
            selected={[...selected.values()]}
            can={perms}
            onClear={() => setSelected(new Map())}
            onDeleted={(ids) => {
              if (openId && ids.includes(openId)) setOpenId(null)
            }}
          />
        )}

        {list.isError && !page ? (
          <Alert variant="danger">
            <AlertDescription>
              {errorMessage(list.error, 'We couldn’t load your contacts.')}{' '}
              <Button
                variant="unstyled"
                type="button"
                className="font-bold underline"
                onClick={() => list.refetch()}
              >
                Try again
              </Button>
            </AlertDescription>
          </Alert>
        ) : (
          <ContactsTable
            rows={rows}
            loading={list.isFetching}
            selectable={perms.edit || perms.export || perms.delete}
            selectedIds={new Set(selected.keys())}
            openId={openId}
            onOpen={setOpenId}
            onToggle={(c) => {
              const next = new Map(selected)
              if (next.has(c.id)) next.delete(c.id)
              else next.set(c.id, c)
              setSelected(next)
            }}
            onToggleAll={(on) =>
              setSelected(new Map(on && rows ? rows.map((r) => [r.id, r] as const) : []))
            }
            empty={emptyState}
            footer={
              page && page.items.length > 0 ? (
                <div className="flex flex-wrap items-center justify-between gap-2.5 border-t px-4 py-3 text-[13px] text-muted-foreground">
                  <span aria-live="polite">
                    {offset !== null
                      ? `Showing ${formatNumber(offset + 1)}–${formatNumber(offset + page.items.length)} of ${formatNumber(page.total)}`
                      : `Showing ${formatNumber(page.items.length)} of ${formatNumber(page.total)}`}
                  </span>
                  <nav aria-label="Pages" className="flex gap-1.5">
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={!cursor || list.isFetching}
                      onClick={() => {
                        const prev = stack.at(-1)
                        setNav({ key: filterKey, stack: stack.slice(0, -1) })
                        onSearchChange({ ...filter, cursor: prev })
                      }}
                    >
                      Previous
                    </Button>
                    <Button
                      variant="outline"
                      size="sm"
                      disabled={!page.next || list.isFetching}
                      onClick={() => {
                        if (!page.next) return
                        setNav({ key: filterKey, stack: [...stack, cursor] })
                        onSearchChange({ ...filter, cursor: page.next })
                      }}
                    >
                      Next
                    </Button>
                  </nav>
                </div>
              ) : null
            }
          />
        )}
      </main>

      {openId && (
        <ContactPanel
          key={openId}
          workspaceId={workspaceId}
          contactId={openId}
          canEdit={perms.edit}
          canDelete={perms.delete}
          onClose={() => setOpenId(null)}
        />
      )}

      <ContactDialog
        workspaceId={workspaceId}
        open={adding}
        onClose={() => setAdding(false)}
        onSaved={setOpenId}
      />
      {perms.import && importing && (
        <ImportWizard workspaceId={workspaceId} onClose={() => setImporting(false)} />
      )}
    </div>
  )
}

function EmptyState({ title, body, actions }: { title: string; body: string; actions: ReactNode }) {
  return (
    <div className="flex flex-col items-center gap-2 px-4 py-14 text-center">
      <span className="mb-1 flex size-12 items-center justify-center rounded-xl bg-brand-soft text-brand-text">
        <UsersIcon className="size-6" aria-hidden="true" />
      </span>
      <p className="text-base font-bold">{title}</p>
      <p className="max-w-prose text-muted-foreground">{body}</p>
      <div className="mt-2 flex flex-wrap justify-center gap-2.5">{actions}</div>
    </div>
  )
}
