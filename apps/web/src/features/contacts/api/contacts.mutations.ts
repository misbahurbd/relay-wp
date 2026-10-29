import {
  bulkContacts,
  createContact,
  createContactTag,
  deleteContact,
  previewContactImport,
  setContactImportMapping,
  startContactImport,
  updateContact,
  uploadContactImport,
} from '@relay/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  BulkInput,
  ColumnMapping,
  ContactImport,
  ContactInput,
  ContactPatch,
  ImportPreview,
  ImportUpload,
  Tag,
} from '../types'
import { contactsKeys, invalidateContacts } from './contacts.keys'

/** A contacts change can move counts, tags and lists, so it refreshes the workspace's contacts. */
function useContactsMutation<TInput, TOutput>(
  workspaceId: string,
  fn: (input: TInput) => Promise<TOutput>,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => invalidateContacts(queryClient, workspaceId),
  })
}

// ---------- contacts ----------

export function useCreateContact(workspaceId: string) {
  return useContactsMutation(workspaceId, async (body: ContactInput) => {
    const { data } = await createContact({
      headers: inWorkspace(workspaceId),
      body,
      throwOnError: true,
    })
    return data
  })
}

export function useUpdateContact(workspaceId: string) {
  return useContactsMutation(
    workspaceId,
    async ({ contactId, body }: { contactId: string; body: ContactPatch }) => {
      const { data } = await updateContact({
        headers: inWorkspace(workspaceId),
        path: { contactId },
        body,
        throwOnError: true,
      })
      return data
    },
  )
}

export function useDeleteContact(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (contactId: string) => {
      await deleteContact({
        headers: inWorkspace(workspaceId),
        path: { contactId },
        throwOnError: true,
      })
      return contactId
    },
    onSuccess: (contactId) => {
      queryClient.removeQueries({ queryKey: contactsKeys.detail(workspaceId, contactId) })
      return invalidateContacts(queryClient, workspaceId)
    },
  })
}

export function useBulkContacts(workspaceId: string) {
  return useContactsMutation(workspaceId, async (body: BulkInput) => {
    const { data } = await bulkContacts({
      headers: inWorkspace(workspaceId),
      body,
      throwOnError: true,
    })
    return data.affected
  })
}

// ---------- tags ----------

/** Creates a tag, or returns the existing one with that name. */
export function useCreateTag(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (name: string): Promise<Tag> => {
      const { data } = await createContactTag({
        headers: inWorkspace(workspaceId),
        body: { name },
        throwOnError: true,
      })
      return data
    },
    onSuccess: () => queryClient.invalidateQueries({ queryKey: contactsKeys.tags(workspaceId) }),
  })
}

// ---------- import (AD-5) ----------

export function useUploadImport(workspaceId: string) {
  return useMutation({
    mutationFn: async (file: File): Promise<ImportUpload> => {
      const { data } = await uploadContactImport({
        headers: inWorkspace(workspaceId),
        body: { file },
        throwOnError: true,
      })
      return {
        ...data,
        columns: (data.columns ?? []).map((c) => ({ ...c, samples: c.samples ?? [] })),
      }
    },
  })
}

/** Saves the mapping, then reads the preview the server counts from the whole file. */
export function useMapImport(workspaceId: string) {
  return useMutation({
    mutationFn: async ({
      importId,
      columns,
      tagName,
    }: {
      importId: string
      columns: ColumnMapping[]
      tagName?: string
    }): Promise<ImportPreview> => {
      await setContactImportMapping({
        headers: inWorkspace(workspaceId),
        path: { importId },
        body: { columns, tag_name: tagName || undefined, opt_in_confirmed: true },
        throwOnError: true,
      })
      const { data } = await previewContactImport({
        headers: inWorkspace(workspaceId),
        path: { importId },
        throwOnError: true,
      })
      return { ...data, sample: data.sample ?? [] }
    },
  })
}

export function useStartImport(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (importId: string): Promise<ContactImport> => {
      const { data } = await startContactImport({
        headers: inWorkspace(workspaceId),
        path: { importId },
        throwOnError: true,
      })
      return data
    },
    onSuccess: (data) => queryClient.setQueryData(contactsKeys.import(workspaceId, data.id), data),
  })
}
