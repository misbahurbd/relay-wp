import { CircleAlertIcon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { errorMessage } from '#/lib/api'

/** A failed load, with the API's reason (501 until a track ships: "isn't available yet") and a retry. */
export function ErrorState({
  error,
  title = 'Couldn’t load billing',
  onRetry,
}: {
  error: unknown
  title?: string
  onRetry?: () => void
}) {
  return (
    <Alert variant="danger">
      <CircleAlertIcon aria-hidden="true" />
      <AlertTitle>{title}</AlertTitle>
      <AlertDescription>
        <p>{errorMessage(error)}</p>
        {onRetry && (
          <Button variant="outline" size="sm" className="mt-2" onClick={onRetry}>
            Try again
          </Button>
        )}
      </AlertDescription>
    </Alert>
  )
}

export function EmptyState({
  icon,
  title,
  children,
  action,
}: {
  icon: ReactNode
  title: string
  children?: ReactNode
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col items-center gap-2 rounded-lg border border-dashed px-4 py-8 text-center">
      <span
        aria-hidden="true"
        className="flex size-10 items-center justify-center rounded-full bg-muted text-muted-foreground"
      >
        {icon}
      </span>
      <p className="font-bold">{title}</p>
      {children && <p className="max-w-prose text-[13px] text-muted-foreground">{children}</p>}
      {action && <div className="mt-2">{action}</div>}
    </div>
  )
}

/** Why an action is missing: the member's role lacks billing.pay. */
export function NoPayPermission({ children }: { children?: ReactNode }) {
  return (
    <p className="text-[13px] text-muted-foreground">
      {children ??
        'Only the owner or members with the “Pay and change plan” permission can do this.'}
    </p>
  )
}
