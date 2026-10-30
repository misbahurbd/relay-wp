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
import { errorMessage } from '#/lib/api'
import type { Template } from '../../types'

/** Asks before deleting; a template Meta knows about is deleted there too (AD-8). */
export function DeleteTemplateDialog({
  template,
  pending,
  error,
  onConfirm,
  onClose,
}: {
  template: Template
  pending: boolean
  error: unknown
  onConfirm: () => void
  onClose: () => void
}) {
  return (
    <Dialog open onOpenChange={(open) => !open && !pending && onClose()}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Delete {template.name}?</DialogTitle>
          <DialogDescription>
            {template.meta_template_id
              ? 'This deletes it on Meta too, and can’t be undone.'
              : 'This draft hasn’t been sent to Meta. Deleting it can’t be undone.'}
          </DialogDescription>
        </DialogHeader>
        {error != null && (
          <Alert variant="danger">
            <AlertDescription>{errorMessage(error)}</AlertDescription>
          </Alert>
        )}
        <DialogFooter>
          <Button variant="outline" onClick={onClose} disabled={pending}>
            Cancel
          </Button>
          <Button variant="destructive" disabled={pending} onClick={onConfirm}>
            {pending ? 'Deleting…' : 'Delete template'}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  )
}
