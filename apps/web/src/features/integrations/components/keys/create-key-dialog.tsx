import { type FormEvent, useState } from 'react'
import { errorMessage } from '#/lib/api'
import { useCreateApiKey } from '../../api/integrations.mutations'
import { keySchema } from '../../schemas/integrations.schema'
import type { NewApiKey } from '../../types'
import { Btn } from '../kit/btn'
import { ErrorText, TextField } from '../kit/field'
import { Modal } from '../kit/modal'

/** Name a new key; the parent shows it once it's made. */
export function CreateKeyDialog({
  workspaceId,
  onCreated,
  onClose,
}: {
  workspaceId: string
  onCreated: (created: NewApiKey) => void
  onClose: () => void
}) {
  const create = useCreateApiKey(workspaceId)
  const [name, setName] = useState('')
  const [nameError, setNameError] = useState<string>()
  const [error, setError] = useState<string>()

  async function submit(e: FormEvent) {
    e.preventDefault()
    const parsed = keySchema.safeParse({ name })
    if (!parsed.success) {
      setNameError(parsed.error.flatten().fieldErrors.name?.[0])
      return
    }
    setNameError(undefined)
    setError(undefined)
    try {
      onCreated(await create.mutateAsync(parsed.data))
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <Modal
      onClose={onClose}
      title="Create API key"
      description="The key acts as you in this workspace. Revoke it any time."
    >
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        <TextField
          id="key-name"
          label="Name"
          placeholder="Website orders"
          value={name}
          onChange={(e) => setName(e.target.value)}
          error={nameError}
          hint="So you can tell your keys apart later."
          maxLength={80}
          autoFocus
        />
        <ErrorText>{error}</ErrorText>
        <div className="flex justify-end gap-2">
          <Btn variant="outline" onClick={onClose}>
            Cancel
          </Btn>
          <Btn type="submit" disabled={create.isPending}>
            {create.isPending ? 'Creating…' : 'Create key'}
          </Btn>
        </div>
      </form>
    </Modal>
  )
}
