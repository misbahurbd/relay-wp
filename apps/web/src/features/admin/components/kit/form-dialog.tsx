import type { FormEvent, ReactNode } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '#/components/ui/dialog'
import { errorMessage, isApiError } from '#/lib/api'

/** A dialog around one form: the API's error (with its field details) shows above the buttons. */
export function FormDialog({
  title,
  description,
  submitLabel,
  pending,
  error,
  onSubmit,
  onClose,
  children,
}: {
  title: ReactNode
  description?: ReactNode
  submitLabel: string
  pending: boolean
  error: unknown
  onSubmit: () => void
  onClose: () => void
  children: ReactNode
}) {
  const details = isApiError(error) ? (error.details ?? []) : []
  const submit = (e: FormEvent) => {
    e.preventDefault()
    onSubmit()
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        {...(description ? {} : { 'aria-describedby': undefined })}
        className="flex max-h-[calc(100dvh-32px)] flex-col gap-4 overflow-y-auto rounded-xl sm:max-w-[520px]"
      >
        <DialogHeader>
          <DialogTitle className="text-xl font-extrabold">{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4">
          {children}
          {error != null && (
            <Alert variant="danger">
              <AlertDescription>
                <p>{errorMessage(error)}</p>
                {details.length > 0 && (
                  <ul className="mt-1 list-disc pl-4">
                    {details.map((d) => (
                      <li key={`${d.field}-${d.message}`}>
                        {d.field ? `${d.field.replace(/^body\./, '')}: ` : ''}
                        {d.message}
                      </li>
                    ))}
                  </ul>
                )}
              </AlertDescription>
            </Alert>
          )}
          <DialogFooter className="gap-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending}>
              {pending ? 'Saving…' : submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
