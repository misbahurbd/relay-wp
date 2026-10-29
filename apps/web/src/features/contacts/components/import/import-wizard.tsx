import { useQuery, useQueryClient } from '@tanstack/react-query'
import { CheckIcon, Loader2Icon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { errorMessage, isApiError } from '#/lib/api'
import { cn } from '#/lib/utils'
import { invalidateContacts } from '../../api/contacts.keys'
import { useMapImport, useStartImport, useUploadImport } from '../../api/contacts.mutations'
import { contactImportQueryOptions } from '../../api/contacts.queries'
import { importFileError, type MappingValues, mappingSchema } from '../../schemas/contacts.schema'
import { formatNumber } from '../kit/format'
import { Modal } from '../kit/modal'
import { DoneStep } from './done-step'
import { initialMapping, type MappingErrors, MapStep } from './map-step'
import { PreviewStep } from './preview-step'
import { UploadStep } from './upload-step'

const STEPS = ['Upload', 'Map columns', 'Preview', 'Done'] as const
type Step = 1 | 2 | 3 | 4

/** Upload, map, preview, then a background import polled every 2 s (AD-5). Mount it to open it. */
export function ImportWizard({
  workspaceId,
  onClose,
}: {
  workspaceId: string
  onClose: () => void
}) {
  const queryClient = useQueryClient()
  const [step, setStep] = useState<Step>(1)
  const [file, setFile] = useState<File | null>(null)
  const [fileError, setFileError] = useState<string>()
  const [mapping, setMapping] = useState<MappingValues | null>(null)
  const [mapErrors, setMapErrors] = useState<MappingErrors>({})

  const upload = useUploadImport(workspaceId)
  const map = useMapImport(workspaceId)
  const start = useStartImport(workspaceId)
  const importId = upload.data?.id
  const job = useQuery({
    ...contactImportQueryOptions(workspaceId, importId ?? ''),
    enabled: step === 4 && !!importId,
  })
  const finished = job.data?.status === 'done' || job.data?.status === 'failed'

  useEffect(() => {
    if (finished) void invalidateContacts(queryClient, workspaceId)
  }, [finished, queryClient, workspaceId])

  const pickFile = (f: File) => {
    const problem = importFileError(f)
    setFile(f)
    setFileError(problem)
    setMapping(null)
    upload.reset()
    if (!problem) {
      upload.mutate(f, {
        onError: (e) => setFileError(errorMessage(e, 'The upload failed. Try again.')),
      })
    }
  }

  const toPreview = () => {
    if (!mapping || !importId) return
    const parsed = mappingSchema.safeParse(mapping)
    if (!parsed.success) {
      const errs: MappingErrors = {}
      for (const issue of parsed.error.issues) {
        const key = issue.path[0] as keyof MappingErrors
        errs[key] ??= issue.message
      }
      setMapErrors(errs)
      return
    }
    setMapErrors({})
    map.mutate(
      {
        importId,
        columns: parsed.data.columns.map((c) => ({
          name: c.name,
          field: c.field,
          attribute_key: c.field === 'attribute' ? c.attribute_key || c.name : undefined,
        })),
        tagName: parsed.data.tag_name,
      },
      { onSuccess: () => setStep(3) },
    )
  }

  const back = () => {
    if (step === 1 || step === 4) onClose()
    else setStep((s) => (s - 1) as Step)
  }

  const ready = map.data ? map.data.ready + map.data.updates : 0
  const busy = upload.isPending || map.isPending || start.isPending

  let nextLabel = 'Next: map columns'
  let next: (() => void) | undefined
  if (step === 1) {
    next = upload.data
      ? () => {
          setMapping((m) => m ?? initialMapping(upload.data))
          setStep(2)
        }
      : undefined
  } else if (step === 2) {
    nextLabel = 'Next: preview'
    next = toPreview
  } else if (step === 3) {
    nextLabel = `Import ${formatNumber(ready)} ${ready === 1 ? 'contact' : 'contacts'}`
    next =
      ready > 0 && importId
        ? () => start.mutate(importId, { onSuccess: () => setStep(4) })
        : undefined
  } else {
    nextLabel = finished ? 'View contacts' : 'Close'
    next = onClose
  }

  const stepError = step === 2 ? map.error : step === 3 ? start.error : null

  return (
    <Modal
      open
      onClose={onClose}
      title="Import contacts"
      description="From a CSV or Excel file"
      closeLabel="Close import"
      width="sm:max-w-[720px]"
      footer={
        <>
          {step === 4 ? (
            <span />
          ) : (
            <Button variant="outline" onClick={back} disabled={busy && step !== 1}>
              {step === 1 ? 'Cancel' : 'Back'}
            </Button>
          )}
          <Button onClick={next} disabled={!next || busy}>
            {busy && step > 1 && <Loader2Icon className="animate-spin" aria-hidden="true" />}
            {nextLabel}
          </Button>
        </>
      }
    >
      <ol
        aria-label="Import steps"
        className="-mx-6 -mt-5 mb-1 flex flex-wrap gap-2 border-b px-6 py-[18px]"
      >
        {STEPS.map((label, i) => {
          const n = i + 1
          const done = n < step
          const now = n === step
          return (
            <li
              key={label}
              aria-current={now ? 'step' : undefined}
              className={cn(
                'flex flex-[1_1_120px] items-center gap-2 text-[13px] font-bold',
                done || now ? 'text-foreground' : 'text-muted-foreground',
              )}
            >
              <span
                className={cn(
                  'flex size-7 flex-none items-center justify-center rounded-full border-2 text-xs',
                  done && 'border-primary bg-primary text-primary-foreground',
                  now && 'border-primary bg-brand-soft text-brand-text',
                  !done && !now && 'border-input',
                )}
              >
                {done ? <CheckIcon className="size-3.5" aria-label="Done" /> : n}
              </span>
              {label}
            </li>
          )
        })}
      </ol>

      <div className="flex min-h-[300px] flex-col gap-4">
        {step === 1 && (
          <UploadStep
            file={file}
            upload={upload.data}
            uploading={upload.isPending}
            error={fileError}
            onFile={pickFile}
          />
        )}
        {step === 2 && upload.data && mapping && (
          <MapStep
            upload={upload.data}
            value={mapping}
            errors={mapErrors}
            onChange={(v) => {
              setMapping(v)
              if (Object.keys(mapErrors).length) setMapErrors({})
            }}
          />
        )}
        {step === 3 && map.data && <PreviewStep preview={map.data} />}
        {step === 4 && (
          <DoneStep
            workspaceId={workspaceId}
            job={job.data ?? start.data}
            tagName={mapping?.tag_name ?? ''}
          />
        )}
        {step === 4 && job.isError && (
          <Alert variant="danger">
            <AlertDescription>
              {errorMessage(job.error, 'We couldn’t check the import’s progress.')}
            </AlertDescription>
          </Alert>
        )}
        {stepError && (
          <Alert variant="danger">
            <AlertDescription>
              {errorMessage(stepError)}
              {isApiError(stepError) && !!stepError.details?.length && (
                <ul className="mt-1 list-disc pl-5">
                  {stepError.details.map((d) => (
                    <li key={`${d.field}-${d.message}`}>{d.message}</li>
                  ))}
                </ul>
              )}
            </AlertDescription>
          </Alert>
        )}
      </div>
    </Modal>
  )
}
