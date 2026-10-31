import { useQueryClient } from '@tanstack/react-query'
import { Loader2Icon, UploadIcon } from 'lucide-react'
import { useId, useState } from 'react'
import { Button } from '#/components/ui/button'
import { Checkbox } from '#/components/ui/checkbox'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import { contactTagsQueryOptions } from '#/features/contacts'
import { errorMessage } from '#/lib/api'
import {
  type ListPreview,
  type ListUpload as Upload,
  useImportList,
  useMapList,
  useUploadList,
} from '../../api/broadcasts.mutations'
import { Fld, Select } from '../kit/field'
import { formatCount } from '../kit/format'

/** A list the wizard added: its tag, and what the import did. */
export type AddedList = {
  tagId: string
  fileName: string
  created: number
  updated: number
  skipped: number
}

/** The import's tag for the list: whitespace collapsed and cut to 50, as the server stores it. */
const tagNameFor = (file: string) =>
  [...`List · ${file.replace(/\.(csv|xlsx)$/i, '')}`.split(/\s+/).filter(Boolean).join(' ')]
    .slice(0, 50)
    .join('')
    .trim()

/**
 * "Upload a list": the file goes through the Contacts import (phone checks, consent, the plan's
 * contact limit), every row gets a list tag, and the broadcast targets that tag. Contacts stay.
 */
export function ListUpload({
  workspaceId,
  lists,
  readOnly,
  onAdd,
  onRemove,
}: {
  workspaceId: string
  lists: AddedList[]
  readOnly: boolean
  onAdd: (list: AddedList) => void
  onRemove: (tagId: string) => void
}) {
  const id = useId()
  const queryClient = useQueryClient()
  const upload = useUploadList(workspaceId)
  const map = useMapList(workspaceId)
  const run = useImportList(workspaceId)
  const [file, setFile] = useState<Upload>()
  const [phone, setPhone] = useState('')
  const [preview, setPreview] = useState<ListPreview>()
  const [consent, setConsent] = useState(false)
  const busy = upload.isPending || map.isPending || run.isPending
  const error = upload.error ?? map.error ?? run.error

  function reset() {
    setFile(undefined)
    setPreview(undefined)
    setConsent(false)
    upload.reset()
    map.reset()
    run.reset()
  }

  async function choose(f: File) {
    reset()
    const u = await upload.mutateAsync(f).catch(() => undefined)
    if (!u) return
    setFile(u)
    const guess = u.columns.find((c) => c.guess === 'phone')?.name ?? ''
    setPhone(guess)
    if (guess) {
      setPreview(
        await map
          .mutateAsync({ upload: u, phone: guess, tagName: tagNameFor(u.fileName) })
          .catch(() => undefined),
      )
    }
  }

  async function pickPhone(column: string) {
    setPhone(column)
    if (!file || !column) return
    setPreview(
      await map
        .mutateAsync({ upload: file, phone: column, tagName: tagNameFor(file.fileName) })
        .catch(() => undefined),
    )
  }

  async function addList() {
    if (!file) return
    const done = await run.mutateAsync(file.importId).catch(() => undefined)
    if (!done) return
    const tags = await queryClient.fetchQuery(contactTagsQueryOptions(workspaceId))
    const name = tagNameFor(file.fileName).toLowerCase()
    const tag = tags.find((t) => t.name.toLowerCase() === name)
    if (tag) onAdd({ tagId: tag.id, fileName: file.fileName, ...done })
    reset()
  }

  return (
    <div className="flex flex-col gap-2.5">
      <h3 className="text-sm font-bold">Upload a list</h3>

      {lists.map((l) => (
        <div
          key={l.tagId}
          className="flex items-center gap-3 rounded-[14px] border-[1.5px] border-primary bg-brand-soft px-3.5 py-3"
        >
          <span className="flex size-10 flex-none items-center justify-center rounded-[10px] bg-card text-[11px] font-extrabold text-brand-text">
            {l.fileName.toLowerCase().endsWith('.xlsx') ? 'XLSX' : 'CSV'}
          </span>
          <span className="flex min-w-0 flex-1 flex-col leading-tight">
            <span className="truncate font-bold">{l.fileName}</span>
            <span className="text-xs text-muted-foreground">
              {formatCount(l.created + l.updated)} contacts · {formatCount(l.created)} new
              {l.skipped > 0 && ` · ${formatCount(l.skipped)} skipped`}
            </span>
          </span>
          {!readOnly && (
            <Button variant="outline" size="sm" onClick={() => onRemove(l.tagId)}>
              Remove
            </Button>
          )}
        </div>
      ))}

      {!file && !readOnly && (
        <Label
          htmlFor={`${id}-file`}
          className="flex cursor-pointer flex-col items-center gap-1.5 rounded-[14px] border-[1.5px] border-dashed bg-muted p-[22px] text-center font-normal hover:border-primary/60"
        >
          {upload.isPending ? (
            <Loader2Icon className="size-[26px] animate-spin text-brand-text" aria-hidden="true" />
          ) : (
            <UploadIcon className="size-[26px] text-brand-text" aria-hidden="true" />
          )}
          <span className="font-bold">
            {upload.isPending ? 'Reading the file…' : 'Choose a CSV or Excel file'}
          </span>
          <span className="text-xs text-muted-foreground">
            Needs a phone column with country code. Up to 50,000 rows and 10 MB. Everyone on it is
            added to your contacts.
          </span>
          <Input
            id={`${id}-file`}
            type="file"
            accept=".csv,.xlsx"
            className="sr-only"
            disabled={busy}
            onChange={(e) => {
              const f = e.target.files?.[0]
              e.target.value = ''
              if (f) void choose(f)
            }}
          />
        </Label>
      )}

      {file && (
        <div className="flex flex-col gap-3 rounded-[14px] border p-4">
          <div className="flex items-center justify-between gap-3">
            <span className="truncate font-bold">{file.fileName}</span>
            <Button variant="ghost" size="sm" disabled={run.isPending} onClick={reset}>
              Cancel
            </Button>
          </div>
          <Fld
            label="Phone column"
            error={!phone ? 'Pick the column with phone numbers' : undefined}
          >
            <Select
              value={phone}
              disabled={busy}
              placeholder="Choose a column"
              onValueChange={(v) => void pickPhone(v)}
              options={file.columns.map((c) => ({ value: c.name, label: c.name }))}
            />
          </Fld>
          {preview && (
            <p className="text-[13px] text-muted-foreground">
              {formatCount(preview.rowsFound)} rows · {formatCount(preview.ready)} new contacts ·{' '}
              {formatCount(preview.updates)} already in your contacts
              {preview.invalid > 0 && ` · ${formatCount(preview.invalid)} invalid, skipped`}
            </p>
          )}
          <Label className="flex cursor-pointer items-start gap-2.5 font-normal">
            <Checkbox
              className="mt-0.5"
              checked={consent}
              onCheckedChange={(v) => setConsent(v === true)}
            />
            <span className="text-[13px]">
              Everyone on this list agreed to get WhatsApp messages from us. Contacts who opted out
              stay opted out.
            </span>
          </Label>
          <Button
            className="self-start"
            disabled={!preview || !consent || busy}
            onClick={() => void addList()}
          >
            {run.isPending && <Loader2Icon className="animate-spin" aria-hidden="true" />}
            {run.isPending ? 'Importing…' : 'Add list to audience'}
          </Button>
        </div>
      )}

      {error && (
        <p className="text-xs font-semibold text-destructive">
          {error instanceof Error && !('code' in error) ? error.message : errorMessage(error)}
        </p>
      )}
    </div>
  )
}
