import { useQuery } from '@tanstack/react-query'
import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import { Checkbox } from '#/components/ui/checkbox'
import { errorMessage } from '#/lib/api'
import { useCreateWebhook, useUpdateWebhook } from '../../api/integrations.mutations'
import { eventsQueryOptions } from '../../api/integrations.queries'
import { endpointSchema } from '../../schemas/integrations.schema'
import type { Endpoint } from '../../types'
import { Btn } from '../kit/btn'
import { ErrorText, TextField } from '../kit/field'
import { Modal } from '../kit/modal'

type Errors = Partial<Record<'url' | 'description' | 'events', string>>

/** Add an endpoint, or change one. A new endpoint's signing secret goes to onCreated. */
export function EndpointDialog({
  workspaceId,
  endpoint,
  onCreated,
  onClose,
}: {
  workspaceId: string
  endpoint?: Endpoint
  onCreated: (created: { endpointId: string; secret: string }) => void
  onClose: () => void
}) {
  const events = useQuery(eventsQueryOptions(workspaceId))
  const create = useCreateWebhook(workspaceId)
  const update = useUpdateWebhook(workspaceId)
  const [url, setUrl] = useState(endpoint?.url ?? '')
  const [description, setDescription] = useState(endpoint?.description ?? '')
  const [picked, setPicked] = useState<string[]>(endpoint?.events ?? [])
  const [errors, setErrors] = useState<Errors>({})
  const [error, setError] = useState<string>()
  const pending = create.isPending || update.isPending

  async function submit(e: FormEvent) {
    e.preventDefault()
    const parsed = endpointSchema.safeParse({
      url,
      description,
      events: picked,
      enabled: endpoint?.enabled ?? true,
    })
    if (!parsed.success) {
      const f = parsed.error.flatten().fieldErrors
      setErrors({ url: f.url?.[0], description: f.description?.[0], events: f.events?.[0] })
      return
    }
    setErrors({})
    setError(undefined)
    try {
      if (endpoint) {
        await update.mutateAsync({ endpointId: endpoint.id, body: parsed.data })
        toast.success('Webhook saved')
        onClose()
      } else {
        const created = await create.mutateAsync(parsed.data)
        onCreated({ endpointId: created.endpoint.id, secret: created.secret })
      }
    } catch (err) {
      const msg = errorMessage(err)
      if (/url/i.test(msg)) setErrors({ url: msg })
      else setError(msg)
    }
  }

  function toggle(type: string, on: boolean) {
    setPicked((p) => (on ? [...p, type] : p.filter((t) => t !== type)))
  }

  return (
    <Modal
      onClose={onClose}
      title={endpoint ? 'Edit webhook' : 'Add webhook'}
      description="We’ll POST a signed JSON event to this URL whenever something happens."
      width="sm:max-w-[600px]"
    >
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        <TextField
          id="webhook-url"
          label="Endpoint URL"
          type="url"
          inputMode="url"
          placeholder="https://api.your-store.com/hooks/relay"
          value={url}
          onChange={(e) => setUrl(e.target.value)}
          error={errors.url}
          hint="A public https:// address. Redirects aren't followed."
          autoFocus
        />
        <TextField
          id="webhook-description"
          label="Description (optional)"
          placeholder="CRM sync"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          error={errors.description}
          maxLength={200}
        />
        <fieldset
          className="flex flex-col gap-2"
          aria-describedby={errors.events ? 'events-error' : undefined}
        >
          <legend className="mb-1 text-[13px] font-bold">Events to send</legend>
          <div className="grid gap-2 sm:grid-cols-2">
            {(events.data ?? []).map((ev) => {
              const on = picked.includes(ev.type)
              return (
                <label
                  key={ev.type}
                  htmlFor={`event-${ev.type}`}
                  className="flex cursor-pointer items-start gap-2.5 rounded-[10px] border p-3 hover:bg-muted has-[:checked]:border-primary has-[:checked]:bg-brand-soft/40"
                >
                  <Checkbox
                    id={`event-${ev.type}`}
                    checked={on}
                    onCheckedChange={(c) => toggle(ev.type, c === true)}
                    className="mt-0.5"
                  />
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <code className="font-mono text-[13px] font-semibold">{ev.type}</code>
                    <span className="text-xs text-muted-foreground">{ev.description}</span>
                  </span>
                </label>
              )
            })}
          </div>
          <ErrorText id="events-error">{errors.events}</ErrorText>
        </fieldset>
        <ErrorText>{error}</ErrorText>
        <div className="flex justify-end gap-2">
          <Btn variant="outline" onClick={onClose}>
            Cancel
          </Btn>
          <Btn type="submit" disabled={pending}>
            {pending ? 'Saving…' : endpoint ? 'Save webhook' : 'Add webhook'}
          </Btn>
        </div>
      </form>
    </Modal>
  )
}
