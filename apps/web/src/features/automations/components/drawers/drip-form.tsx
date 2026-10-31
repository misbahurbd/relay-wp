import { useQuery } from '@tanstack/react-query'
import { Input } from '#/components/ui/input'
import { contactTagsQueryOptions } from '#/features/contacts'
import { variableCount } from '../../schemas/automations.schema'
import { AddRow, Check, controlClass, Fld, Select } from '../kit/field'
import { TemplateSelect, useApprovedTemplates } from '../kit/template-select'
import type { FormProps } from './automation-drawer'

export function DripForm({ workspaceId, settings, errors, onChange }: FormProps) {
  const tags = useQuery(contactTagsQueryOptions(workspaceId))
  // A step has nowhere to map variables, so only first-name templates qualify.
  const templates = useApprovedTemplates(workspaceId).filter(
    (t) => variableCount(t.components.body.text) <= 1,
  )
  const s = settings.drip
  if (!s) return null
  const steps = s.steps ?? []
  const set = (patch: Partial<typeof s>) => onChange({ drip: { ...s, ...patch } })
  const setStep = (i: number, patch: Partial<(typeof steps)[number]>) =>
    set({ steps: steps.map((st, n) => (n === i ? { ...st, ...patch } : st)) })
  return (
    <div className="flex flex-col gap-4">
      <Fld label="Who gets this sequence" error={errors.tag_id}>
        <Select
          value={s.tag_id ?? ''}
          onChange={(e) => set({ tag_id: e.target.value || undefined })}
        >
          <option value="">Choose a tag</option>
          {tags.data?.map((t) => (
            <option key={t.id} value={t.id}>
              Contacts tagged “{t.name}”
            </option>
          ))}
        </Select>
      </Fld>
      <div className="flex flex-col">
        {errors.steps && (
          <span className="pb-2 text-xs font-semibold text-destructive">{errors.steps}</span>
        )}
        {steps.map((st, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: steps are ordered and have no identity
          <div key={i} className="flex gap-3">
            <div className="flex flex-col items-center">
              <span className="flex size-[30px] items-center justify-center rounded-full bg-brand-soft text-xs font-extrabold text-brand-text">
                {i + 1}
              </span>
              <span className="min-h-4 w-0.5 flex-1 bg-border" />
            </div>
            <div className="flex flex-1 flex-wrap gap-2 pb-3.5">
              <Fld label="Day" error={errors[`steps[${i}].day`]} className="flex-[0_1_110px]">
                <Input
                  unstyled
                  type="number"
                  min={0}
                  className={controlClass}
                  value={st.day}
                  onChange={(e) => setStep(i, { day: Number(e.target.value) })}
                />
              </Fld>
              <Fld
                label="Template"
                error={errors[`steps[${i}].template_id`]}
                className="flex-[1_1_200px]"
              >
                <TemplateSelect
                  templates={templates}
                  value={st.template_id}
                  onChange={(template_id) => setStep(i, { template_id })}
                />
              </Fld>
            </div>
          </div>
        ))}
      </div>
      <AddRow
        onClick={() => set({ steps: [...steps, { day: (steps[steps.length - 1]?.day ?? 0) + 1 }] })}
      >
        + Add step
      </AddRow>
      <Check checked={s.stop_on_reply} onChange={(stop_on_reply) => set({ stop_on_reply })}>
        Stop the sequence when the contact replies
      </Check>
    </div>
  )
}
