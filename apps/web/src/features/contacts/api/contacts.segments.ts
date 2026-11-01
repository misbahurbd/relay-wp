import {
  createContactSegment,
  deleteContactSegment,
  listContactSegments,
  previewContactSegment,
  type SegmentBody,
  type SegmentDto,
  updateContactSegment,
} from '@relay/api-client'
import { keepPreviousData, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { Segment, SegmentPreview, SegmentRules } from '../types'
import { contactsKeys } from './contacts.keys'

/** Fills the lists the API may send as null. */
export function toSegment(s: SegmentDto): Segment {
  return {
    ...s,
    rules: {
      groups: (s.rules.groups ?? []).map((g) => ({
        match: g.match,
        conditions: (g.conditions ?? []).map((c) => ({ ...c, key: c.key ?? '' })),
      })),
    },
  }
}

/** The workspace's segments by name, each with who matches it now. */
export const segmentsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: contactsKeys.segments(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await listContactSegments({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return (data.items ?? []).map(toSegment)
    },
  })

/** Who unsaved rules match, as the builder changes. */
export const segmentPreviewQueryOptions = (workspaceId: string, rules: SegmentRules) =>
  queryOptions({
    queryKey: contactsKeys.segmentPreview(workspaceId, rules),
    queryFn: async ({ signal }): Promise<SegmentPreview> => {
      const { data } = await previewContactSegment({
        headers: inWorkspace(workspaceId),
        body: { rules },
        signal,
        throwOnError: true,
      })
      return { count: data.count, sample: data.sample ?? [] }
    },
    placeholderData: keepPreviousData,
    retry: false,
  })

export function useSaveSegment(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ id, body }: { id?: string; body: SegmentBody }) => {
      const { data } = id
        ? await updateContactSegment({
            headers: inWorkspace(workspaceId),
            path: { segmentId: id },
            body,
            throwOnError: true,
          })
        : await createContactSegment({
            headers: inWorkspace(workspaceId),
            body,
            throwOnError: true,
          })
      return toSegment(data)
    },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: contactsKeys.segments(workspaceId) }),
  })
}

export function useDeleteSegment(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (id: string) => {
      await deleteContactSegment({
        headers: inWorkspace(workspaceId),
        path: { segmentId: id },
        throwOnError: true,
      })
    },
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: contactsKeys.segments(workspaceId) }),
  })
}
