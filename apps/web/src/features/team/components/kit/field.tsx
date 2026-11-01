import type { ReactNode } from 'react'

export const inputClass =
  'h-11 w-full rounded-[10px] border border-input bg-muted px-3 text-base text-foreground placeholder:text-muted-foreground focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring disabled:opacity-60 aria-invalid:border-destructive md:text-sm'

export function ErrorText({ id, children }: { id?: string; children?: ReactNode }) {
  if (!children) return null
  return (
    <p id={id} role="alert" className="text-[13px] text-destructive">
      {children}
    </p>
  )
}

// ---------- formatting ----------
