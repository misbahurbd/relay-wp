import { useQuery } from '@tanstack/react-query'
import { useCreateTag } from '../api/contacts.mutations'
import { contactTagsQueryOptions } from '../api/contacts.queries'
import type { TagRef } from '../types'

/** The workspace's tags, plus a create that returns the new (or existing) tag. */
export function useTags(workspaceId: string) {
  const { data: tags = [] } = useQuery(contactTagsQueryOptions(workspaceId))
  const createTag = useCreateTag(workspaceId)
  const create = async (name: string): Promise<TagRef> => {
    const tag = await createTag.mutateAsync(name)
    return { id: tag.id, name: tag.name }
  }
  return { tags, create }
}
