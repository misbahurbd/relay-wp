import { createFileRoute } from '@tanstack/react-router'
import { TemplatesPage, templateSearchSchema } from '#/features/templates'

export const Route = createFileRoute('/_app/w/$workspaceId/_shell/templates/')({
  validateSearch: templateSearchSchema,
  head: () => ({ meta: [{ title: 'Templates · Relay' }] }),
  component: Page,
})

function Page() {
  const { workspaceId } = Route.useParams()
  const filter = Route.useSearch()
  const navigate = Route.useNavigate()
  return (
    <TemplatesPage
      workspaceId={workspaceId}
      filter={filter}
      onFilter={(next) => navigate({ search: next, replace: true })}
    />
  )
}
