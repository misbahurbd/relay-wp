import { useStore } from '@tanstack/react-form'
import { useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import { ChevronLeftIcon, SendIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { useAccess } from '#/features/team'
import { errorMessage, isApiError } from '#/lib/api'
import { useDeleteTemplate } from '../../api/templates.mutations'
import { templateQueryOptions } from '../../api/templates.queries'
import { type TemplateEditorState, useTemplateEditor } from '../../hooks/use-template-editor'
import { categoryLabel, isFrozen, languageLabel } from '../../schemas/templates.schema'
import type { Template } from '../../types'
import { ActionError } from '../kit/action-error'
import { DeleteTemplateDialog } from '../kit/delete-template-dialog'
import { StatusBadge } from '../kit/status-badge'
import { ApprovalTips } from '../preview/approval-tips'
import { TemplatePreview } from '../preview/template-preview'
import { BasicsSection } from './basics-section'
import { BodySection } from './body-section'
import { ButtonsSection } from './buttons-section'
import { FooterSection } from './footer-section'
import { HeaderSection } from './header-section'
import { StatusNotice } from './status-notice'

type Props = { workspaceId: string; templateId?: string; businessName: string }

/** Create (no templateId) or edit a template, with the live phone preview beside the form. */
export function TemplateEditor({ workspaceId, templateId, businessName }: Props) {
  const query = useQuery({
    ...templateQueryOptions(workspaceId, templateId ?? ''),
    enabled: !!templateId,
  })

  if (!templateId) return <EditorForm workspaceId={workspaceId} businessName={businessName} />
  if (query.isPending) return <EditorSkeleton />
  if (query.isError) {
    const missing = isApiError(query.error) && query.error.code === 'template_not_found'
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col items-start gap-3 p-4 sm:p-7">
        <BackLink workspaceId={workspaceId} />
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">
          {missing ? 'Template not found' : 'Couldn’t load this template'}
        </h1>
        <p className="text-muted-foreground">
          {missing
            ? 'It may have been deleted, or it belongs to another workspace.'
            : errorMessage(query.error)}
        </p>
        {!missing && (
          <Button variant="outline" onClick={() => query.refetch()}>
            Try again
          </Button>
        )}
      </main>
    )
  }
  return <EditorForm workspaceId={workspaceId} template={query.data} businessName={businessName} />
}

function EditorForm({
  workspaceId,
  template,
  businessName,
}: {
  workspaceId: string
  template?: Template
  businessName: string
}) {
  const navigate = useNavigate()
  const { access, can } = useAccess(workspaceId)
  const editor = useTemplateEditor({ workspaceId, template })
  const remove = useDeleteTemplate(workspaceId)
  const [deleting, setDeleting] = useState(false)
  const current = editor.current

  // While access loads, keep the form usable; the API checks every request anyway.
  const mayCreate = !access || can('templates.create')
  const frozen = !!current && isFrozen(current)
  const readOnly = frozen || !mayCreate
  const showActions = !readOnly && !!access
  const busy = editor.busy

  const title = !current
    ? 'Create template'
    : readOnly
      ? current.name
      : current.status === 'rejected'
        ? 'Edit and resubmit'
        : 'Edit template'

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1">
          <BackLink workspaceId={workspaceId} />
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-[28px] font-extrabold tracking-[-0.02em] break-all">{title}</h1>
            {current && <StatusBadge status={current.status} />}
          </div>
          <p className="max-w-prose text-[15px] text-muted-foreground">
            {readOnly
              ? 'You can look at this template, but not change it right now.'
              : 'Fill in the form. The phone on the right shows exactly what your customer will see.'}
          </p>
        </div>
        <div className="flex flex-wrap gap-2.5">
          {current && can('templates.delete') && (
            <Button
              variant="outline"
              className="text-destructive"
              onClick={() => setDeleting(true)}
              disabled={!!busy}
            >
              <Trash2Icon aria-hidden="true" />
              Delete
            </Button>
          )}
          {showActions && (
            <Button variant="outline" type="submit" form="template-form" disabled={!!busy}>
              {busy === 'draft' ? 'Saving…' : 'Save draft'}
            </Button>
          )}
          {showActions && can('templates.submit') && (
            <Button disabled={!!busy} onClick={() => editor.run({ action: 'submit' })}>
              <SendIcon aria-hidden="true" />
              {busy === 'submit' ? 'Submitting…' : 'Submit to Meta'}
            </Button>
          )}
        </div>
      </div>

      {current && <StatusNotice template={current} canEdit={!readOnly} />}
      {editor.problem !== undefined && (
        <ActionError workspaceId={workspaceId} error={editor.problem} />
      )}

      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(320px,380px)]">
        <form
          id="template-form"
          ref={editor.formRef}
          noValidate
          onSubmit={(e) => {
            e.preventDefault()
            if (!readOnly) void editor.run({ action: 'draft' })
          }}
        >
          <fieldset disabled={readOnly} className="flex min-w-0 flex-col gap-4">
            <legend className="sr-only">Template</legend>
            <BasicsSection editor={editor} locked={!!current?.meta_template_id} />
            <HeaderSection editor={editor} />
            <BodySection editor={editor} />
            <FooterSection editor={editor} />
            <ButtonsSection editor={editor} />
          </fieldset>
        </form>
        <PreviewColumn editor={editor} businessName={businessName} />
      </div>

      {deleting && current && (
        <DeleteTemplateDialog
          template={current}
          pending={remove.isPending}
          error={remove.error}
          onClose={() => {
            remove.reset()
            setDeleting(false)
          }}
          onConfirm={() =>
            remove.mutate(current.id, {
              onSuccess: () => {
                toast.success(`${current.name} deleted`)
                void navigate({ to: '/w/$workspaceId/templates', params: { workspaceId } })
              },
            })
          }
        />
      )}
    </main>
  )
}

function PreviewColumn({
  editor,
  businessName,
}: {
  editor: TemplateEditorState
  businessName: string
}) {
  const values = useStore(editor.form.store, (s) => s.values)
  return (
    <div className="flex flex-col gap-4 lg:sticky lg:top-4">
      <div className="flex items-center justify-between gap-2">
        <h2 className="text-base font-extrabold">Live preview</h2>
        <span className="rounded-full bg-neutral-soft px-2.5 py-0.5 text-xs font-bold text-neutral">
          {categoryLabel(values.category)} · {languageLabel(values.language)}
        </span>
      </div>
      <TemplatePreview
        values={values}
        sample={editor.current?.header_sample}
        localSample={editor.localSample}
        businessName={businessName}
      />
      <ApprovalTips />
    </div>
  )
}

function BackLink({ workspaceId }: { workspaceId: string }) {
  return (
    <Link
      to="/w/$workspaceId/templates"
      params={{ workspaceId }}
      className="inline-flex w-fit items-center gap-1 text-[13px] font-bold text-brand-text no-underline hover:text-foreground"
    >
      <ChevronLeftIcon className="size-4" aria-hidden="true" />
      Back to templates
    </Link>
  )
}

function EditorSkeleton() {
  return (
    <main
      aria-busy="true"
      aria-label="Loading template"
      className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7"
    >
      <Skeleton className="h-4 w-36" />
      <Skeleton className="h-9 w-72" />
      <div className="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(320px,380px)]">
        <div className="flex flex-col gap-4">
          <Skeleton className="h-56 w-full rounded-xl" />
          <Skeleton className="h-32 w-full rounded-xl" />
          <Skeleton className="h-64 w-full rounded-xl" />
        </div>
        <Skeleton className="h-[520px] w-full rounded-[28px]" />
      </div>
    </main>
  )
}
