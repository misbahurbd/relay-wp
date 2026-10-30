import { useQuery } from '@tanstack/react-query'
import { Trash2Icon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import { useCreateApiKey, useRevokeApiKey } from '../../api/integrations.mutations'
import { apiKeysQueryOptions } from '../../api/integrations.queries'
import { APP_BY_ID, KEY_APP_STEPS } from '../../schemas/integrations.schema'
import type { KeyApp, NewApiKey } from '../../types'
import { AppTile } from '../kit/app-tile'
import { Btn } from '../kit/btn'
import { CopyField } from '../kit/copy-field'
import { relativeTime } from '../kit/format'
import { Modal } from '../kit/modal'

/** Zapier, Make and n8n connect with an API key tagged for the app. */
export function KeyAppDialog({
  workspaceId,
  app: appId,
  onClose,
}: {
  workspaceId: string
  app: KeyApp
  onClose: () => void
}) {
  const app = APP_BY_ID[appId]
  const keys = useQuery(apiKeysQueryOptions(workspaceId))
  const create = useCreateApiKey(workspaceId)
  const revoke = useRevokeApiKey(workspaceId)
  const [created, setCreated] = useState<NewApiKey | null>(null)
  const appKeys = (keys.data ?? []).filter((k) => k.app === appId)

  async function onCreate() {
    try {
      setCreated(await create.mutateAsync({ name: app.name, app: appId }))
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  async function onRevoke(keyId: string) {
    try {
      await revoke.mutateAsync(keyId)
      toast.success('Key revoked')
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <Modal
      onClose={onClose}
      title={`Connect ${app.name}`}
      description={app.description}
      icon={<AppTile mono={app.mono} tone={app.tone} />}
      width="sm:max-w-[560px]"
    >
      <ol className="list-decimal space-y-1 rounded-[10px] bg-muted p-3 pl-8 text-[13px] text-muted-foreground">
        {KEY_APP_STEPS[appId].map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>

      {created ? (
        <div className="flex flex-col gap-3">
          <p className="text-[13px] font-semibold">Copy the key now. You won't see it again.</p>
          <CopyField value={created.key} label={`${app.name} API key`} />
          <div className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Workspace ID</span>
            <CopyField value={workspaceId} label="Workspace ID" />
          </div>
        </div>
      ) : (
        <Btn onClick={onCreate} disabled={create.isPending} className="self-start">
          {create.isPending ? 'Creating…' : `Create a ${app.name} key`}
        </Btn>
      )}

      {appKeys.length > 0 && (
        <div className="flex flex-col gap-2">
          <span className="text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase">
            Active {app.name} keys
          </span>
          <ul className="flex flex-col divide-y rounded-[10px] border">
            {appKeys.map((k) => (
              <li key={k.id} className="flex items-center gap-3 px-3 py-2">
                <code className="min-w-0 flex-1 truncate font-mono text-xs">
                  {k.prefix}
                  {'•'.repeat(8)}
                  {k.last4}
                </code>
                <span className="text-xs text-muted-foreground">
                  {k.last_used_at ? `Used ${relativeTime(k.last_used_at)}` : 'Never used'}
                </span>
                <button
                  type="button"
                  onClick={() => onRevoke(k.id)}
                  aria-label={`Revoke key ending ${k.last4}`}
                  className="flex size-8 items-center justify-center rounded-lg text-destructive hover:bg-destructive-soft"
                >
                  <Trash2Icon className="size-4" aria-hidden="true" />
                </button>
              </li>
            ))}
          </ul>
        </div>
      )}
      <div className="flex justify-end">
        <Btn variant="outline" onClick={onClose}>
          Done
        </Btn>
      </div>
    </Modal>
  )
}
