import type { ReactNode } from 'react'
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '#/components/ui/dialog'

export function Modal({
  open = true,
  onClose,
  title,
  description,
  children,
  footer,
}: {
  open?: boolean
  onClose: () => void
  title: ReactNode
  description?: ReactNode
  children?: ReactNode
  footer?: ReactNode
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        // Without a description, don't point aria-describedby at a missing element.
        {...(description ? {} : { 'aria-describedby': undefined })}
        className="flex max-h-[calc(100dvh-32px)] flex-col gap-4 overflow-y-auto rounded-xl sm:max-w-[520px]"
      >
        <DialogHeader>
          <DialogTitle className="text-xl font-extrabold">{title}</DialogTitle>
          {description && <DialogDescription>{description}</DialogDescription>}
        </DialogHeader>
        {children}
        {footer && <DialogFooter className="gap-2">{footer}</DialogFooter>}
      </DialogContent>
    </Dialog>
  )
}
