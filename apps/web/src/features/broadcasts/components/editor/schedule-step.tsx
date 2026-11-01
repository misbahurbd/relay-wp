import { CalendarIcon, InfoIcon, SendIcon } from 'lucide-react'
import { useId } from 'react'
import { Input } from '#/components/ui/input'
import { RadioGroup } from '#/components/ui/radio-group'
import type { Schedule } from '../../types'
import { controlClass, Fld, Select } from '../kit/field'
import { zoneOptions } from '../kit/time-zone'
import { StepHeading } from './audience-step'
import { OptionCard } from './option-card'

/** Step 3: now, or a date, time and time zone. */
export function ScheduleStep({
  schedule,
  readOnly,
  error,
  onChange,
}: {
  schedule: Schedule
  readOnly: boolean
  error?: string
  onChange: (s: Schedule) => void
}) {
  const id = useId()
  const later = schedule.when === 'later'
  return (
    <>
      <StepHeading title="When should it go out?">
        Messages are sent in batches so your number stays healthy.
      </StepHeading>
      <RadioGroup
        value={schedule.when}
        disabled={readOnly}
        aria-label="When to send"
        onValueChange={(v) => onChange({ ...schedule, when: v as Schedule['when'] })}
        className="grid gap-3 sm:grid-cols-2"
      >
        <OptionCard id={`${id}-now`} value="now" selected={!later} icon={<SendIcon />}>
          <span className="text-[15px] font-bold">Send now</span>
          <span className="text-[13px] text-muted-foreground">
            Starts within a minute after you confirm
          </span>
        </OptionCard>
        <OptionCard id={`${id}-later`} value="later" selected={later} icon={<CalendarIcon />}>
          <span className="text-[15px] font-bold">Schedule for later</span>
          <span className="text-[13px] text-muted-foreground">Pick a date and time</span>
        </OptionCard>
      </RadioGroup>
      {later && (
        <>
          <div className="grid gap-3 sm:grid-cols-3">
            <Fld label="Date">
              <Input
                unstyled
                type="date"
                className={controlClass}
                value={schedule.date}
                disabled={readOnly}
                onChange={(e) => onChange({ ...schedule, date: e.target.value })}
              />
            </Fld>
            <Fld label="Time">
              <Input
                unstyled
                type="time"
                className={controlClass}
                value={schedule.time}
                disabled={readOnly}
                onChange={(e) => onChange({ ...schedule, time: e.target.value })}
              />
            </Fld>
            <Fld label="Time zone">
              <Select
                value={schedule.zone}
                disabled={readOnly}
                onValueChange={(zone) => onChange({ ...schedule, zone })}
                options={zoneOptions()}
              />
            </Fld>
          </div>
          {error && <span className="text-xs font-semibold text-destructive">{error}</span>}
          <div className="flex gap-2.5 rounded-xl bg-info-soft px-3.5 py-3 text-[13px]">
            <InfoIcon className="size-[18px] flex-none text-info" aria-hidden="true" />
            <span>
              You can change or cancel a scheduled broadcast until it starts. Up to 90 days ahead.
            </span>
          </div>
        </>
      )}
    </>
  )
}
