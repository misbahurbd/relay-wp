import { XIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '#/components/ui/dialog'
import { cn } from '#/lib/utils'

/** The design's dialog: title and description, a close icon, optional footer. */
export function Modal({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  width = 'sm:max-w-[560px]',
  closeLabel = 'Close',
}: {
  open: boolean
  onClose: () => void
  title: ReactNode
  description?: ReactNode
  children: ReactNode
  footer?: ReactNode
  width?: string
  closeLabel?: string
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        showCloseButton={false}
        {...(description ? {} : { 'aria-describedby': undefined })}
        className={cn(
          'flex max-h-[calc(100dvh-64px)] flex-col gap-0 overflow-hidden rounded-[20px] border-0 bg-card p-0',
          width,
        )}
      >
        <div className="flex items-start justify-between gap-3 px-6 pt-5">
          <div className="flex flex-col gap-0.5">
            <DialogTitle className="text-xl font-extrabold">{title}</DialogTitle>
            {description && (
              <DialogDescription className="text-[13px] text-muted-foreground">
                {description}
              </DialogDescription>
            )}
          </div>
          <button
            type="button"
            aria-label={closeLabel}
            onClick={onClose}
            className="flex size-10 flex-none items-center justify-center rounded-[10px] text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <XIcon className="size-[18px]" aria-hidden="true" />
          </button>
        </div>
        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-y-auto px-6 py-5">
          {children}
        </div>
        {footer && (
          <div className="flex flex-wrap items-center justify-between gap-2.5 border-t px-6 py-4">
            {footer}
          </div>
        )}
      </DialogContent>
    </Dialog>
  )
}
