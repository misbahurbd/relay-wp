import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import {
  useCreateTeam,
  useDeleteTeam,
  useSetTeamMembers,
  useUpdateTeam,
} from '../../api/team.mutations'
import { COLORS, ROUTING, TONE, teamSchema } from '../../schemas/team.schema'
import type { Member, RoleColor, Team, TeamInput } from '../../types'
import { PersonAvatar } from '../kit/avatar'
import { Btn } from '../kit/btn'
import { ErrorText, inputClass, selectClass } from '../kit/field'
import { Modal } from '../kit/modal'
import { SectionLabel } from '../kit/panel'
import { RadioOption } from '../kit/radio'

type NumberOption = { id: string; display_phone_number: string; verified_name: string }

/** Create or edit a team: details, number, routing and who is in it. */
export function TeamDialog({
  workspaceId,
  team,
  people,
  numbers,
  onClose,
}: {
  workspaceId: string
  team?: Team
  people: Member[]
  numbers: NumberOption[]
  onClose: () => void
}) {
  const create = useCreateTeam(workspaceId)
  const update = useUpdateTeam(workspaceId)
  const setMembers = useSetTeamMembers(workspaceId)
  const remove = useDeleteTeam(workspaceId)
  const [name, setName] = useState(team?.name ?? '')
  const [description, setDescription] = useState(team?.description ?? '')
  const [color, setColor] = useState<RoleColor>(team?.color ?? 'green')
  const [numberId, setNumberId] = useState(team?.number_id ?? '')
  const [routing, setRouting] = useState<NonNullable<TeamInput['routing_method']>>(
    team?.routing_method ?? 'round_robin',
  )
  const [ids, setIds] = useState<string[]>(team?.member_ids ?? [])
  const [nameError, setNameError] = useState<string>()
  const [error, setError] = useState<string>()
  const pending = create.isPending || update.isPending || setMembers.isPending

  async function submit(e: FormEvent) {
    e.preventDefault()
    const parsed = teamSchema.safeParse({ name, description })
    if (!parsed.success) {
      setNameError(parsed.error.flatten().fieldErrors.name?.[0])
      return
    }
    setNameError(undefined)
    setError(undefined)
    const body: TeamInput = {
      ...parsed.data,
      color,
      routing_method: routing,
      number_id: numberId || undefined,
    }
    try {
      let teamId = team?.id
      if (team) await update.mutateAsync({ teamId: team.id, body })
      else teamId = (await create.mutateAsync(body)).id
      const before = team?.member_ids ?? []
      const changed = ids.length !== before.length || ids.some((id) => !before.includes(id))
      if (teamId && changed) await setMembers.mutateAsync({ teamId, userIds: ids })
      toast.success(team ? `${parsed.data.name} saved` : `${parsed.data.name} created`)
      onClose()
    } catch (err) {
      const msg = errorMessage(err)
      // A duplicate name belongs next to the Name field.
      if (/name/i.test(msg)) setNameError(msg)
      else setError(msg)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={team ? `Edit ${team.name}` : 'Create team'}
      description="Teams decide who gets which chats. Roles decide what people can do."
      width="sm:max-w-[600px]"
    >
      <form className="flex flex-col gap-4" onSubmit={submit} noValidate>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="team-name" className="text-[13px] font-bold">
            Team name
          </label>
          <input
            id="team-name"
            className={inputClass}
            value={name}
            onChange={(e) => setName(e.target.value)}
            placeholder="e.g. Sales"
            aria-invalid={!!nameError}
            aria-describedby={nameError ? 'team-name-error' : undefined}
          />
          <ErrorText id="team-name-error">{nameError}</ErrorText>
        </div>
        <div className="flex flex-col gap-1.5">
          <label htmlFor="team-description" className="text-[13px] font-bold">
            Description
          </label>
          <input
            id="team-description"
            className={inputClass}
            value={description}
            maxLength={200}
            onChange={(e) => setDescription(e.target.value)}
            placeholder="e.g. Pre-sale questions and catalog leads"
          />
        </div>
        <div role="radiogroup" aria-label="Colour" className="flex flex-col gap-1.5">
          <span className="text-[13px] font-bold">Colour</span>
          <div className="flex flex-wrap gap-2">
            {COLORS.map((c) => (
              <RadioOption
                key={c.value}
                name="team-color"
                label={c.label}
                checked={color === c.value}
                onSelect={() => setColor(c.value)}
                className={cn(
                  'flex size-11 items-center justify-center rounded-lg border-2 bg-card',
                  color === c.value ? 'border-foreground' : 'border-border',
                )}
              >
                <span className={cn('size-6 rounded-[7px]', TONE[c.value].solid)} />
              </RadioOption>
            ))}
          </div>
        </div>
        <div className="grid gap-3 sm:grid-cols-2">
          <div className="flex flex-col gap-1.5">
            <label htmlFor="team-number" className="text-[13px] font-bold">
              WhatsApp number
            </label>
            <select
              id="team-number"
              className={selectClass}
              value={numberId}
              onChange={(e) => setNumberId(e.target.value)}
            >
              <option value="">Any number</option>
              {numbers.map((n) => (
                <option key={n.id} value={n.id}>
                  {n.verified_name} ({n.display_phone_number})
                </option>
              ))}
            </select>
          </div>
          <div className="flex flex-col gap-1.5">
            <label htmlFor="team-routing" className="text-[13px] font-bold">
              Routing
            </label>
            <select
              id="team-routing"
              className={selectClass}
              value={routing}
              onChange={(e) => setRouting(e.target.value as typeof routing)}
            >
              {ROUTING.map((r) => (
                <option key={r.value} value={r.value}>
                  {r.label}
                </option>
              ))}
            </select>
          </div>
        </div>
        <div className="flex flex-col gap-2">
          <SectionLabel>Members · {ids.length}</SectionLabel>
          <ul className="flex max-h-64 flex-col gap-1.5 overflow-y-auto">
            {people
              .filter((p) => p.role_key !== 'owner' || ids.includes(p.user_id))
              .map((p) => {
                const on = ids.includes(p.user_id)
                return (
                  <li key={p.user_id}>
                    <label
                      className={cn(
                        'flex min-h-12 cursor-pointer items-center gap-3 rounded-lg border-2 px-3 py-2',
                        on
                          ? 'border-primary bg-brand-soft'
                          : 'border-border bg-card hover:bg-muted',
                      )}
                    >
                      <input
                        type="checkbox"
                        className="size-[18px] accent-primary"
                        checked={on}
                        onChange={(e) =>
                          setIds(
                            e.target.checked
                              ? [...ids, p.user_id]
                              : ids.filter((x) => x !== p.user_id),
                          )
                        }
                      />
                      <PersonAvatar name={p.name} seed={p.user_id} size={32} />
                      <span className="flex min-w-0 flex-col leading-[1.3]">
                        <span className="truncate text-sm font-bold">{p.name}</span>
                        <span className="truncate text-xs text-muted-foreground">
                          {p.role_name} · {p.email}
                        </span>
                      </span>
                    </label>
                  </li>
                )
              })}
          </ul>
        </div>
        <ErrorText>{error ?? (remove.isError ? errorMessage(remove.error) : undefined)}</ErrorText>
        <div className="flex flex-wrap items-center justify-between gap-2.5">
          {team ? (
            <Btn
              variant="danger-soft"
              size="md"
              disabled={remove.isPending}
              onClick={() =>
                remove.mutate(team.id, {
                  onSuccess: () => {
                    toast.success(`${team.name} deleted`)
                    onClose()
                  },
                })
              }
            >
              Delete team
            </Btn>
          ) : (
            <span />
          )}
          <div className="flex gap-2.5">
            <Btn variant="outline" onClick={onClose}>
              Cancel
            </Btn>
            <Btn type="submit" disabled={pending}>
              {pending ? 'Saving…' : team ? 'Save team' : 'Create team'}
            </Btn>
          </div>
        </div>
      </form>
    </Modal>
  )
}
