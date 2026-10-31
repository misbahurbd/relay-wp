import { Input } from '#/components/ui/input'
import { CART_DELAYS, CART_SOURCES, variableCount } from '../../schemas/automations.schema'
import { Check, controlClass, Fld, Select } from '../kit/field'
import { TemplateSelect, useApprovedTemplates } from '../kit/template-select'
import { fitVariables, VariableRows } from '../kit/variable-rows'
import type { FormProps } from './automation-drawer'

export function CartForm({ workspaceId, settings, errors, onChange }: FormProps) {
  const templates = useApprovedTemplates(workspaceId)
  const s = settings.cart
  if (!s) return null
  const set = (patch: Partial<typeof s>) => onChange({ cart: { ...s, ...patch } })
  const pick = (template_id: string | undefined) => {
    const t = templates.find((x) => x.id === template_id)
    const count = variableCount(t?.components.body.text ?? '')
    set({ template_id, variables: fitVariables(s.variables ?? [], count) })
  }
  return (
    <>
      <Fld label="Send reminder" error={errors.delay_minutes}>
        <Select
          value={s.delay_minutes}
          onChange={(e) => set({ delay_minutes: Number(e.target.value) })}
        >
          {CART_DELAYS.map((d) => (
            <option key={d.value} value={d.value}>
              {d.label}
            </option>
          ))}
        </Select>
      </Fld>
      <Fld label="Template" error={errors.template_id}>
        <TemplateSelect templates={templates} value={s.template_id} onChange={pick} />
      </Fld>
      <VariableRows
        variables={s.variables ?? []}
        sources={CART_SOURCES}
        errors={errors}
        onChange={(variables) => set({ variables })}
      />
      <Fld label="Discount code (optional)" error={errors.discount_code}>
        <Input
          unstyled
          className={controlClass}
          value={s.discount_code}
          onChange={(e) => set({ discount_code: e.target.value })}
        />
      </Fld>
      <Check checked={s.second_reminder} onChange={(second_reminder) => set({ second_reminder })}>
        Send a second reminder after 24 hours
      </Check>
      <div className="rounded-[10px] bg-warning-soft px-3.5 py-3 text-[13px]">
        Marketing template. Each reminder costs about $0.025 in Meta fees.
      </div>
    </>
  )
}
