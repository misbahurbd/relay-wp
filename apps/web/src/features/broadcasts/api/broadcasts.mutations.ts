import {
  type BroadcastBody,
  type ColumnMappingDto,
  cancelBroadcast,
  createBroadcast,
  deleteBroadcast,
  duplicateBroadcast,
  getContactImport,
  previewContactImport,
  sendBroadcast,
  setContactImportMapping,
  startContactImport,
  updateBroadcast,
  uploadContactImport,
} from '@relay/api-client'
import { type QueryClient, useMutation, useQueryClient } from '@tanstack/react-query'
import { contactsKeys } from '#/features/contacts'
import { inWorkspace } from '#/lib/api'
import type { Broadcast } from '../types'
import { broadcastsKeys, invalidateBroadcasts } from './broadcasts.keys'
import { toBroadcast } from './broadcasts.queries'

/** Stores the broadcast the API returned and refreshes the lists and meters. */
function remember(queryClient: QueryClient, workspaceId: string, b: Broadcast) {
  queryClient.setQueryData(broadcastsKeys.detail(workspaceId, b.id), b)
  return invalidateBroadcasts(queryClient, workspaceId)
}

export function useCreateBroadcast(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: BroadcastBody) => {
      const { data } = await createBroadcast({
        headers: inWorkspace(workspaceId),
        body,
        throwOnError: true,
      })
      return toBroadcast(data)
    },
    onSuccess: (b) => remember(queryClient, workspaceId, b),
  })
}

export function useUpdateBroadcast(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ broadcastId, body }: { broadcastId: string; body: BroadcastBody }) => {
      const { data } = await updateBroadcast({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        body,
        throwOnError: true,
      })
      return toBroadcast(data)
    },
    onSuccess: (b) => remember(queryClient, workspaceId, b),
  })
}

export function useSendBroadcast(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({
      broadcastId,
      scheduledAt,
    }: {
      broadcastId: string
      scheduledAt?: string
    }) => {
      const { data } = await sendBroadcast({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        body: { scheduled_at: scheduledAt },
        throwOnError: true,
      })
      return toBroadcast(data)
    },
    onSuccess: (b) => remember(queryClient, workspaceId, b),
  })
}

export function useCancelBroadcast(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (broadcastId: string) => {
      const { data } = await cancelBroadcast({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        throwOnError: true,
      })
      return toBroadcast(data)
    },
    onSuccess: (b) => remember(queryClient, workspaceId, b),
  })
}

export function useDuplicateBroadcast(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (broadcastId: string) => {
      const { data } = await duplicateBroadcast({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        throwOnError: true,
      })
      return toBroadcast(data)
    },
    onSuccess: (b) => remember(queryClient, workspaceId, b),
  })
}

export function useDeleteBroadcast(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (broadcastId: string) => {
      await deleteBroadcast({
        headers: inWorkspace(workspaceId),
        path: { broadcastId },
        throwOnError: true,
      })
      return broadcastId
    },
    onSuccess: (id) => {
      queryClient.removeQueries({ queryKey: broadcastsKeys.detail(workspaceId, id) })
      return invalidateBroadcasts(queryClient, workspaceId)
    },
  })
}

// ---------- Uploaded lists (BC-7) ----------
// A list goes through the Contacts import, which checks phones, consent and the plan's contact
// limit, and tags every row; the broadcast then targets that tag.

export type ListUpload = {
  importId: string
  fileName: string
  columns: { name: string; guess: string }[]
}

export type ListPreview = { rowsFound: number; ready: number; updates: number; invalid: number }

export function useUploadList(workspaceId: string) {
  return useMutation({
    mutationFn: async (file: File): Promise<ListUpload> => {
      const { data } = await uploadContactImport({
        headers: inWorkspace(workspaceId),
        body: { file },
        throwOnError: true,
      })
      return {
        importId: data.id,
        fileName: data.file_name,
        columns: (data.columns ?? []).map((c) => ({ name: c.name, guess: c.guess })),
      }
    },
  })
}

/** Maps the phone (and name) column, tags every row, and counts what will be imported. */
export function useMapList(workspaceId: string) {
  return useMutation({
    mutationFn: async ({
      upload,
      phone,
      tagName,
    }: {
      upload: ListUpload
      phone: string
      tagName: string
    }): Promise<ListPreview> => {
      const columns = upload.columns.map((c) => ({
        name: c.name,
        field: (c.name === phone
          ? 'phone'
          : c.guess === 'name'
            ? 'name'
            : 'skip') as ColumnMappingDto['field'],
      }))
      await setContactImportMapping({
        headers: inWorkspace(workspaceId),
        path: { importId: upload.importId },
        body: { columns, tag_name: tagName, opt_in_confirmed: true },
        throwOnError: true,
      })
      const { data } = await previewContactImport({
        headers: inWorkspace(workspaceId),
        path: { importId: upload.importId },
        throwOnError: true,
      })
      return {
        rowsFound: data.rows_found,
        ready: data.ready,
        updates: data.updates,
        invalid: data.invalid,
      }
    },
  })
}

const sleep = (ms: number) => new Promise((r) => setTimeout(r, ms))

/** Runs the import and waits for it, then refreshes tags so the list's tag can be picked. */
export function useImportList(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (importId: string) => {
      await startContactImport({
        headers: inWorkspace(workspaceId),
        path: { importId },
        throwOnError: true,
      })
      for (let i = 0; i < 300; i++) {
        await sleep(1500)
        const { data } = await getContactImport({
          headers: inWorkspace(workspaceId),
          path: { importId },
          throwOnError: true,
        })
        if (data.status === 'failed') throw new Error(data.error ?? 'The import failed')
        if (data.status === 'done') {
          await queryClient.invalidateQueries({ queryKey: contactsKeys.workspace(workspaceId) })
          return { created: data.created, updated: data.updated, skipped: data.skipped }
        }
      }
      throw new Error('The import is taking long; check Contacts in a few minutes.')
    },
  })
}
