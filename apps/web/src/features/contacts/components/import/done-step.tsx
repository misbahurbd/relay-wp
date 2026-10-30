import { CheckIcon, Loader2Icon, TriangleAlertIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'
import { downloadSkippedRows } from '../../api/contacts.downloads'
import type { ContactImport } from '../../types'
import { plural } from '../kit/format'

/** Step 4: progress while the worker runs, then the result. */
export function DoneStep({
  workspaceId,
  job,
  tagName,
}: {
  workspaceId: string
  job: ContactImport | undefined
  tagName: string
}) {
  const [downloading, setDownloading] = useState(false)

  if (!job || job.status === 'importing' || job.status === 'mapped') {
    return (
      <div className="flex flex-col items-center gap-2.5 px-4 py-7 text-center" role="status">
        <Loader2Icon className="size-10 animate-spin text-brand-text" aria-hidden="true" />
        <span className="text-xl font-extrabold">Importing contacts…</span>
        <span className="max-w-[420px] text-muted-foreground">
          Large files take a minute or two. You can close this window; the import keeps going.
        </span>
      </div>
    )
  }

  if (job.status === 'failed') {
    return (
      <div className="flex flex-col items-center gap-2.5 px-4 py-7 text-center" role="alert">
        <span className="flex size-16 items-center justify-center rounded-full bg-destructive-soft text-destructive">
          <TriangleAlertIcon className="size-8" aria-hidden="true" />
        </span>
        <span className="text-xl font-extrabold">The import stopped</span>
        <span className="max-w-[420px] text-muted-foreground">
          {job.error || 'Something went wrong while importing.'}{' '}
          {job.created + job.updated > 0 &&
            `${plural(job.created, 'contact')} added and ${formatUpdated(job.updated)} before it stopped.`}
        </span>
      </div>
    )
  }

  const tag = tagName.trim()
  return (
    <div className="flex flex-col items-center gap-2.5 px-4 py-7 text-center" role="status">
      <span className="flex size-16 items-center justify-center rounded-full bg-brand-soft text-brand-text">
        <CheckIcon className="size-8" aria-hidden="true" />
      </span>
      <span className="text-xl font-extrabold">{plural(job.created, 'contact')} imported</span>
      <span className="max-w-[420px] text-muted-foreground">
        {formatUpdated(job.updated)} and {plural(job.skipped, 'row')}{' '}
        {job.skipped === 1 ? 'was' : 'were'} skipped.
        {tag && ` Everyone is tagged “${tag}”.`}
      </span>
      {job.has_skipped_file && job.skipped > 0 && (
        <Button
          variant="outline"
          className="mt-2 h-10"
          disabled={downloading}
          onClick={async () => {
            setDownloading(true)
            try {
              await downloadSkippedRows(workspaceId, job.id)
            } catch (e) {
              toast.error(errorMessage(e, 'The download failed. Please try again.'))
            } finally {
              setDownloading(false)
            }
          }}
        >
          {downloading && <Loader2Icon className="animate-spin" aria-hidden="true" />}
          Download skipped rows
        </Button>
      )}
    </div>
  )
}

function formatUpdated(n: number) {
  return `${plural(n, 'existing contact')} ${n === 1 ? 'was' : 'were'} updated`
}
