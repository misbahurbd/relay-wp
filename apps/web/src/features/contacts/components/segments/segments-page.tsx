import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { useAccess } from '#/features/team'
import { errorMessage, isApiError } from '#/lib/api'
import { contactTagsQueryOptions } from '../../api/contacts.queries'
import { segmentsQueryOptions, useDeleteSegment, useSaveSegment } from '../../api/contacts.segments'
import { blankCondition, rulesComplete } from '../../schemas/segments.schema'
import type { Segment } from '../../types'
import { ContactsTabs } from '../kit/contacts-tabs'
import { formatNumber } from '../kit/format'
import { SegmentBuilder, type SegmentDraft } from './segment-builder'
import { SegmentList } from './segment-list'
import { WhoMatches } from './who-matches'

const draftOf = (s: Segment): SegmentDraft => ({
  name: s.name,
  description: s.description,
  rules: structuredClone(s.rules),
})

/** Contacts › Segments: saved rules on the left, the builder, and who matches. */
export function SegmentsPage({
  workspaceId,
  startNew = false,
}: {
  workspaceId: string
  /** Open on a blank segment instead of the first saved one. */
  startNew?: boolean
}) {
  const navigate = useNavigate()
  const { access, can } = useAccess(workspaceId)
  const segments = useQuery(segmentsQueryOptions(workspaceId))
  const tags = useQuery(contactTagsQueryOptions(workspaceId))
  const save = useSaveSegment(workspaceId)
  const remove = useDeleteSegment(workspaceId)
  const [selectedId, setSelectedId] = useState<string | undefined>(startNew ? '' : undefined)
  const [draft, setDraft] = useState<SegmentDraft>()
  const [dirty, setDirty] = useState(false)
  const [errors, setErrors] = useState<Record<string, string>>({})

  const canEdit = !access || can('contacts.edit')
  const list = segments.data ?? []
  const tagList = tags.data ?? []
  const selected = list.find((s) => s.id === selectedId)
  const fresh: SegmentDraft = {
    name: 'Untitled segment',
    description: '',
    rules: { groups: [{ match: 'all', conditions: [blankCondition('opt_in', tagList)] }] },
  }
  const shown = selectedId === undefined ? list[0] : selected
  const current = draft ?? (shown ? draftOf(shown) : fresh)
  // '' is a new segment; undefined, before any choice, is the first one.
  const editingId = selectedId ?? list[0]?.id

  function edit(d: SegmentDraft) {
    // The first segment is shown before any is picked; editing it keeps editing it.
    if (selectedId === undefined) setSelectedId(list[0]?.id ?? '')
    setDraft(d)
    setDirty(true)
  }

  function select(s?: Segment) {
    setSelectedId(s?.id ?? '')
    setDraft(s ? draftOf(s) : fresh)
    setDirty(false)
    setErrors({})
  }

  async function submit(thenBroadcast: boolean) {
    try {
      const saved = await save.mutateAsync({ id: editingId || undefined, body: current })
      setSelectedId(saved.id)
      setDraft(draftOf(saved))
      setDirty(false)
      setErrors({})
      toast.success(`${saved.name} saved`)
      if (thenBroadcast) navigate({ to: '/broadcasts/new', search: { segment: saved.id } })
    } catch (e) {
      const map: Record<string, string> = {}
      if (isApiError(e)) for (const d of e.details ?? []) if (d.field) map[d.field] ??= d.message
      setErrors(map)
    }
  }

  return (
    <main className="mx-auto flex w-full max-w-[1440px] flex-col gap-4 p-4 sm:px-7 sm:py-6">
      <div className="flex flex-wrap items-end justify-between gap-3">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Contacts</h1>
          <p className="text-muted-foreground">
            {formatNumber(list.length)} {list.length === 1 ? 'segment' : 'segments'} · Segments
            group contacts automatically by rules you set
          </p>
        </div>
        {canEdit && (
          <Button onClick={() => select(undefined)}>
            <PlusIcon aria-hidden="true" />
            New segment
          </Button>
        )}
      </div>
      <ContactsTabs current="segments" segments={segments.data?.length} />

      {segments.isError && (
        <Alert variant="danger">
          <AlertDescription>
            {errorMessage(segments.error, 'Couldn’t load segments.')}
          </AlertDescription>
        </Alert>
      )}

      <div className="grid items-start gap-5 xl:grid-cols-[300px_minmax(0,1fr)_320px]">
        <SegmentList
          segments={list}
          loading={segments.isPending}
          selectedId={editingId}
          canEdit={canEdit}
          canBroadcast={!access || can('broadcasts.create')}
          deleting={remove.isPending}
          onSelect={select}
          onDuplicate={(s) => {
            setSelectedId('')
            edit({ ...draftOf(s), name: `${s.name} (copy)`.slice(0, 80) })
          }}
          onDelete={(s) =>
            remove.mutate(s.id, {
              onSuccess: () => {
                toast.success(`${s.name} deleted`)
                select(undefined)
              },
            })
          }
        />
        <SegmentBuilder
          title={editingId ? `Edit “${shown?.name ?? current.name}”` : 'New segment'}
          draft={current}
          tags={tagList}
          readOnly={!canEdit}
          dirty={dirty}
          errors={errors}
          onChange={edit}
          actions={
            canEdit && (
              <>
                <Button
                  disabled={save.isPending || !rulesComplete(current.rules) || !current.name.trim()}
                  onClick={() => submit(false)}
                >
                  {save.isPending ? 'Saving…' : 'Save segment'}
                </Button>
                {can('broadcasts.create') && (
                  <Button
                    variant="outline"
                    disabled={
                      save.isPending || !rulesComplete(current.rules) || !current.name.trim()
                    }
                    onClick={() => submit(true)}
                  >
                    Save and create broadcast
                  </Button>
                )}
                {dirty && (
                  <Button variant="ghost" onClick={() => select(editingId ? shown : undefined)}>
                    Discard changes
                  </Button>
                )}
                {save.error && !Object.keys(errors).length && (
                  <span className="text-[13px] text-destructive">{errorMessage(save.error)}</span>
                )}
              </>
            )
          }
        />
        <WhoMatches workspaceId={workspaceId} rules={current.rules} />
      </div>
    </main>
  )
}
