import { useState } from 'react'
import { toast } from 'sonner'
import { isApiError } from '#/lib/api'
import {
  useCreateBroadcast,
  useSendBroadcast,
  useUpdateBroadcast,
} from '../api/broadcasts.mutations'
import { browserZone, partsIn, zonedToIso } from '../components/kit/time-zone'
import { toBody } from '../schemas/broadcasts.schema'
import type { Broadcast, BroadcastForm, Schedule } from '../types'

/** Server fields that belong to each step, so an error sends the user back there. */
const STEP_OF: [RegExp, number][] = [
  [/^(name|audience)/, 1],
  [/^(template_id|number_id|variables)/, 2],
  [/^scheduled_at/, 3],
]

function initialSchedule(b?: Broadcast): Schedule {
  const zone = browserZone()
  const at = b?.scheduled_at ?? new Date(Date.now() + 60 * 60 * 1000).toISOString()
  return { when: b?.status === 'scheduled' ? 'later' : 'now', zone, ...partsIn(at, zone) }
}

/**
 * The broadcast wizard's state: the form, its step, when it goes, the errors the server found per
 * field ("variables.body[0].fallback"), and saving or sending it. A new broadcast is saved as a
 * draft first, so sending always works on a stored one; the wizard keeps its ID instead of
 * changing page, so its steps stay where they are.
 */
export function useBroadcastEditor({
  workspaceId,
  broadcast,
  initial,
}: {
  workspaceId: string
  broadcast?: Broadcast
  initial: BroadcastForm
}) {
  const [form, setForm] = useState(initial)
  const [step, setStep] = useState(1)
  const [schedule, setSchedule] = useState(() => initialSchedule(broadcast))
  const [savedId, setSavedId] = useState(broadcast?.id)
  const [sent, setSent] = useState<Broadcast>()
  const [errors, setErrors] = useState<Record<string, string>>({})
  const [error, setError] = useState<unknown>()
  const create = useCreateBroadcast(workspaceId)
  const update = useUpdateBroadcast(workspaceId)
  const send = useSendBroadcast(workspaceId)
  const busy = create.isPending || update.isPending ? 'save' : send.isPending ? 'send' : undefined

  function fail(e: unknown) {
    setError(e)
    const map: Record<string, string> = {}
    if (isApiError(e)) for (const d of e.details ?? []) if (d.field) map[d.field] ??= d.message
    setErrors(map)
    const first = Object.keys(map)[0]
    const back = first && STEP_OF.find(([re]) => re.test(first))?.[1]
    if (back) setStep(back)
  }

  /** Saves the form; returns the stored broadcast, or nothing when it couldn't. */
  async function save(): Promise<Broadcast | undefined> {
    const body = toBody({ ...form, name: form.name.trim() || 'Untitled broadcast' })
    try {
      const saved = savedId
        ? await update.mutateAsync({ broadcastId: savedId, body })
        : await create.mutateAsync(body)
      setSavedId(saved.id)
      setErrors({})
      setError(undefined)
      return saved
    } catch (e) {
      fail(e)
    }
  }

  /** Saves, then sends now or at the scheduled time. */
  async function sendIt() {
    const saved = await save()
    if (!saved) return
    const scheduledAt =
      schedule.when === 'later'
        ? zonedToIso(schedule.date, schedule.time, schedule.zone)
        : undefined
    try {
      const done = await send.mutateAsync({ broadcastId: saved.id, scheduledAt })
      setSent(done)
      toast.success(
        done.status === 'scheduled' ? `${done.name} scheduled` : `${done.name} is sending`,
      )
    } catch (e) {
      fail(e)
    }
  }

  return {
    form,
    setForm,
    step,
    setStep,
    schedule,
    setSchedule,
    savedId,
    sent,
    errors,
    error,
    busy,
    save,
    send: sendIt,
  }
}
