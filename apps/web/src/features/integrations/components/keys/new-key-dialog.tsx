import { KeyRoundIcon } from 'lucide-react'
import type { NewApiKey } from '../../types'
import { Btn } from '../kit/btn'
import { CopyField } from '../kit/copy-field'
import { Modal } from '../kit/modal'

/** The one time a key is shown in full. */
export function NewKeyDialog({
  created,
  workspaceId,
  onClose,
}: {
  created: NewApiKey
  workspaceId: string
  onClose: () => void
}) {
  return (
    <Modal
      onClose={onClose}
      title="Copy your API key"
      description="This is the only time you'll see it. Store it somewhere safe, like a password manager."
      icon={
        <span className="flex size-10 flex-none items-center justify-center rounded-xl bg-brand-soft text-brand-text">
          <KeyRoundIcon className="size-5" aria-hidden="true" />
        </span>
      }
    >
      <div className="flex flex-col gap-1.5">
        <span className="text-[13px] font-bold">{created.apiKey.name}</span>
        <CopyField value={created.key} label="API key" />
      </div>
      <div className="flex flex-col gap-1.5">
        <span className="text-[13px] font-bold">Workspace ID</span>
        <CopyField value={workspaceId} label="Workspace ID" />
      </div>
      <div className="rounded-[10px] bg-info-soft p-3 text-[13px] text-info">
        Send <code className="font-mono">Authorization: Bearer &lt;key&gt;</code> and{' '}
        <code className="font-mono">X-Workspace-ID</code> with every request to{' '}
        <code className="font-mono">/v1/workspace/…</code>. The key can do what you can do in this
        workspace, and stops working if you leave it.
      </div>
      <div className="flex justify-end">
        <Btn onClick={onClose}>I've saved it</Btn>
      </div>
    </Modal>
  )
}
