import { ORDER_SOURCES, ORDER_TRIGGERS, variableCount } from '../../schemas/automations.schema'
import { ChatPreview, Fld, Select } from '../kit/field'
import { TemplateSelect, useApprovedTemplates } from '../kit/template-select'
import { fitVariables, VariableRows } from '../kit/variable-rows'
import type { FormProps } from './automation-drawer'

const SAMPLE: Record<string, string> = {
  customer_first_name: 'Daniel',
  order_number: '#4821',
  order_total: '$86.50',
  tracking_url: 'grnlf.co/t/4821',
}

export function OrderForm({ workspaceId, settings, errors, onChange }: FormProps) {
  const templates = useApprovedTemplates(workspaceId)
  const s = settings.order
  if (!s) return null
  const set = (patch: Partial<typeof s>) => onChange({ order: { ...s, ...patch } })
  const template = templates.find((t) => t.id === s.template_id)
  const pick = (template_id: string | undefined) => {
    const t = templates.find((x) => x.id === template_id)
    const count = t ? variableCount(t.components.body.text) : (s.variables?.length ?? 0)
    set({ template_id, variables: fitVariables(s.variables ?? [], count) })
  }
  const preview = template?.components.body.text.replace(/\{\{(\d+)\}\}/g, (_, n) => {
    const v = s.variables?.[Number(n) - 1]
    return v?.source === 'custom' ? v.custom : (SAMPLE[v?.source ?? ''] ?? '')
  })
  return (
    <>
      <Fld label="Trigger" error={errors.trigger}>
        <Select
          value={s.trigger}
          onValueChange={(trigger) => set({ trigger })}
          options={ORDER_TRIGGERS.map((t) => ({ value: t.value, label: t.label }))}
        />
      </Fld>
      <Fld label="Template" error={errors.template_id}>
        <TemplateSelect templates={templates} value={s.template_id} onChange={pick} />
      </Fld>
      <VariableRows
        variables={s.variables ?? []}
        sources={ORDER_SOURCES}
        errors={errors}
        onChange={(variables) => set({ variables })}
      />
      {preview && <ChatPreview text={preview} />}
    </>
  )
}
