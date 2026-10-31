import { XIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '#/components/ui/button'
import { Dialog, DialogContent, DialogDescription, DialogTitle } from '#/components/ui/dialog'
import { cn } from '#/lib/utils'

/** The prototype's modal: 18px radius, title + description, close icon. */
export function Modal({
  open = true,
  onClose,
  title,
  description,
  icon,
  children,
  width = 'sm:max-w-[520px]',
}: {
  open?: boolean
  onClose: () => void
  title: ReactNode
  description?: ReactNode
  icon?: ReactNode
  children: ReactNode
  width?: string
}) {
  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        showCloseButton={false}
        {...(description ? {} : { 'aria-describedby': undefined })}
        className={cn(
          'flex max-h-[calc(100dvh-64px)] flex-col gap-4 overflow-y-auto rounded-[18px] border-0 bg-card p-6',
          width,
        )}
      >
        <div className="flex items-start justify-between gap-3">
          <div className="flex min-w-0 items-start gap-3">
            {icon}
            <div className="flex min-w-0 flex-col gap-0.5">
              <DialogTitle className="text-xl font-extrabold">{title}</DialogTitle>
              {description && (
                <DialogDescription className="text-[13px] text-muted-foreground">
                  {description}
                </DialogDescription>
              )}
            </div>
          </div>
          <Button
            variant="unstyled"
            type="button"
            aria-label="Close"
            onClick={onClose}
            className="flex size-10 flex-none items-center justify-center rounded-[10px] text-muted-foreground hover:bg-muted hover:text-foreground"
          >
            <XIcon className="size-[18px]" aria-hidden="true" />
          </Button>
        </div>
        {children}
      </DialogContent>
    </Dialog>
  )
}
