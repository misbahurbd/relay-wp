import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { useState } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import {
  fetchMemberRolePreview,
  useChangeMemberRole,
  useRemoveMember,
} from '../../api/team.mutations'
import {
  permissionsQueryOptions,
  rolesQueryOptions,
  seatsQueryOptions,
} from '../../api/team.queries'
import { Btn } from '../kit/btn'
import { ErrorText, inputClass } from '../kit/field'
import { Modal } from '../kit/modal'
import { Pill } from '../kit/pill'
import { RadioCard } from '../kit/radio'

type Target = { user_id: string; name: string; role_id: string }

/** "Change Rafi's role": pick a role, see what they gain and lose, confirm. */
export function ChangeRoleDialog({
  workspaceId,
  member,
  initialRoleId,
  onClose,
}: {
  workspaceId: string
  member: Target
  initialRoleId?: string
  onClose: () => void
}) {
  const roles = useQuery(rolesQueryOptions(workspaceId))
  const catalog = useQuery(permissionsQueryOptions())
  const seats = useQuery(seatsQueryOptions(workspaceId))
  const change = useChangeMemberRole(workspaceId, member.user_id)
  const [pending, setPending] = useState(initialRoleId ?? member.role_id)
  const first = member.name.split(' ')[0]
  const choices = roles.data?.filter((r) => r.system_key !== 'owner') ?? []
  const current = roles.data?.find((r) => r.id === member.role_id)
  const next = roles.data?.find((r) => r.id === pending)
  const diff = pending !== member.role_id
  const preview = useQuery({
    queryKey: ['team', workspaceId, 'member-role-preview', member.user_id, pending],
    queryFn: () => fetchMemberRolePreview(workspaceId, member.user_id, pending),
    enabled: diff,
  })
  const label = (k: string) => catalog.data?.find((p) => p.key === k)?.label ?? k
  let seatNote = 'Seat usage doesn’t change. Extra permissions stay in place.'
  if (current && next && seats.data) {
    const used = seats.data.used + seats.data.reserved
    if (current.uses_seat && !next.uses_seat)
      seatNote = `This frees 1 seat (${used - 1} of ${seats.data.limit} used). Extra permissions stay in place.`
    else if (!current.uses_seat && next.uses_seat)
      seatNote = `This uses 1 seat (${used + 1} of ${seats.data.limit} used).`
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={`Change ${first}’s role`}
      description={`Currently ${current?.name ?? '…'}. Changes apply the next time they load a page.`}
      width="sm:max-w-[600px]"
    >
      <div role="radiogroup" aria-label="Role" className="flex flex-col gap-2">
        {choices.map((r) => (
          <RadioCard
            key={r.id}
            name="role"
            checked={r.id === pending}
            onSelect={() => setPending(r.id)}
          >
            <span className="flex flex-1 flex-col leading-[1.35]">
              <span className="flex flex-wrap items-center gap-1.5">
                <span className="font-bold">{r.name}</span>
                {!r.system_key && (
                  <Pill tone="gray" square className="text-[11px]">
                    Custom
                  </Pill>
                )}
                {r.id === member.role_id && (
                  <Pill tone="blue" square className="text-[11px]">
                    Current
                  </Pill>
                )}
              </span>
              <span className="text-[13px] text-muted-foreground">{r.description}</span>
            </span>
          </RadioCard>
        ))}
      </div>
      <Link to="/team/roles" className="self-start text-[13px] font-bold">
        Manage roles
      </Link>
      {diff && preview.data && (
        <>
          <div className="grid gap-2.5 sm:grid-cols-2">
            <div className="flex flex-col gap-1.5 rounded-lg bg-brand-soft px-3.5 py-3">
              <span className="text-xs font-extrabold text-brand-text">Will gain</span>
              {preview.data.gains.map((g) => (
                <span key={g} className="text-[13px]">
                  + {label(g)}
                </span>
              ))}
              {preview.data.gains.length === 0 && (
                <span className="text-[13px] text-muted-foreground">Nothing new</span>
              )}
            </div>
            <div className="flex flex-col gap-1.5 rounded-lg bg-destructive-soft px-3.5 py-3">
              <span className="text-xs font-extrabold text-destructive">Will lose</span>
              {preview.data.losses.map((l) => (
                <span key={l} className="text-[13px]">
                  − {label(l)}
                </span>
              ))}
              {preview.data.losses.length === 0 && (
                <span className="text-[13px] text-muted-foreground">Nothing removed</span>
              )}
            </div>
          </div>
          <span className="text-xs text-muted-foreground">{seatNote}</span>
        </>
      )}
      <ErrorText>
        {(change.error || preview.error) && errorMessage(change.error ?? preview.error)}
      </ErrorText>
      <div className="flex justify-end gap-2.5">
        <Btn variant="outline" onClick={onClose}>
          Cancel
        </Btn>
        <Btn
          disabled={!diff || change.isPending}
          onClick={() =>
            change.mutate(pending, {
              onSuccess: () => {
                toast.success(`Role changed to ${next?.name}`)
                onClose()
              },
            })
          }
        >
          {change.isPending ? 'Saving…' : 'Confirm change'}
        </Btn>
      </div>
    </Modal>
  )
}

/** "Remove Rafi Hasan?": type the name to confirm. */
export function RemoveMemberDialog({
  workspaceId,
  member,
  onClose,
  onRemoved,
}: {
  workspaceId: string
  member: Target
  onClose: () => void
  onRemoved: () => void
}) {
  const remove = useRemoveMember(workspaceId, member.user_id)
  const [typed, setTyped] = useState('')
  const matches = typed.trim().toLowerCase() === member.name.toLowerCase()
  return (
    <Modal
      open
      onClose={onClose}
      title={`Remove ${member.name}?`}
      description="They lose access right away and their seat is freed. Their past messages stay in the chat history."
    >
      <div className="flex flex-col gap-1.5">
        <label htmlFor="remove-confirm" className="text-[13px] font-bold">
          Type <strong>{member.name}</strong> to confirm
        </label>
        <input
          id="remove-confirm"
          className={inputClass}
          value={typed}
          autoComplete="off"
          placeholder={member.name}
          onChange={(e) => setTyped(e.target.value)}
          aria-invalid={remove.isError}
          aria-describedby={remove.isError ? 'remove-error' : undefined}
        />
        <ErrorText id="remove-error">{remove.isError && errorMessage(remove.error)}</ErrorText>
      </div>
      <div className="flex justify-end gap-2.5">
        <Btn variant="outline" onClick={onClose}>
          Cancel
        </Btn>
        <Btn
          variant="danger"
          disabled={!matches || remove.isPending}
          onClick={() =>
            remove.mutate(typed.trim(), {
              onSuccess: () => {
                toast.success(`${member.name} was removed`)
                onRemoved()
              },
            })
          }
        >
          {remove.isPending ? 'Removing…' : 'Remove member'}
        </Btn>
      </div>
    </Modal>
  )
}
