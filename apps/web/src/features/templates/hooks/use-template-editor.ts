import type { UpdateInputBody } from '@relay/api-client'
import { revalidateLogic, useForm } from '@tanstack/react-form'
import { useQueryClient } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from 'sonner'
import { fieldError, focusFirstInvalid } from '#/components/form'
import { isApiError } from '#/lib/api'
import { templatesKeys } from '../api/templates.keys'
import {
  useCreateTemplate,
  useSubmitTemplate,
  useUpdateTemplate,
  useUploadHeaderSample,
} from '../api/templates.mutations'
import {
  emptyTemplateForm,
  formToInput,
  templateDraftSchema,
  templateIssues,
  templateToForm,
} from '../schemas/templates.schema'
import type { Template, TemplateFormValues } from '../types'

type Action = { action: 'draft' } | { action: 'submit' } | { action: 'upload'; file: File }

/** A sample picked in this session, so the preview can show the real image. */
export type LocalSample = { name: string; url?: string }

/** The API's field locations sometimes carry the request prefix ("body.components.body.text"). */
export function serverField(field: string) {
  return field
    .replace(/^body\.components\./, '')
    .replace(/^components\./, '')
    .replace(/^body\.(name|language|category)$/, '$1')
}

/**
 * The create/edit form: values and format checks in TanStack Form, plus the errors only a
 * submit or the server finds (completeness, name taken, per-field 422 details).
 * "Save draft" creates or updates; "Submit to Meta" saves, then submits (AD-8).
 */
export function useTemplateEditor({
  workspaceId,
  template,
}: {
  workspaceId: string
  template?: Template
}) {
  const navigate = useNavigate()
  const queryClient = useQueryClient()
  const create = useCreateTemplate(workspaceId)
  const update = useUpdateTemplate(workspaceId)
  const upload = useUploadHeaderSample(workspaceId)
  const submit = useSubmitTemplate(workspaceId)
  const formRef = useRef<HTMLFormElement>(null)

  // On the new page, a draft created by a submit or upload that then failed; later saves update it.
  const [created, setCreated] = useState<Template>()
  const current = template ?? created
  const [extra, setExtra] = useState<Record<string, string>>({})
  const [problem, setProblem] = useState<unknown>()
  const [busy, setBusy] = useState<Action['action'] | null>(null)
  const [localSample, setLocalSample] = useState<LocalSample>()

  useEffect(
    () => () => void (localSample?.url && URL.revokeObjectURL(localSample.url)),
    [localSample],
  )

  const showErrors = useCallback((errors: Record<string, string>) => {
    setExtra(errors)
    focusFirstInvalid(formRef.current)
  }, [])

  function fail(error: unknown) {
    if (isApiError(error)) {
      const details = (error.details ?? []).filter((d) => d.field)
      if (error.code === 'invalid_template' && details.length) {
        const map: Record<string, string> = {}
        for (const d of details) map[serverField(d.field ?? '')] ??= d.message
        showErrors(map)
        setProblem(error)
        return
      }
      if (error.code === 'template_exists') return showErrors({ name: error.message })
      if (error.code === 'invalid_sample') return showErrors({ header: error.message })
      if (error.code === 'template_in_review' || error.code === 'template_not_editable') {
        if (current) {
          void queryClient.invalidateQueries({
            queryKey: templatesKeys.detail(workspaceId, current.id),
          })
        }
      }
    }
    setProblem(error)
  }

  async function persist(value: TemplateFormValues) {
    const input = formToInput(value)
    if (!current) {
      const t = await create.mutateAsync(input)
      setCreated(t)
      return t
    }
    // Meta keys a submitted template by name and language, so they can't change (AD-8).
    const body: UpdateInputBody = current.meta_template_id
      ? { category: input.category, components: input.components }
      : input
    return update.mutateAsync({ templateId: current.id, body })
  }

  const form = useForm({
    defaultValues: current ? templateToForm(current) : emptyTemplateForm(),
    validators: { onDynamic: templateDraftSchema },
    validationLogic: revalidateLogic({ mode: 'blur', modeAfterSubmission: 'change' }),
    onSubmitMeta: { action: 'draft' } as Action,
    onSubmitInvalid: () => focusFirstInvalid(formRef.current),
    onSubmit: async ({ value, meta }) => {
      setProblem(undefined)
      if (meta.action === 'submit') {
        const issues = templateIssues(value, { complete: true, sample: current?.header_sample })
        if (issues.length) {
          const map: Record<string, string> = {}
          for (const i of issues) map[i.field] ??= i.message
          return showErrors(map)
        }
      }
      setBusy(meta.action)
      try {
        let saved: Template
        try {
          saved = await persist(value)
        } catch (e) {
          return fail(e)
        }
        setExtra({})

        if (meta.action === 'upload') {
          try {
            saved = await upload.mutateAsync({ templateId: saved.id, file: meta.file })
          } catch (e) {
            return fail(e)
          }
          setLocalSample({
            name: meta.file.name,
            url: meta.file.type.startsWith('image/') ? URL.createObjectURL(meta.file) : undefined,
          })
          toast.success('Sample uploaded')
        }

        if (meta.action === 'submit') {
          try {
            await submit.mutateAsync(saved.id)
          } catch (e) {
            return fail(e)
          }
          toast.success(`${saved.name} sent to Meta`, {
            description: 'Meta usually replies within 24 hours.',
          })
          return navigate({ to: '/templates' })
        }

        if (meta.action === 'draft') toast.success('Draft saved')
        if (!template) return goToEdit(saved.id)
      } finally {
        setBusy(null)
      }
    },
  })

  function goToEdit(templateId: string) {
    return navigate({
      to: '/templates/$templateId',
      params: { templateId },
      replace: true,
    })
  }

  /** Drops the submit/server errors for a field (and its children) once the user edits it. */
  const clearExtra = useCallback((...paths: string[]) => {
    setExtra((prev) => {
      const keys = Object.keys(prev).filter((k) =>
        paths.some((p) => k === p || k.startsWith(`${p}.`) || k.startsWith(`${p}[`)),
      )
      if (!keys.length) return prev
      const next = { ...prev }
      for (const k of keys) delete next[k]
      return next
    })
  }, [])

  const setExtraError = useCallback((field: string, message: string) => {
    setExtra((prev) => ({ ...prev, [field]: message }))
  }, [])

  /** The message to show under a field: its format error, else a submit or server one. */
  function errorOf(field: Parameters<typeof fieldError>[0] & { name: string }) {
    return fieldError(field) ?? extra[field.name]
  }

  return {
    form,
    formRef,
    current,
    extra,
    clearExtra,
    setExtraError,
    errorOf,
    problem,
    busy,
    localSample,
    run: (action: Action) => form.handleSubmit(action),
  }
}

export type TemplateEditorState = ReturnType<typeof useTemplateEditor>
export type TemplateForm = TemplateEditorState['form']
