import { useEffect } from 'react'
import { Input } from '#/components/ui/input'
import { Textarea } from '#/components/ui/textarea'
import { DAY_NAMES, timezones } from '../../schemas/automations.schema'
import { controlClass, Fld, Select } from '../kit/field'
import { Toggle } from '../kit/toggle'
import type { FormProps } from './automation-drawer'

export function AwayForm({ settings, errors, onChange, firstSave }: FormProps) {
  const s = settings.away
  // biome-ignore lint/correctness/useExhaustiveDependencies: only the first open picks a timezone
  useEffect(() => {
    if (s && firstSave) {
      onChange({ away: { ...s, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone } })
    }
  }, [])
  if (!s) return null
  const hours = s.hours ?? []
  const set = (patch: Partial<typeof s>) => onChange({ away: { ...s, ...patch } })
  const setDay = (i: number, patch: Partial<(typeof hours)[number]>) =>
    set({ hours: hours.map((d, n) => (n === i ? { ...d, ...patch } : d)) })
  return (
    <>
      <Fld label="Timezone" error={errors.timezone}>
        <Select value={s.timezone} onChange={(e) => set({ timezone: e.target.value })}>
          {timezones(s.timezone).map((z) => (
            <option key={z} value={z}>
              {z}
            </option>
          ))}
        </Select>
      </Fld>
      <div className="flex flex-col gap-2">
        <span className="text-[13px] font-bold">Working hours</span>
        {hours.map((d, i) => {
          const day = DAY_NAMES[d.day] ?? { short: d.day, name: d.day }
          const error = errors[`hours[${i}]`]
          return (
            <div key={d.day} className="flex flex-col gap-1">
              <div className="flex flex-wrap items-center gap-2.5 rounded-[10px] border px-2.5 py-2">
                <Toggle
                  small
                  on={d.open}
                  onChange={(open) => setDay(i, { open })}
                  label={`${day.name} open`}
                />
                <span className="w-11 font-bold">{day.short}</span>
                {d.open ? (
                  <span className="flex flex-1 items-center gap-2">
                    <Input
                      unstyled
                      type="time"
                      aria-label={`${day.name} from`}
                      className={`${controlClass} min-h-9 w-[120px] px-2 py-1.5`}
                      value={d.from}
                      onChange={(e) => setDay(i, { from: e.target.value })}
                    />
                    <span className="text-muted-foreground">to</span>
                    <Input
                      unstyled
                      type="time"
                      aria-label={`${day.name} to`}
                      className={`${controlClass} min-h-9 w-[120px] px-2 py-1.5`}
                      value={d.to}
                      onChange={(e) => setDay(i, { to: e.target.value })}
                    />
                  </span>
                ) : (
                  <span className="flex-1 text-muted-foreground">Closed all day</span>
                )}
              </div>
              {error && <span className="text-xs font-semibold text-destructive">{error}</span>}
            </div>
          )
        })}
      </div>
      <Fld label="Away message" error={errors.message}>
        <Textarea
          unstyled
          className={controlClass}
          rows={3}
          value={s.message}
          onChange={(e) => set({ message: e.target.value })}
        />
      </Fld>
    </>
  )
}
