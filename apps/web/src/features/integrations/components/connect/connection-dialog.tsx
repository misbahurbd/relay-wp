import { AlertTriangleIcon, CheckCircle2Icon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import { useDisconnect, useTestConnection } from '../../api/integrations.mutations'
import { APP_BY_ID } from '../../schemas/integrations.schema'
import type { Connection, Provider } from '../../types'
import { AppTile } from '../kit/app-tile'
import { Btn } from '../kit/btn'
import { formatDate, relativeTime } from '../kit/format'
import { Modal } from '../kit/modal'

/** A connected store or Sheets: its account, last check, Test, Reconnect and Disconnect. */
export function ConnectionDialog({
  workspaceId,
  provider,
  connection,
  onReconnect,
  onClose,
}: {
  workspaceId: string
  provider: Provider
  connection?: Connection
  onReconnect: () => void
  onClose: () => void
}) {
  const test = useTestConnection(workspaceId)
  const disconnect = useDisconnect(workspaceId)
  const [confirming, setConfirming] = useState(false)
  const app = APP_BY_ID[provider]
  // The test result is newer than the overview the dialog opened with.
  const c = test.data ?? connection

  async function onTest() {
    try {
      const r = await test.mutateAsync(provider)
      if (r.status === 'connected') toast.success(`${app.name} is working`)
      else toast.error(r.last_error ?? `${app.name} didn't answer`)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  async function onDisconnect() {
    try {
      await disconnect.mutateAsync(provider)
      toast.success(`${app.name} disconnected`)
      onClose()
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <Modal onClose={onClose} title={app.name} icon={<AppTile mono={app.mono} tone={app.tone} />}>
      {c && (
        <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2 rounded-[10px] bg-muted p-3 text-[13px]">
          <dt className="text-muted-foreground">Account</dt>
          <dd className="truncate font-semibold">{c.account_label}</dd>
          <dt className="text-muted-foreground">Connected</dt>
          <dd>{formatDate(c.created_at)}</dd>
          <dt className="text-muted-foreground">Last check</dt>
          <dd className="flex items-center gap-1.5">
            {c.status === 'error' ? (
              <AlertTriangleIcon className="size-3.5 text-warning" aria-hidden="true" />
            ) : (
              <CheckCircle2Icon className="size-3.5 text-brand-text" aria-hidden="true" />
            )}
            {c.last_checked_at ? relativeTime(c.last_checked_at) : 'Never'}
          </dd>
        </dl>
      )}
      {c?.status === 'error' && c.last_error && (
        <p role="alert" className="rounded-[10px] bg-warning-soft p-3 text-[13px] text-warning">
          {c.last_error}
        </p>
      )}
      {confirming ? (
        <div className="flex flex-col gap-3 rounded-[10px] border border-destructive/40 p-3">
          <p className="text-[13px]">
            Relay forgets the credentials and stops syncing with {app.name}. You can connect again
            later.
          </p>
          <div className="flex justify-end gap-2">
            <Btn variant="outline" size="sm" onClick={() => setConfirming(false)}>
              Keep connected
            </Btn>
            <Btn variant="danger" size="sm" onClick={onDisconnect} disabled={disconnect.isPending}>
              Disconnect
            </Btn>
          </div>
        </div>
      ) : (
        <div className="flex flex-wrap justify-between gap-2">
          <Btn variant="danger-soft" onClick={() => setConfirming(true)}>
            Disconnect
          </Btn>
          <div className="flex gap-2">
            <Btn variant="outline" onClick={onReconnect}>
              Reconnect
            </Btn>
            <Btn onClick={onTest} disabled={test.isPending}>
              {test.isPending ? 'Checking…' : 'Test connection'}
            </Btn>
          </div>
        </div>
      )}
    </Modal>
  )
}
