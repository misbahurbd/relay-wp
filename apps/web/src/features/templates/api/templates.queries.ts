import { getTemplate, listTemplates, type TemplateDto } from '@relay/api-client'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { Template, TemplateFilter } from '../types'
import { templatesKeys } from './templates.keys'

const PAGE_SIZE = 50

/** Fills the lists the API may send as null, so components never check for it. */
export function toTemplate(t: TemplateDto): Template {
  const c = t.components
  return {
    ...t,
    components: {
      header: c.header,
      body: { text: c.body.text, examples: c.body.examples ?? [] },
      footer: c.footer,
      buttons: c.buttons ?? [],
    },
    warnings: t.warnings ?? [],
  }
}

export const templatesQueryOptions = (workspaceId: string, filter: TemplateFilter = {}) =>
  infiniteQueryOptions({
    queryKey: templatesKeys.list(workspaceId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listTemplates({
        headers: inWorkspace(workspaceId),
        query: {
          status: filter.status,
          category: filter.category,
          q: filter.q || undefined,
          limit: PAGE_SIZE,
          cursor: pageParam,
        },
        signal,
        throwOnError: true,
      })
      return {
        items: (data.items ?? []).map(toTemplate),
        counts: data.counts,
        next: data.next_cursor ?? undefined,
      }
    },
    getNextPageParam: (last) => last.next,
  })

export const templateQueryOptions = (workspaceId: string, templateId: string) =>
  queryOptions({
    queryKey: templatesKeys.detail(workspaceId, templateId),
    queryFn: async ({ signal }) => {
      const { data } = await getTemplate({
        headers: inWorkspace(workspaceId),
        path: { templateId },
        signal,
        throwOnError: true,
      })
      return toTemplate(data)
    },
  })
