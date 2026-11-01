import { Textarea } from '#/components/ui/textarea'
import { WELCOME_DELAYS } from '../../schemas/automations.schema'
import { ChatPreview, Check, controlClass, Fld, Select } from '../kit/field'
import type { FormProps } from './automation-drawer'

export function WelcomeForm({ settings, errors, onChange }: FormProps) {
  const s = settings.welcome
  if (!s) return null
  const set = (patch: Partial<typeof s>) => onChange({ welcome: { ...s, ...patch } })
  return (
    <>
      <Fld label="Welcome message" error={errors.message}>
        <Textarea
          unstyled
          className={controlClass}
          rows={4}
          value={s.message}
          onChange={(e) => set({ message: e.target.value })}
        />
      </Fld>
      <Fld label="When to send" error={errors.delay_minutes}>
        <Select
          value={String(s.delay_minutes)}
          onValueChange={(v) => set({ delay_minutes: Number(v) })}
          options={WELCOME_DELAYS.map((d) => ({ value: String(d.value), label: d.label }))}
        />
      </Fld>
      <Check checked={s.first_time_only} onChange={(first_time_only) => set({ first_time_only })}>
        Only send to first-time contacts
      </Check>
      <ChatPreview text={s.message.replaceAll('{{name}}', 'Ayesha')} />
    </>
  )
}
