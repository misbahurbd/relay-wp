import {
  getIntegrations,
  listApiKeys,
  listWebhookDeliveries,
  listWebhookEvents,
  listWebhooks,
} from '@relay/api-client'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { Endpoint, Overview } from '../types'
import { integrationsKeys } from './integrations.keys'

export const overviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: integrationsKeys.overview(workspaceId),
    queryFn: async ({ signal }): Promise<Overview> => {
      const { data } = await getIntegrations({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return { ...data, apps: data.apps ?? [], connections: data.connections ?? [] }
    },
  })

export const apiKeysQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: integrationsKeys.apiKeys(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await listApiKeys({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data.keys ?? []
    },
  })

export const webhooksQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: integrationsKeys.webhooks(workspaceId),
    queryFn: async ({ signal }): Promise<Endpoint[]> => {
      const { data } = await listWebhooks({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return (data.endpoints ?? []).map((e) => ({ ...e, events: e.events ?? [] }))
    },
  })

export const eventsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: integrationsKeys.events(),
    queryFn: async ({ signal }) => {
      const { data } = await listWebhookEvents({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data.events ?? []
    },
    staleTime: Infinity,
  })

export const deliveriesQueryOptions = (workspaceId: string, endpointId: string) =>
  infiniteQueryOptions({
    queryKey: integrationsKeys.deliveries(workspaceId, endpointId),
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listWebhookDeliveries({
        headers: inWorkspace(workspaceId),
        path: { endpointId },
        query: { cursor: pageParam, limit: 25 },
        signal,
        throwOnError: true,
      })
      return { deliveries: data.deliveries ?? [], next: data.next_cursor ?? undefined }
    },
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last) => last.next,
  })
