import { Loader2Icon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'
import { Modal } from './modal'

/** "Are you sure?" for destructive actions, with the API error kept on the dialog. */
export function ConfirmDialog({
  open,
  onClose,
  title,
  description,
  confirmLabel,
  pending,
  error,
  onConfirm,
}: {
  open: boolean
  onClose: () => void
  title: ReactNode
  description: ReactNode
  confirmLabel: string
  pending: boolean
  error: unknown
  onConfirm: () => void
}) {
  return (
    <Modal
      open={open}
      onClose={onClose}
      title={title}
      width="sm:max-w-[440px]"
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button variant="destructive" onClick={onConfirm} disabled={pending}>
            {pending && <Loader2Icon className="animate-spin" aria-hidden="true" />}
            {confirmLabel}
          </Button>
        </>
      }
    >
      <p className="text-muted-foreground">{description}</p>
      {!!error && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(error)}</AlertDescription>
        </Alert>
      )}
    </Modal>
  )
}
