import { TriangleAlertIcon } from 'lucide-react'

/** A problem Meta may reject the template for, which doesn't block saving (AD-7 warnings). */
export function WarningNote({ children }: { children: string }) {
  return (
    <p className="flex items-start gap-2 rounded-lg bg-warning-soft px-3 py-2 text-sm text-foreground">
      <TriangleAlertIcon className="mt-0.5 size-4 flex-none text-warning" aria-hidden="true" />
      <span>
        <span className="sr-only">Warning: </span>
        {children}
      </span>
    </p>
  )
}
