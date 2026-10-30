import type { FormEvent, ReactNode } from 'react'
import { Button } from '#/components/ui/button'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '#/components/ui/dialog'
import { ApiErrorAlert } from './api-error'

/** A dialog around one form: the API's error (with its field details) shows above the buttons. */
export function FormDialog({
  title,
  description,
  submitLabel,
  pending,
  disabled = false,
  wide = false,
  error,
  onSubmit,
  onClose,
  children,
}: {
  title: ReactNode
  description?: ReactNode
  submitLabel: string
  pending: boolean
  /** Nothing to submit yet, e.g. no plan to pick. */
  disabled?: boolean
  /** Room for a table. */
  wide?: boolean
  error: unknown
  onSubmit: () => void
  onClose: () => void
  children: ReactNode
}) {
  const submit = (e: FormEvent) => {
    e.preventDefault()
    onSubmit()
  }
  return (
    <Dialog open onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        {...(description ? {} : { 'aria-describedby': undefined })}
        className={`flex max-h-[calc(100dvh-32px)] flex-col gap-4 overflow-y-auto rounded-xl ${wide ? 'sm:max-w-[680px]' : 'sm:max-w-[520px]'}`}
      >
        <DialogHeader>
          <DialogTitle className="text-xl font-extrabold">{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        <form onSubmit={submit} className="flex flex-col gap-4">
          {children}
          <ApiErrorAlert error={error} />
          <DialogFooter className="gap-2">
            <Button type="button" variant="outline" onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" disabled={pending || disabled}>
              {pending ? 'Saving…' : submitLabel}
            </Button>
          </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  )
}
