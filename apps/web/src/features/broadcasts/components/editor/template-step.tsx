import { Link } from '@tanstack/react-router'
import { useId } from 'react'
import { Badge } from '#/components/ui/badge'
import { RadioGroup } from '#/components/ui/radio-group'
import { Skeleton } from '#/components/ui/skeleton'
import type { Template } from '#/features/templates'
import { fitVariables, templateVariables } from '../../schemas/broadcasts.schema'
import type { BroadcastForm } from '../../types'
import { BroadcastPreview } from '../kit/broadcast-preview'
import { StepHeading } from './audience-step'
import { OptionCard } from './option-card'
import { VariablesSection } from './variables-section'

const CATEGORY: Record<string, string> = {
  marketing: 'Marketing',
  utility: 'Utility',
  authentication: 'Authentication',
}

/** Step 2: an approved template, its blanks, and the message as customers see it. */
export function TemplateStep({
  templates,
  loading,
  form,
  template,
  readOnly,
  errors,
  senderName,
  onChange,
}: {
  templates: Template[]
  loading: boolean
  form: BroadcastForm
  template?: Template
  readOnly: boolean
  errors: Record<string, string>
  /** The sending number's verified name, shown at the top of the preview chat. */
  senderName?: string
  onChange: (f: BroadcastForm) => void
}) {
  const id = useId()
  return (
    <>
      <StepHeading title="Choose a template">
        Only Meta-approved templates can start a conversation. Fill in the blanks below.
      </StepHeading>
      <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,320px)]">
        <div className="flex min-w-0 flex-col gap-2.5">
          {loading ? (
            <Skeleton className="h-16 w-full" />
          ) : templates.length === 0 ? (
            <p className="text-[13px] text-muted-foreground">
              No approved templates yet.{' '}
              <Link to="/templates/new" className="font-bold text-brand-text">
                Create one
              </Link>{' '}
              and send it to Meta for review.
            </p>
          ) : (
            <RadioGroup
              value={form.template_id ?? ''}
              disabled={readOnly}
              aria-label="Template"
              className="gap-2.5"
              onValueChange={(v) => {
                const next = templates.find((t) => t.id === v)
                onChange(
                  fitVariables(
                    { ...form, template_id: next?.id, header: undefined, body: [] },
                    next,
                  ),
                )
              }}
            >
              {templates.map((t) => {
                const v = templateVariables(t)
                const count = v.body + (v.header ? 1 : 0)
                return (
                  <OptionCard
                    key={t.id}
                    id={`${id}-${t.id}`}
                    value={t.id}
                    selected={t.id === form.template_id}
                  >
                    <span className="flex flex-wrap items-center gap-2">
                      <span className="min-w-0 flex-1 truncate font-bold">{t.name}</span>
                      <Badge variant={t.category === 'marketing' ? 'success' : 'info'}>
                        {CATEGORY[t.category] ?? t.category}
                      </Badge>
                      <Badge variant="success">Approved</Badge>
                    </span>
                    <span className="text-xs text-muted-foreground">
                      {t.language} · {count} {count === 1 ? 'variable' : 'variables'}
                    </span>
                  </OptionCard>
                )
              })}
            </RadioGroup>
          )}
          {errors.template_id && (
            <span className="text-xs font-semibold text-destructive">{errors.template_id}</span>
          )}
          {template && (
            <>
              <h3 className="mt-2.5 text-sm font-bold">Variables</h3>
              <VariablesSection
                form={form}
                readOnly={readOnly}
                errors={errors}
                onChange={(v) => onChange({ ...form, ...v })}
              />
            </>
          )}
        </div>
        <BroadcastPreview template={template} form={form} senderName={senderName} />
      </div>
    </>
  )
}
