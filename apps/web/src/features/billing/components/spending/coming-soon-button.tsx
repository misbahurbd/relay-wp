import type { ReactNode } from 'react'
import { Button } from '#/components/ui/button'

/** A preview action that isn't built yet: shown, disabled, and saying so. */
export function ComingSoonButton({
  icon,
  children,
  variant = 'outline',
}: {
  icon?: ReactNode
  children: ReactNode
  variant?: 'outline' | 'link' | 'soft'
}) {
  return (
    <Button type="button" variant={variant} size="sm" disabled>
      {icon}
      {children}
      <span className="text-xs font-semibold text-muted-foreground">Coming soon</span>
    </Button>
  )
}
