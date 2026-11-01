import {
  type BroadcastDto,
  getBroadcast,
  listBroadcastRecipients,
  listBroadcasts,
  listWhatsappNumbers,
  previewBroadcastAudience,
} from '@relay/api-client'
import { infiniteQueryOptions, keepPreviousData, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  Audience,
  AudienceCount,
  Broadcast,
  BroadcastFilter,
  RecipientFilter,
  Variable,
} from '../types'
import { broadcastsKeys } from './broadcasts.keys'

const PAGE_SIZE = 25

const toVariable = (v: { source: Variable['source']; value?: string; fallback?: string }) => ({
  source: v.source,
  value: v.value ?? '',
  fallback: v.fallback ?? '',
})

/** Fills the lists the API may send as null, so components never check for it. */
export function toBroadcast(b: BroadcastDto): Broadcast {
  return {
    ...b,
    audience: {
      all: b.audience.all ?? false,
      tag_ids: b.audience.tag_ids ?? [],
      segment_ids: b.audience.segment_ids ?? [],
      skip_recent_marketing: b.audience.skip_recent_marketing ?? false,
    },
    variables: {
      header: b.variables.header ? toVariable(b.variables.header) : undefined,
      body: (b.variables.body ?? []).map(toVariable),
    },
    failures: b.failures ?? [],
  }
}

export const broadcastsQueryOptions = (workspaceId: string, filter: BroadcastFilter = {}) =>
  infiniteQueryOptions({
    queryKey: broadcastsKeys.list(workspaceId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listBroadcasts({
        headers: inWorkspace(workspaceId),
        query: {
          status: filter.status,
          q: filter.q || undefined,
          limit: PAGE_SIZE,
          cursor: pageParam,
        },
        signal,
        throwOnError: true,
      })
      return { items: data.items ?? [], next: data.next_cursor ?? undefined }
    },
    getNextPageParam: (last) => last.next,
    // A sending broadcast's counts move; keep the list fresh while one is.
    refetchInterval: (q) =>
      q.state.data?.pages.some((p) => p.items.some((b) => b.status === 'sending')) ? 10_000 : false,
  })

export const broadcastQueryOptions = (workspaceId: string, broadcastId: string) =>
  queryOptions({
    queryKey: broadcastsKeys.detail(workspaceId, broadcastId),
    queryFn: async ({ signal }) => {
      const { data } = await getBroadcast({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        signal,
        throwOnError: true,
      })
      return toBroadcast(data)
    },
    // Progress is polled while it sends (ADR 0013, AD-8).
    refetchInterval: (q) => (q.state.data?.status === 'sending' ? 5_000 : false),
  })

export const recipientsQueryOptions = (
  workspaceId: string,
  broadcastId: string,
  filter?: RecipientFilter,
) =>
  infiniteQueryOptions({
    queryKey: broadcastsKeys.recipients(workspaceId, broadcastId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listBroadcastRecipients({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        query: { filter, limit: 50, cursor: pageParam },
        signal,
        throwOnError: true,
      })
      return { items: data.items ?? [], next: data.next_cursor ?? undefined }
    },
    getNextPageParam: (last) => last.next,
  })

/** Who an audience reaches, who it leaves out, and the number's daily tier. */
export const audienceQueryOptions = (workspaceId: string, audience: Audience, numberId?: string) =>
  queryOptions({
    queryKey: broadcastsKeys.audience(workspaceId, audience, numberId),
    queryFn: async ({ signal }): Promise<AudienceCount> => {
      const { data } = await previewBroadcastAudience({
        headers: inWorkspace(workspaceId),
        body: { ...audience, number_id: numberId },
        signal,
        throwOnError: true,
      })
      return data
    },
    placeholderData: keepPreviousData,
  })

/** The workspace's connected WhatsApp numbers: the ones a broadcast can send from. */
export const sendingNumbersQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: broadcastsKeys.numbers(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await listWhatsappNumbers({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return (data.numbers ?? []).filter((n) => n.status === 'connected')
    },
  })
