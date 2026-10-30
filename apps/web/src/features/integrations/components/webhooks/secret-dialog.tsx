import { ShieldCheckIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import { useRotateSecret } from '../../api/integrations.mutations'
import { Btn } from '../kit/btn'
import { CopyField } from '../kit/copy-field'
import { Modal } from '../kit/modal'

/** The signing secret, how to check a signature with it, and Rotate. */
export function SecretDialog({
  workspaceId,
  endpointId,
  secret: initial,
  isNew,
  onClose,
}: {
  workspaceId: string
  endpointId: string
  secret: string
  isNew?: boolean
  onClose: () => void
}) {
  const rotate = useRotateSecret(workspaceId)
  const [secret, setSecret] = useState(initial)
  const [confirming, setConfirming] = useState(false)

  async function onRotate() {
    try {
      setSecret(await rotate.mutateAsync(endpointId))
      setConfirming(false)
      toast.success('Signing secret replaced')
    } catch (err) {
      toast.error(errorMessage(err))
    }
  }

  return (
    <Modal
      onClose={onClose}
      title={isNew ? 'Webhook added' : 'Signing secret'}
      description="Check each request's Relay-Signature header with this secret to know it came from Relay."
      icon={
        <span className="flex size-10 flex-none items-center justify-center rounded-xl bg-info-soft text-info">
          <ShieldCheckIcon className="size-5" aria-hidden="true" />
        </span>
      }
      width="sm:max-w-[600px]"
    >
      <CopyField value={secret} label="Signing secret" hideable={!isNew} />
      <div className="flex flex-col gap-2 rounded-[10px] bg-muted p-3 text-[13px]">
        <span className="font-bold">Checking a signature</span>
        <ol className="list-decimal space-y-1 pl-5 text-muted-foreground">
          <li>
            Split <code className="font-mono">Relay-Signature: t=…,v1=…</code> into the timestamp
            and the signature.
          </li>
          <li>
            Compute HMAC-SHA256 of <code className="font-mono">{'{t}.{raw body}'}</code> with the
            secret, as hex.
          </li>
          <li>
            Compare it to <code className="font-mono">v1</code> in constant time, and reject
            timestamps older than 5 minutes.
          </li>
        </ol>
        <span className="text-muted-foreground">
          Deliveries can repeat: use the <code className="font-mono">Relay-Delivery</code> header to
          skip ones you've already handled.
        </span>
      </div>
      <div className="flex flex-wrap items-center justify-between gap-2">
        {!confirming && (
          <Btn variant="ghost" size="sm" onClick={() => setConfirming(true)}>
            Rotate secret
          </Btn>
        )}
        {confirming && (
          <div className="flex flex-wrap items-center gap-2">
            <span className="text-[13px] text-muted-foreground">
              Requests are signed with the new secret at once.
            </span>
            <Btn variant="danger" size="sm" onClick={onRotate} disabled={rotate.isPending}>
              Rotate now
            </Btn>
            <Btn variant="outline" size="sm" onClick={() => setConfirming(false)}>
              Cancel
            </Btn>
          </div>
        )}
        <Btn className="ml-auto" onClick={onClose}>
          Done
        </Btn>
      </div>
    </Modal>
  )
}
