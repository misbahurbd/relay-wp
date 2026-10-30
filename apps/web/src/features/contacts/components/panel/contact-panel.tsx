import { useQuery } from '@tanstack/react-query'
import { PencilIcon, PlusIcon, Trash2Icon, XIcon } from 'lucide-react'
import { useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { useDeleteContact, useUpdateContact } from '../../api/contacts.mutations'
import { contactQueryOptions } from '../../api/contacts.queries'
import { useTags } from '../../hooks/use-tags'
import { SOURCE_LABEL } from '../../schemas/contacts.schema'
import type { ContactDetail, TagRef } from '../../types'
import { ContactDialog } from '../form/contact-dialog'
import { ContactAvatar } from '../kit/avatar'
import { ConfirmDialog } from '../kit/confirm-dialog'
import { formatDate } from '../kit/format'
import { OptInPill } from '../kit/opt-in-pill'
import { TagChip } from '../kit/tag-chip'
import { TagPicker } from '../kit/tag-picker'

const sectionTitle = 'text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase'

/** The right-hand contact panel. Orders, Spent, Broadcasts, activity and "Open chat" wait for Inbox and commerce (AD-11). */
export function ContactPanel({
  workspaceId,
  contactId,
  canEdit,
  canDelete,
  onClose,
}: {
  workspaceId: string
  contactId: string
  canEdit: boolean
  canDelete: boolean
  onClose: () => void
}) {
  const headingRef = useRef<HTMLHeadingElement>(null)

  // Move focus to the panel when it opens or shows another contact; on small screens it sits below the table.
  // biome-ignore lint/correctness/useExhaustiveDependencies: refocus when the contact changes
  useEffect(() => {
    headingRef.current?.focus({ preventScroll: true })
    headingRef.current?.scrollIntoView({ block: 'nearest', behavior: 'smooth' })
  }, [contactId])

  return (
    <aside
      aria-labelledby="contact-panel-title"
      className="flex w-full min-w-0 flex-col gap-[18px] border-t bg-card px-[18px] py-5 xl:sticky xl:top-16 xl:h-[calc(100dvh-4rem)] xl:w-[340px] xl:flex-none xl:overflow-y-auto xl:border-t-0 xl:border-l"
    >
      <div className="flex items-center justify-between">
        <h2
          id="contact-panel-title"
          ref={headingRef}
          tabIndex={-1}
          className={`${sectionTitle} outline-none`}
        >
          Contact
        </h2>
        <Button variant="ghost" size="icon-sm" aria-label="Close contact panel" onClick={onClose}>
          <XIcon aria-hidden="true" />
        </Button>
      </div>
      <ContactDetails
        workspaceId={workspaceId}
        contactId={contactId}
        canEdit={canEdit}
        canDelete={canDelete}
        onDeleted={onClose}
      />
    </aside>
  )
}

/**
 * A contact's summary, tags and details, loaded by ID. The contacts panel wraps it; other
 * features (the inbox's customer panel) embed it with `summary={false}` when they show the
 * name and phone themselves.
 */
export function ContactDetails({
  workspaceId,
  contactId,
  canEdit,
  canDelete = false,
  summary = true,
  onDeleted = () => {},
}: {
  workspaceId: string
  contactId: string
  canEdit: boolean
  canDelete?: boolean
  summary?: boolean
  onDeleted?: () => void
}) {
  const query = useQuery(contactQueryOptions(workspaceId, contactId))
  if (query.isPending) return <PanelSkeleton />
  if (query.isError)
    return (
      <Alert variant="danger">
        <AlertDescription>
          {errorMessage(query.error, 'We couldn’t load this contact.')}{' '}
          <button type="button" className="font-bold underline" onClick={() => query.refetch()}>
            Try again
          </button>
        </AlertDescription>
      </Alert>
    )
  return (
    <PanelBody
      workspaceId={workspaceId}
      contact={query.data}
      canEdit={canEdit}
      canDelete={canDelete}
      summary={summary}
      onDeleted={onDeleted}
    />
  )
}

function PanelBody({
  workspaceId,
  contact,
  canEdit,
  canDelete,
  summary,
  onDeleted,
}: {
  workspaceId: string
  contact: ContactDetail
  canEdit: boolean
  canDelete: boolean
  summary: boolean
  onDeleted: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [deleting, setDeleting] = useState(false)
  const update = useUpdateContact(workspaceId)
  const remove = useDeleteContact(workspaceId)
  const { tags, create } = useTags(workspaceId)

  const setTags = (next: TagRef[]) =>
    update.mutate(
      { contactId: contact.id, body: { tag_ids: next.map((t) => t.id) } },
      { onError: (e) => toast.error(errorMessage(e)) },
    )
  const addTag = (tag: TagRef) => {
    if (!contact.tags.some((t) => t.id === tag.id)) setTags([...contact.tags, tag])
  }

  const details: { k: string; v: string }[] = [
    { k: 'Email', v: contact.email ?? '—' },
    ...Object.entries(contact.attributes)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, v]) => ({ k, v: v || '—' })),
    { k: 'Language', v: contact.language ?? '—' },
    { k: 'Customer since', v: formatDate(contact.created_at) },
    { k: 'Source', v: SOURCE_LABEL[contact.source] },
    { k: 'Assigned agent', v: contact.assigned_to?.name ?? 'Unassigned' },
  ]
  if (contact.opt_in_at) {
    details.push({
      k: contact.opt_in === 'out' ? 'Opted out' : 'Opt-in recorded',
      v: [formatDate(contact.opt_in_at), contact.opt_in_source].filter(Boolean).join(' · '),
    })
  }

  return (
    <>
      {summary && (
        <div className="flex flex-col items-center gap-2 text-center">
          <ContactAvatar name={contact.name} seed={contact.id} size="lg" />
          <div className="flex flex-col leading-tight">
            <span className="text-[17px] font-extrabold break-words">{contact.name}</span>
            <span className="text-muted-foreground tabular-nums">{contact.phone_display}</span>
          </div>
          <OptInPill value={contact.opt_in} />
        </div>
      )}

      {(canEdit || canDelete) && (
        <div className="flex gap-2">
          {canEdit && (
            <Button variant="outline" className="flex-1" onClick={() => setEditing(true)}>
              <PencilIcon aria-hidden="true" />
              Edit
            </Button>
          )}
          {canDelete && (
            <Button
              variant="outline"
              size="icon"
              aria-label={`Delete ${contact.name}`}
              className="text-destructive hover:text-destructive"
              onClick={() => setDeleting(true)}
            >
              <Trash2Icon aria-hidden="true" />
            </Button>
          )}
        </div>
      )}

      <section aria-labelledby="contact-tags-title" className="flex flex-col gap-2">
        <h3 id="contact-tags-title" className={sectionTitle}>
          Tags
        </h3>
        <div className="flex flex-wrap items-center gap-1.5">
          {contact.tags.length === 0 && !canEdit && (
            <span className="text-[13px] text-muted-foreground">No tags</span>
          )}
          {contact.tags.map((t) => (
            <TagChip
              key={t.id}
              name={t.name}
              round
              onRemove={
                canEdit ? () => setTags(contact.tags.filter((x) => x.id !== t.id)) : undefined
              }
            />
          ))}
          {canEdit && (
            <TagPicker
              tags={tags}
              selectedIds={contact.tags.map((t) => t.id)}
              onPick={addTag}
              onCreate={create}
              trigger={
                <button
                  type="button"
                  disabled={update.isPending}
                  className="inline-flex h-7 items-center gap-1 rounded-full border border-dashed border-input px-2.5 text-xs font-bold text-brand-text hover:bg-muted disabled:opacity-60"
                >
                  <PlusIcon className="size-3" aria-hidden="true" />
                  Add tag
                </button>
              }
            />
          )}
        </div>
      </section>

      <section aria-labelledby="contact-details-title" className="flex flex-col">
        <h3 id="contact-details-title" className={`${sectionTitle} mb-1`}>
          Details
        </h3>
        <dl className="flex flex-col">
          {details.map((d) => (
            <div
              key={d.k}
              className="flex justify-between gap-3 border-b py-[7px] text-[13px] last:border-b-0"
            >
              <dt className="text-muted-foreground">{d.k}</dt>
              <dd className="min-w-0 text-right font-semibold break-words">{d.v}</dd>
            </div>
          ))}
        </dl>
      </section>

      <ContactDialog
        workspaceId={workspaceId}
        contact={contact}
        open={editing}
        onClose={() => setEditing(false)}
      />
      <ConfirmDialog
        open={deleting}
        onClose={() => {
          setDeleting(false)
          remove.reset()
        }}
        title={`Delete ${contact.name}?`}
        description="The contact, its tags and custom fields are removed. This can’t be undone."
        confirmLabel="Delete contact"
        pending={remove.isPending}
        error={remove.error}
        onConfirm={() =>
          remove.mutate(contact.id, {
            onSuccess: () => {
              toast.success('Contact deleted')
              setDeleting(false)
              onDeleted()
            },
          })
        }
      />
    </>
  )
}

function PanelSkeleton() {
  return (
    <div className="flex flex-col gap-4" role="status" aria-label="Loading contact">
      <div className="flex flex-col items-center gap-2">
        <Skeleton className="size-16 rounded-full" />
        <Skeleton className="h-5 w-36" />
        <Skeleton className="h-4 w-28" />
      </div>
      <Skeleton className="h-11 w-full" />
      <Skeleton className="h-6 w-2/3" />
      {[0, 1, 2, 3, 4].map((i) => (
        <Skeleton key={i} className="h-5 w-full" />
      ))}
    </div>
  )
}
