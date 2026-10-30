import { CheckCircle2Icon, InfoIcon, Loader2Icon, UploadIcon } from 'lucide-react'
import { useId, useRef, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'
import { downloadSampleFile } from '../../api/contacts.downloads'
import type { ImportUpload } from '../../types'
import { formatBytes, plural } from '../kit/format'

/** Step 1: pick or drop a .csv/.xlsx file; it uploads straight away. */
export function UploadStep({
  file,
  upload,
  uploading,
  error,
  onFile,
}: {
  file: File | null
  upload: ImportUpload | undefined
  uploading: boolean
  error: string | undefined
  onFile: (file: File) => void
}) {
  const inputRef = useRef<HTMLInputElement>(null)
  const inputId = useId()
  const [dragging, setDragging] = useState(false)

  return (
    <>
      <section
        aria-label="File drop area"
        onDragOver={(e) => {
          e.preventDefault()
          setDragging(true)
        }}
        onDragLeave={() => setDragging(false)}
        onDrop={(e) => {
          e.preventDefault()
          setDragging(false)
          const f = e.dataTransfer.files[0]
          if (f) onFile(f)
        }}
        className={cn(
          'flex flex-col items-center gap-2.5 rounded-xl border-2 border-dashed bg-muted px-4 py-8 text-center',
          dragging ? 'border-primary' : 'border-input',
        )}
      >
        <span className="flex size-13 items-center justify-center rounded-[14px] bg-brand-soft text-brand-text">
          <UploadIcon className="size-6.5" aria-hidden="true" />
        </span>
        <span className="text-base font-bold">Drag your file here</span>
        <span className="text-[13px] text-muted-foreground">
          .csv or .xlsx · up to 10 MB and 50,000 rows
        </span>
        <input
          ref={inputRef}
          id={inputId}
          type="file"
          accept=".csv,.xlsx,text/csv,application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
          className="sr-only"
          tabIndex={-1}
          onChange={(e) => {
            const f = e.target.files?.[0]
            if (f) onFile(f)
            e.target.value = ''
          }}
        />
        <Button
          variant="outline"
          className="h-10"
          disabled={uploading}
          onClick={() => inputRef.current?.click()}
        >
          Browse files
        </Button>
      </section>

      {file && (
        <div className="flex items-center gap-3 rounded-lg border px-3.5 py-3">
          <span
            aria-hidden="true"
            className="flex size-[38px] flex-none items-center justify-center rounded-sm bg-brand-soft text-[10px] font-extrabold text-brand-text uppercase"
          >
            {file.name.split('.').pop()}
          </span>
          <span className="flex min-w-0 flex-1 flex-col leading-tight">
            <span className="truncate font-bold">{file.name}</span>
            <span className="text-xs text-muted-foreground">
              {upload
                ? `${plural(upload.row_count, 'row')} · ${plural(upload.columns.length, 'column')} · ${formatBytes(file.size)}`
                : formatBytes(file.size)}
            </span>
          </span>
          {uploading ? (
            <Loader2Icon
              className="size-[18px] animate-spin text-muted-foreground"
              role="img"
              aria-label="Uploading"
            />
          ) : upload ? (
            <CheckCircle2Icon
              className="size-[18px] text-brand-text"
              role="img"
              aria-label="Uploaded"
            />
          ) : null}
        </div>
      )}

      {error && (
        <Alert variant="danger">
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      <div className="flex gap-2.5 rounded-lg bg-info-soft px-3.5 py-3 text-[13px]">
        <InfoIcon className="mt-0.5 size-4 flex-none text-info" aria-hidden="true" />
        <span>
          <strong>Tip:</strong> include the country code in phone numbers (for example +44 7700
          900412). Numbers without one are skipped.{' '}
          <button
            type="button"
            className="font-semibold text-brand-text underline"
            onClick={downloadSampleFile}
          >
            Download sample file
          </button>
        </span>
      </div>
    </>
  )
}
