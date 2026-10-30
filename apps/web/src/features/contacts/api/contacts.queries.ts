import {
  getContact,
  getContactImport,
  getContactsSummary,
  listContacts,
  listContactTags,
  listMembers,
} from '@relay/api-client'
import { keepPreviousData, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  Contact,
  ContactDetail,
  ContactFilter,
  ContactImport,
  ContactPage,
  ContactsSummary,
  Person,
  Tag,
} from '../types'
import { contactsKeys } from './contacts.keys'

export const PAGE_SIZE = 25

type RawContact = {
  tags: Contact['tags'] | null
  assigned_to: Person | null | undefined
}

function normalize<T extends RawContact>(c: T) {
  return { ...c, tags: c.tags ?? [], assigned_to: c.assigned_to ?? null }
}

export const contactsListQueryOptions = (
  workspaceId: string,
  filter: ContactFilter,
  cursor?: string,
) =>
  queryOptions({
    queryKey: contactsKeys.list(workspaceId, filter, cursor),
    queryFn: async ({ signal }): Promise<ContactPage> => {
      const { data } = await listContacts({
        headers: inWorkspace(workspaceId),
        query: { ...filter, cursor, limit: PAGE_SIZE },
        signal,
        throwOnError: true,
      })
      return {
        items: (data.items ?? []).map((c) => normalize(c)),
        next: data.next_cursor ?? null,
        total: data.total,
      }
    },
    placeholderData: keepPreviousData,
  })

export const contactsSummaryQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: contactsKeys.summary(workspaceId),
    queryFn: async ({ signal }): Promise<ContactsSummary> => {
      const { data } = await getContactsSummary({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data
    },
  })

export const contactQueryOptions = (workspaceId: string, contactId: string) =>
  queryOptions({
    queryKey: contactsKeys.detail(workspaceId, contactId),
    queryFn: async ({ signal }): Promise<ContactDetail> => {
      const { data } = await getContact({
        headers: inWorkspace(workspaceId),
        path: { contactId },
        signal,
        throwOnError: true,
      })
      return { ...normalize(data), attributes: data.attributes ?? {} }
    },
  })

export const contactTagsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: contactsKeys.tags(workspaceId),
    queryFn: async ({ signal }): Promise<Tag[]> => {
      const { data } = await listContactTags({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data ?? []
    },
  })

/** Active members a contact can be assigned to (first 100, by the API's order). */
export const contactAgentsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: contactsKeys.agents(workspaceId),
    queryFn: async ({ signal }): Promise<Person[]> => {
      const { data } = await listMembers({
        headers: inWorkspace(workspaceId),
        query: { status: 'active', page_size: 100 },
        signal,
        throwOnError: true,
      })
      return (data.members ?? []).map((m) => ({ id: m.user_id, name: m.name }))
    },
    staleTime: 5 * 60_000,
  })

const POLL_MS = 2_000

/** An import's status; polls every 2 s while the worker runs (AD-5). */
export const contactImportQueryOptions = (workspaceId: string, importId: string) =>
  queryOptions({
    queryKey: contactsKeys.import(workspaceId, importId),
    queryFn: async ({ signal }): Promise<ContactImport> => {
      const { data } = await getContactImport({
        headers: inWorkspace(workspaceId),
        path: { importId },
        signal,
        throwOnError: true,
      })
      return data
    },
    refetchInterval: (query) => {
      const status = query.state.data?.status
      return status === 'done' || status === 'failed' ? false : POLL_MS
    },
    staleTime: 0,
  })
