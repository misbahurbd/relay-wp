import { useInfiniteQuery } from '@tanstack/react-query'
import { type Template, templatesQueryOptions } from '#/features/templates'
import { Select } from './field'

const CATEGORY: Record<string, string> = {
  marketing: 'Marketing',
  utility: 'Utility',
  authentication: 'Authentication',
}

/** Approved templates only: nothing else can be sent. */
export function useApprovedTemplates(workspaceId: string) {
  const q = useInfiniteQuery(templatesQueryOptions(workspaceId, { status: 'approved' }))
  return (q.data?.pages.flatMap((p) => p.items) ?? []) as Template[]
}

export function TemplateSelect({
  templates,
  value,
  onChange,
}: {
  templates: Template[]
  value?: string
  onChange: (id: string | undefined) => void
}) {
  return (
    <Select value={value ?? ''} onChange={(e) => onChange(e.target.value || undefined)}>
      <option value="">Choose a template</option>
      {templates.map((t) => (
        <option key={t.id} value={t.id}>
          {t.name} · {CATEGORY[t.category] ?? t.category} · Approved
        </option>
      ))}
    </Select>
  )
}
