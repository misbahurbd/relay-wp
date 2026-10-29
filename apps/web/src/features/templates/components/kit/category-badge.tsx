import { cn } from '#/lib/utils'
import { categoryLabel } from '../../schemas/templates.schema'
import type { TemplateCategory } from '../../types'

const DOT = {
  marketing: 'bg-info',
  utility: 'bg-brand',
  authentication: 'bg-warning',
} as const satisfies Record<TemplateCategory, string>

export function CategoryBadge({ category }: { category: TemplateCategory }) {
  return (
    <span className="inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-bold whitespace-nowrap">
      <span aria-hidden="true" className={cn('size-2 rounded-full', DOT[category])} />
      {categoryLabel(category)}
    </span>
  )
}
