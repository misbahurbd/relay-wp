import { useQuery } from '@tanstack/react-query'
import { KeyRoundIcon, PlusIcon, RefreshCwIcon, Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { useRevokeApiKey, useRollApiKey } from '../../api/integrations.mutations'
import { apiKeysQueryOptions } from '../../api/integrations.queries'
import { APPS } from '../../schemas/integrations.schema'
import type { ApiKey, NewApiKey } from '../../types'
import { Btn } from '../kit/btn'
import { formatDate, relativeTime } from '../kit/format'
import { Modal } from '../kit/modal'
import { Pill } from '../kit/pill'
import { CreateKeyDialog } from './create-key-dialog'
import { NewKeyDialog } from './new-key-dialog'

type Confirm = { kind: 'roll' | 'revoke'; key: ApiKey } | null

/** The workspace's API keys: create, roll and revoke. */
export function ApiKeysPanel({ id, workspaceId }: { id: string; workspaceId: string }) {
  const keys = useQuery(apiKeysQueryOptions(workspaceId))
  const roll = useRollApiKey(workspaceId)
  const revoke = useRevokeApiKey(workspaceId)
  const [creating, setCreating] = useState(false)
  const [created, setCreated] = useState<NewApiKey | null>(null)
  const [confirm, setConfirm] = useState<Confirm>(null)

  async function onConfirm() {
    if (!confirm) return
    try {
      if (confirm.kind === 'roll') setCreated(await roll.mutateAsync(confirm.key.id))
      else {
        await revoke.mutateAsync(confirm.key.id)
        toast.success(`${confirm.key.name} revoked`)
      }
      setConfirm(null)
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <section
      id={id}
      aria-labelledby={`${id}-title`}
      className="flex scroll-mt-6 flex-col gap-4 rounded-xl border bg-card p-5 shadow-card sm:px-[22px]"
    >
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex flex-col gap-0.5">
          <h2 id={`${id}-title`} className="text-[17px] font-bold">
            API keys
          </h2>
          <span className="text-[13px] text-muted-foreground">
            Send messages and manage contacts from your own app.
          </span>
        </div>
        <Btn size="sm" onClick={() => setCreating(true)}>
          <PlusIcon className="size-4" aria-hidden="true" />
          Create key
        </Btn>
      </div>

      {keys.isPending ? (
        <Skeleton className="h-24 rounded-[10px]" />
      ) : keys.isError ? (
        <p role="alert" className="text-sm text-destructive">
          {errorMessage(keys.error)}
        </p>
      ) : keys.data.length === 0 ? (
        <div className="flex flex-col items-center gap-2 rounded-[10px] border border-dashed p-6 text-center">
          <KeyRoundIcon className="size-6 text-muted-foreground" aria-hidden="true" />
          <p className="text-[13px] text-muted-foreground">
            No keys yet. Create one to call the Relay API from your own software.
          </p>
        </div>
      ) : (
        <ul className="flex flex-col divide-y rounded-[10px] border">
          {keys.data.map((k) => (
            <li key={k.id} className="flex flex-wrap items-center gap-3 p-3">
              <div className="flex min-w-0 flex-1 flex-col gap-1">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="truncate text-sm font-bold">{k.name}</span>
                  {k.app && (
                    <Pill tone="amber">{APPS.find((a) => a.id === k.app)?.name ?? k.app}</Pill>
                  )}
                </div>
                <code className="font-mono text-xs text-muted-foreground">
                  {k.prefix}
                  {'•'.repeat(12)}
                  {k.last4}
                </code>
                <span className="text-xs text-muted-foreground">
                  Created {formatDate(k.created_at)} by {k.created_by.name} ·{' '}
                  {k.last_used_at ? `Last used ${relativeTime(k.last_used_at)}` : 'Never used'}
                </span>
              </div>
              <div className="flex gap-1.5">
                <Btn
                  size="sm"
                  variant="outline"
                  onClick={() => setConfirm({ kind: 'roll', key: k })}
                  aria-label={`Regenerate ${k.name}`}
                >
                  <RefreshCwIcon className="size-3.5" aria-hidden="true" />
                  Regenerate
                </Btn>
                <Btn
                  size="sm"
                  variant="danger-soft"
                  onClick={() => setConfirm({ kind: 'revoke', key: k })}
                  aria-label={`Revoke ${k.name}`}
                >
                  <Trash2Icon className="size-3.5" aria-hidden="true" />
                </Btn>
              </div>
            </li>
          ))}
        </ul>
      )}

      {creating && (
        <CreateKeyDialog
          workspaceId={workspaceId}
          onCreated={(c) => {
            setCreating(false)
            setCreated(c)
          }}
          onClose={() => setCreating(false)}
        />
      )}
      {created && (
        <NewKeyDialog
          created={created}
          workspaceId={workspaceId}
          onClose={() => setCreated(null)}
        />
      )}
      {confirm && (
        <Modal
          onClose={() => setConfirm(null)}
          title={
            confirm.kind === 'roll'
              ? `Regenerate ${confirm.key.name}?`
              : `Revoke ${confirm.key.name}?`
          }
          description={
            confirm.kind === 'roll'
              ? 'You get a new key now. The current one stops working at once, so update every app that uses it.'
              : 'Apps using this key stop working at once. This can’t be undone.'
          }
        >
          <div className="flex justify-end gap-2">
            <Btn variant="outline" onClick={() => setConfirm(null)}>
              Cancel
            </Btn>
            <Btn variant="danger" onClick={onConfirm} disabled={roll.isPending || revoke.isPending}>
              {confirm.kind === 'roll' ? 'Regenerate' : 'Revoke key'}
            </Btn>
          </div>
        </Modal>
      )}
    </section>
  )
}
