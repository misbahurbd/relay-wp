import type { ReactNode } from 'react'

/** Back on the left, the main action on the right; stacks on small screens. */
export function StepFooter({ children }: { children: ReactNode }) {
  return (
    <div className="mt-2 flex flex-col-reverse gap-3 border-t pt-5 sm:flex-row sm:items-center sm:justify-between">
      {children}
    </div>
  )
}
