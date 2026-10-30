import { cn } from '#/lib/utils'
import type { MetaCostCategory } from '../../types'

/**
 * Name and chart colour of each Meta message category. Colours are chart tokens, so both themes
 * work; wherever a colour shows, the name shows too.
 */
export const CATEGORY: Record<MetaCostCategory, { label: string; dot: string; color: string }> = {
  marketing: { label: 'Marketing', dot: 'bg-chart-2', color: 'var(--chart-2)' },
  utility: { label: 'Utility', dot: 'bg-chart-3', color: 'var(--chart-3)' },
  authentication: { label: 'Authentication', dot: 'bg-chart-4', color: 'var(--chart-4)' },
  service: { label: 'Service (customer replies)', dot: 'bg-chart-5', color: 'var(--chart-5)' },
}

/** A colour swatch followed by the category's name. */
export function CategoryLabel({
  category,
  label,
  className,
}: {
  category: MetaCostCategory
  label?: string
  className?: string
}) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <span
        aria-hidden="true"
        className={cn('size-2.5 shrink-0 rounded-[3px]', CATEGORY[category].dot)}
      />
      {label ?? CATEGORY[category].label}
    </span>
  )
}
