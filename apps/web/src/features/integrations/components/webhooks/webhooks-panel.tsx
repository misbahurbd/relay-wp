import { useQuery } from '@tanstack/react-query'
import {
  KeyRoundIcon,
  ListIcon,
  MoreVerticalIcon,
  PencilIcon,
  PlusIcon,
  SendIcon,
  Trash2Icon,
  WebhookIcon,
} from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import {
  useDeleteWebhook,
  useRevealSecret,
  useSendTest,
  useUpdateWebhook,
} from '../../api/integrations.mutations'
import { webhooksQueryOptions } from '../../api/integrations.queries'
import type { Endpoint } from '../../types'
import { Btn } from '../kit/btn'
import { percent, plural, relativeTime } from '../kit/format'
import { Modal } from '../kit/modal'
import { Pill } from '../kit/pill'
import { Toggle } from '../kit/toggle'
import { DeliveriesDialog, DeliveryPill } from './deliveries-dialog'
import { EndpointDialog } from './endpoint-dialog'
import { SecretDialog } from './secret-dialog'

type Open =
  | { kind: 'add' }
  | { kind: 'edit'; endpoint: Endpoint }
  | { kind: 'secret'; endpointId: string; secret: string; isNew?: boolean }
  | { kind: 'log'; endpoint: Endpoint }
  | { kind: 'delete'; endpoint: Endpoint }
  | null

/** Webhook endpoints with their health, and every action on them. */
export function WebhooksPanel({ id, workspaceId }: { id: string; workspaceId: string }) {
  const endpoints = useQuery(webhooksQueryOptions(workspaceId))
  const update = useUpdateWebhook(workspaceId)
  const remove = useDeleteWebhook(workspaceId)
  const test = useSendTest(workspaceId)
  const reveal = useRevealSecret(workspaceId)
  const [open, setOpen] = useState<Open>(null)

  async function setEnabled(e: Endpoint, enabled: boolean) {
    try {
      await update.mutateAsync({
        endpointId: e.id,
        body: { url: e.url, description: e.description, events: e.events, enabled },
      })
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  async function sendTest(e: Endpoint) {
    try {
      const d = await test.mutateAsync(e.id)
      if (d.status === 'succeeded')
        toast.success(`Test event delivered: ${d.response_status} in ${d.response_ms} ms`)
      else toast.error(`Test event failed: ${d.error ?? 'no response'}`)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  async function showSecret(e: Endpoint) {
    try {
      setOpen({ kind: 'secret', endpointId: e.id, secret: await reveal.mutateAsync(e.id) })
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  async function onDelete(e: Endpoint) {
    try {
      await remove.mutateAsync(e.id)
      toast.success('Webhook removed')
      setOpen(null)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  const list = endpoints.data ?? []
  return (
    <section
      id={id}
      aria-labelledby={`${id}-title`}
      className="flex scroll-mt-6 flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:px-[22px]"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-0.5">
          <h2 id={`${id}-title`} className="text-[17px] font-bold">
            Webhooks
          </h2>
          <span className="text-[13px] text-muted-foreground">
            We’ll POST a JSON event to your URL whenever something happens.
          </span>
        </div>
        <Btn size="sm" onClick={() => setOpen({ kind: 'add' })} disabled={list.length >= 10}>
          <PlusIcon className="size-4" aria-hidden="true" />
          Add endpoint
        </Btn>
      </div>

      {endpoints.isPending ? (
        <Skeleton className="h-24 rounded-[10px]" />
      ) : endpoints.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {errorMessage(endpoints.error)}
        </p>
      ) : list.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-[10px] border border-dashed p-6 text-center">
          <WebhookIcon className="size-6 text-muted-foreground" aria-hidden="true" />
          <p className="text-[13px] text-muted-foreground">
            No endpoints yet. Add one to get messages, contacts and broadcast events as they happen.
          </p>
        </div>
      ) : (
        <ul className="flex flex-col divide-y rounded-[10px] border">
          {list.map((e) => (
            <li key={e.id} className="flex flex-col gap-2 p-3">
              <div className="flex items-start gap-3">
                <div className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="truncate font-mono text-[13px] font-semibold" title={e.url}>
                    {e.url}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {e.description ? `${e.description} · ` : ''}
                    {plural(e.events.length, 'event')}
                    {e.week_total > 0 &&
                      ` · ${percent(e.week_succeeded, e.week_total)} delivered in 7 days`}
                  </span>
                </div>
                <Toggle
                  on={e.enabled}
                  onChange={(on) => setEnabled(e, on)}
                  label={`${e.enabled ? 'Turn off' : 'Turn on'} ${e.url}`}
                  disabled={update.isPending}
                />
                <DropdownMenu>
                  <DropdownMenuTrigger asChild>
                    <button
                      type="button"
                      aria-label={`Actions for ${e.url}`}
                      className="flex size-8 flex-none items-center justify-center rounded-lg text-muted-foreground hover:bg-muted hover:text-foreground"
                    >
                      <MoreVerticalIcon className="size-4" aria-hidden="true" />
                    </button>
                  </DropdownMenuTrigger>
                  <DropdownMenuContent align="end">
                    <DropdownMenuItem onSelect={() => setOpen({ kind: 'edit', endpoint: e })}>
                      <PencilIcon aria-hidden="true" />
                      Edit
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => setOpen({ kind: 'log', endpoint: e })}>
                      <ListIcon aria-hidden="true" />
                      Deliveries
                    </DropdownMenuItem>
                    <DropdownMenuItem onSelect={() => showSecret(e)}>
                      <KeyRoundIcon aria-hidden="true" />
                      Signing secret
                    </DropdownMenuItem>
                    <DropdownMenuSeparator />
                    <DropdownMenuItem
                      variant="destructive"
                      onSelect={() => setOpen({ kind: 'delete', endpoint: e })}
                    >
                      <Trash2Icon aria-hidden="true" />
                      Remove
                    </DropdownMenuItem>
                  </DropdownMenuContent>
                </DropdownMenu>
              </div>
              {e.disabled_reason && (
                <Pill tone="amber" className="self-start">
                  {e.disabled_reason}
                </Pill>
              )}
              <div className="flex flex-wrap items-center justify-between gap-2">
                <span className="flex items-center gap-2 text-xs text-muted-foreground">
                  {e.last_delivery_at && e.last_status ? (
                    <>
                      <DeliveryPill
                        d={{ status: e.last_status, response_status: e.last_response_status }}
                      />
                      Last delivery {relativeTime(e.last_delivery_at)}
                    </>
                  ) : (
                    'No deliveries yet'
                  )}
                </span>
                <Btn
                  size="sm"
                  variant="outline"
                  onClick={() => sendTest(e)}
                  disabled={test.isPending}
                >
                  <SendIcon className="size-3.5" aria-hidden="true" />
                  Send test event
                </Btn>
              </div>
            </li>
          ))}
        </ul>
      )}

      {(open?.kind === 'add' || open?.kind === 'edit') && (
        <EndpointDialog
          workspaceId={workspaceId}
          endpoint={open.kind === 'edit' ? open.endpoint : undefined}
          onCreated={(c) => setOpen({ kind: 'secret', ...c, isNew: true })}
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === 'secret' && (
        <SecretDialog
          workspaceId={workspaceId}
          endpointId={open.endpointId}
          secret={open.secret}
          isNew={open.isNew}
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === 'log' && (
        <DeliveriesDialog
          workspaceId={workspaceId}
          endpoint={open.endpoint}
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === 'delete' && (
        <Modal
          onClose={() => setOpen(null)}
          title="Remove this webhook?"
          description="Relay stops sending to it now, and its delivery log is deleted."
        >
          <span className="break-all font-mono text-[13px]">{open.endpoint.url}</span>
          <div className="flex justify-end gap-2">
            <Btn variant="outline" onClick={() => setOpen(null)}>
              Cancel
            </Btn>
            <Btn
              variant="danger"
              onClick={() => onDelete(open.endpoint)}
              disabled={remove.isPending}
            >
              Remove webhook
            </Btn>
          </div>
        </Modal>
      )}
    </section>
  )
}
