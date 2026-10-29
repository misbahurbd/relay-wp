import {
  type CreateInputBody1,
  createTemplate,
  deleteTemplate,
  submitTemplate,
  syncTemplates,
  type UpdateInputBody,
  updateTemplate,
  uploadTemplateHeaderSample,
} from '@relay/api-client'
import { type QueryClient, useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { Template } from '../types'
import { invalidateTemplateLists, templatesKeys } from './templates.keys'
import { toTemplate } from './templates.queries'

/** Stores the template the API returned and refreshes the lists, whose counts may have moved. */
function remember(queryClient: QueryClient, workspaceId: string, template: Template) {
  queryClient.setQueryData(templatesKeys.detail(workspaceId, template.id), template)
  return invalidateTemplateLists(queryClient, workspaceId)
}

export function useCreateTemplate(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: CreateInputBody1) => {
      const { data } = await createTemplate({
        headers: inWorkspace(workspaceId),
        body,
        throwOnError: true,
      })
      return toTemplate(data)
    },
    onSuccess: (t) => remember(queryClient, workspaceId, t),
  })
}

export function useUpdateTemplate(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ templateId, body }: { templateId: string; body: UpdateInputBody }) => {
      const { data } = await updateTemplate({
        headers: inWorkspace(workspaceId),
        path: { templateId },
        body,
        throwOnError: true,
      })
      return toTemplate(data)
    },
    onSuccess: (t) => remember(queryClient, workspaceId, t),
  })
}

export function useUploadHeaderSample(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async ({ templateId, file }: { templateId: string; file: File }) => {
      const { data } = await uploadTemplateHeaderSample({
        headers: inWorkspace(workspaceId),
        path: { templateId },
        body: { file },
        throwOnError: true,
      })
      return toTemplate(data)
    },
    onSuccess: (t) => remember(queryClient, workspaceId, t),
  })
}

export function useSubmitTemplate(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (templateId: string) => {
      const { data } = await submitTemplate({
        headers: inWorkspace(workspaceId),
        path: { templateId },
        throwOnError: true,
      })
      return toTemplate(data)
    },
    onSuccess: (t) => remember(queryClient, workspaceId, t),
  })
}

export function useDeleteTemplate(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (templateId: string) => {
      await deleteTemplate({
        headers: inWorkspace(workspaceId),
        path: { templateId },
        throwOnError: true,
      })
      return templateId
    },
    onSuccess: (templateId) => {
      queryClient.removeQueries({ queryKey: templatesKeys.detail(workspaceId, templateId) })
      return invalidateTemplateLists(queryClient, workspaceId)
    },
  })
}

export function useSyncTemplates(workspaceId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async () => {
      const { data } = await syncTemplates({
        headers: inWorkspace(workspaceId),
        throwOnError: true,
      })
      return data.synced
    },
    // Synced templates can overwrite ones already open, so refresh every templates query.
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: templatesKeys.workspace(workspaceId) }),
  })
}
