import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  AlertTriangleIcon,
  CheckIcon,
  InfoIcon,
  LinkIcon,
  PauseCircleIcon,
  PlusIcon,
  ShieldIcon,
  UserMinusIcon,
  XIcon,
} from 'lucide-react'
import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import { Skeleton } from '#/components/ui/skeleton'
import { numbersQueryOptions } from '#/features/onboarding'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import {
  useAddOverride,
  useLeaveWorkspace,
  useRestoreMember,
  useRevokeOverride,
  useSuspendMember,
  useUpdateMemberAccess,
} from '../../api/team.mutations'
import {
  accessLogQueryOptions,
  memberQueryOptions,
  permissionsQueryOptions,
  rolesQueryOptions,
  seatsQueryOptions,
  teamsQueryOptions,
} from '../../api/team.queries'
import { useAccess } from '../../hooks/use-access'
import { LOG_ACTIONS, SCOPES, TONE } from '../../schemas/team.schema'
import type { ChatScope, MemberDetail, Override, Permission, Role, Team } from '../../types'
import { PersonAvatar } from '../kit/avatar'
import { Btn } from '../kit/btn'
import { ErrorText, inputClass, selectClass } from '../kit/field'
import { formatDate, formatDateTime, formatShortDate, relativeTime } from '../kit/format'
import { describeLog } from '../kit/log-format'
import { Modal } from '../kit/modal'
import { Breadcrumb, Panel, PanelHeader, SectionLabel } from '../kit/panel'
import { Pill } from '../kit/pill'
import { RadioOption } from '../kit/radio'
import { ChangeRoleDialog, RemoveMemberDialog } from './member-dialogs'

type Props = { workspaceId: string; userId: string; meId: string }

const STATUS_DOT = { online: 'bg-brand', away: 'bg-warning', offline: 'bg-neutral' } as const
const AVAILABILITY = { online: 'Online', away: 'Away', offline: 'Offline' } as const

export function MemberPage({ workspaceId, userId, meId }: Props) {
  const member = useQuery(memberQueryOptions(workspaceId, userId))
  const roles = useQuery(rolesQueryOptions(workspaceId))
  const catalog = useQuery(permissionsQueryOptions())
  const { can } = useAccess(workspaceId)
  const [removed, setRemoved] = useState<{ name: string } | null>(null)

  if (removed) return <RemovedState workspaceId={workspaceId} name={removed.name} />
  if (member.isError) {
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
        <Breadcrumb current="Member" />
        <Panel role="alert" className="text-destructive">
          {errorMessage(member.error, 'Could not load this member.')}
        </Panel>
      </main>
    )
  }
  if (!member.data || !roles.data || !catalog.data) {
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
        <Skeleton className="h-28 w-full rounded-xl" />
        <Skeleton className="h-72 w-full rounded-xl" />
      </main>
    )
  }

  const m = member.data
  const self = m.user_id === meId
  const owner = m.role_key === 'owner'
  // The API enforces these too (cannot_change_self, owner_protected); this just hides dead ends.
  const editable = can('team.roles') && !self && !owner
  const role = roles.data.find((r) => r.id === m.role_id)
  const label = (k: string) => catalog.data.find((p) => p.key === k)?.label ?? k
  const first = m.name.split(' ')[0]
  const suspended = m.status === 'suspended'

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
      <Breadcrumb current={m.name} />

      <section
        aria-label="Member summary"
        className="flex flex-wrap items-center gap-[18px] rounded-xl border bg-card px-5 py-5 shadow-card sm:px-[22px]"
      >
        <PersonAvatar name={m.name} seed={m.user_id} size={64}>
          <span
            aria-hidden="true"
            className={cn(
              'absolute right-0.5 bottom-0.5 size-3 box-content rounded-full border-[3px] border-card',
              suspended ? 'bg-destructive' : STATUS_DOT[m.availability],
            )}
          />
        </PersonAvatar>
        <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-1">
          <div className="flex flex-wrap items-center gap-2.5">
            <h1 className="text-2xl font-extrabold tracking-[-0.02em]">{m.name}</h1>
            <Pill tone={role?.color ?? 'gray'}>{m.role_name}</Pill>
            {suspended && <Pill tone="red">Suspended</Pill>}
            {self && <Pill tone="gray">You</Pill>}
          </div>
          <span className="text-muted-foreground">
            {m.email} · {suspended ? 'Suspended' : AVAILABILITY[m.availability]}
          </span>
        </div>
        <dl className="flex flex-wrap gap-x-7 gap-y-3">
          <Stat label="Joined" value={formatDate(m.joined_at)} />
          <Stat label="Last active" value={relativeTime(m.last_active_at)} />
          <Stat label="Open chats" value="—" />
          <Stat label="Seat" value={m.uses_seat ? 'Uses 1 seat' : 'Free, no seat'} />
        </dl>
      </section>

      {suspended && (
        <SuspendedBanner
          workspaceId={workspaceId}
          member={m}
          canRestore={can('team.remove') && !self}
        />
      )}

      <div className="flex flex-wrap items-start gap-5">
        <div className="flex min-w-0 flex-[2_1_560px] flex-col gap-5">
          <RoleCard
            member={m}
            role={role}
            label={label}
            workspaceId={workspaceId}
            editable={editable}
          />
          <AccessCard workspaceId={workspaceId} member={m} editable={editable} />
          {can('team.roles') && !owner && (
            <ExtraCard
              workspaceId={workspaceId}
              member={m}
              role={role}
              catalog={catalog.data}
              label={label}
              editable={editable}
            />
          )}
          {self && !owner && <LeaveCard workspaceId={workspaceId} />}
          {!self && can('team.remove') && (
            <ActionsCard
              workspaceId={workspaceId}
              member={m}
              owner={owner}
              onRemoved={() => setRemoved({ name: m.name })}
            />
          )}
        </div>
        <aside className="flex min-w-0 flex-[1_1_320px] flex-col gap-5">
          <EffectiveCard member={m} role={role} label={label} first={self ? 'You' : first} />
          {can('team.roles') && (
            <ActivityCard workspaceId={workspaceId} userId={m.user_id} label={label} />
          )}
        </aside>
      </div>
    </main>
  )
}

function Stat({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex flex-col gap-0.5">
      <dt className="text-xs font-semibold text-muted-foreground">{label}</dt>
      <dd className="font-bold">{value}</dd>
    </div>
  )
}

function SuspendedBanner({
  workspaceId,
  member,
  canRestore,
}: {
  workspaceId: string
  member: MemberDetail
  canRestore: boolean
}) {
  const restore = useRestoreMember(workspaceId, member.user_id)
  const first = member.name.split(' ')[0]
  return (
    <div
      role="status"
      className="flex flex-wrap items-center gap-3 rounded-lg border border-destructive bg-destructive-soft px-4 py-3"
    >
      <PauseCircleIcon className="size-5 flex-none text-destructive" aria-hidden="true" />
      <span className="flex-[1_1_300px] text-sm">
        <strong>
          Access suspended{member.suspended_at && ` on ${formatDate(member.suspended_at)}`}.
        </strong>{' '}
        {first} can’t log in or reply. Their role, teams and history are kept, and their seat stays
        reserved.
        {member.suspend_reason && ` Reason: ${member.suspend_reason}`}
      </span>
      {canRestore && (
        <Btn
          size="md"
          disabled={restore.isPending}
          onClick={() =>
            restore.mutate(undefined, {
              onSuccess: () => toast.success(`${first}’s access is restored`),
              onError: (e) => toast.error(errorMessage(e)),
            })
          }
        >
          Restore access
        </Btn>
      )}
    </div>
  )
}

// ---------- Role ----------

function RoleCard({
  workspaceId,
  member,
  role,
  label,
  editable,
}: {
  workspaceId: string
  member: MemberDetail
  role?: Role
  label: (k: string) => string
  editable: boolean
}) {
  const [open, setOpen] = useState(false)
  const owner = member.role_key === 'owner'
  const perms = owner
    ? ['Everything, including billing and transferring ownership']
    : (role?.permissions ?? []).map(label)
  const tone = TONE[role?.color ?? 'blue']
  return (
    <Panel aria-labelledby="role-h" className="gap-3.5">
      <PanelHeader
        id="role-h"
        title="Role"
        description="The role sets the base permissions. Extra permissions below add to it."
        action={
          editable && (
            <Btn variant="outline" size="md" onClick={() => setOpen(true)}>
              Change role
            </Btn>
          )
        }
      />
      <div className="flex items-start gap-3.5 rounded-lg bg-muted p-4">
        <span
          className={cn(
            'flex size-11 flex-none items-center justify-center rounded-lg',
            tone.soft,
            tone.text,
          )}
        >
          <ShieldIcon className="size-[18px]" aria-hidden="true" />
        </span>
        <div className="flex min-w-0 flex-1 flex-col gap-2">
          <div className="flex flex-col leading-[1.35]">
            <span className="text-base font-extrabold">{member.role_name}</span>
            <span className="text-[13px] text-muted-foreground">{role?.description}</span>
          </div>
          <ul className="flex flex-wrap gap-1.5">
            {perms.map((p) => (
              <li
                key={p}
                className="inline-flex items-center gap-1 rounded-full border bg-card px-2.5 py-1 text-xs font-semibold"
              >
                <CheckIcon className="size-3.5 text-brand-text" aria-hidden="true" />
                {p}
              </li>
            ))}
            {perms.length === 0 && (
              <li className="text-[13px] text-muted-foreground">No permissions</li>
            )}
          </ul>
        </div>
      </div>
      {open && (
        <ChangeRoleDialog
          workspaceId={workspaceId}
          member={member}
          onClose={() => setOpen(false)}
        />
      )}
    </Panel>
  )
}

// ---------- Teams and numbers ----------

function AccessCard({
  workspaceId,
  member,
  editable,
}: {
  workspaceId: string
  member: MemberDetail
  editable: boolean
}) {
  const teams = useQuery(teamsQueryOptions(workspaceId))
  const numbers = useQuery(numbersQueryOptions('', workspaceId))
  const save = useUpdateMemberAccess(workspaceId, member.user_id)
  const [adding, setAdding] = useState(false)
  const first = member.name.split(' ')[0]
  const owner = member.role_key === 'owner'
  const mine = (teams.data ?? []).filter((t) => member.team_ids.includes(t.id))
  const others = (teams.data ?? []).filter((t) => !member.team_ids.includes(t.id))

  function change(body: Parameters<typeof save.mutate>[0], done: string) {
    save.mutate(body, {
      onSuccess: () => toast.success(done),
      onError: (e) => toast.error(errorMessage(e)),
    })
  }
  const usedBy = (numberId: string) =>
    (teams.data ?? [])
      .filter((t: Team) => t.number_id === numberId)
      .map((t) => t.name)
      .join(', ')

  return (
    <Panel aria-labelledby="teams-h" className="gap-3.5">
      <PanelHeader
        id="teams-h"
        title="Teams and number access"
        description="Agents only see chats routed to their teams, on the numbers ticked here."
      />
      <div className="flex flex-col gap-2">
        <SectionLabel>Teams</SectionLabel>
        {owner ? (
          <span className="text-sm text-muted-foreground">The Owner sees every team.</span>
        ) : (
          <div className="flex flex-wrap items-center gap-2">
            {mine.map((t) => (
              <span
                key={t.id}
                className={cn(
                  'inline-flex h-[34px] items-center gap-1 rounded-full pr-1 pl-3 text-[13px] font-bold',
                  TONE[t.color].soft,
                  TONE[t.color].text,
                  !editable && 'pr-3',
                )}
              >
                {t.name}
                {editable && (
                  <button
                    type="button"
                    aria-label={`Remove from ${t.name}`}
                    disabled={save.isPending}
                    onClick={() =>
                      change(
                        { team_ids: member.team_ids.filter((x) => x !== t.id) },
                        `Removed from ${t.name}`,
                      )
                    }
                    className="flex size-7 items-center justify-center rounded-full hover:bg-card"
                  >
                    <XIcon className="size-3.5" aria-hidden="true" />
                  </button>
                )}
              </span>
            ))}
            {mine.length === 0 && !editable && (
              <span className="text-sm text-muted-foreground">No teams</span>
            )}
            {editable && (
              <button
                type="button"
                aria-expanded={adding}
                onClick={() => setAdding(!adding)}
                className="h-[34px] rounded-full border border-dashed px-3 text-[13px] font-bold text-brand-text hover:bg-muted"
              >
                + Add to team
              </button>
            )}
          </div>
        )}
        {adding && editable && (
          <div className="flex flex-wrap gap-2 rounded-lg border bg-muted p-2.5">
            {others.map((t) => (
              <Btn
                key={t.id}
                variant="outline"
                size="sm"
                disabled={save.isPending}
                onClick={() => {
                  change({ team_ids: [...member.team_ids, t.id] }, `Added to ${t.name}`)
                  setAdding(false)
                }}
              >
                {t.name}
              </Btn>
            ))}
            {others.length === 0 && (
              <span className="px-1 text-[13px] text-muted-foreground">
                {teams.data?.length
                  ? `${first} is already in every team.`
                  : 'There are no teams yet.'}
              </span>
            )}
          </div>
        )}
      </div>

      <div className="flex flex-col">
        <SectionLabel className="mb-1">WhatsApp numbers</SectionLabel>
        {owner && (
          <span className="text-sm text-muted-foreground">The Owner sees every number.</span>
        )}
        {!owner &&
          (numbers.data ?? []).map((n) => {
            const on = member.number_ids.includes(n.id)
            return (
              <label
                key={n.id}
                className={cn(
                  'flex flex-wrap items-center gap-3 border-b py-3',
                  editable ? 'cursor-pointer' : 'cursor-default',
                )}
              >
                <input
                  type="checkbox"
                  className="size-[18px] accent-primary"
                  checked={on}
                  disabled={!editable || save.isPending}
                  onChange={() =>
                    change(
                      {
                        number_ids: on
                          ? member.number_ids.filter((x) => x !== n.id)
                          : [...member.number_ids, n.id],
                      },
                      on
                        ? `Removed access to ${n.verified_name}`
                        : `Gave access to ${n.verified_name}`,
                    )
                  }
                />
                <span className="flex flex-1 flex-col leading-[1.3]">
                  <span className="font-bold">{n.verified_name}</span>
                  <span className="text-xs text-muted-foreground">{n.display_phone_number}</span>
                </span>
                <span className="text-xs text-muted-foreground">
                  {usedBy(n.id) ? `Used by ${usedBy(n.id)}` : 'Not linked to a team'}
                </span>
              </label>
            )
          })}
        {!owner && numbers.data?.length === 0 && (
          <span className="text-sm text-muted-foreground">
            No WhatsApp numbers are connected yet.
          </span>
        )}
        {!owner && numbers.data && numbers.data.length > 0 && (
          <span className="pt-2 text-xs text-muted-foreground">
            None ticked means every number their teams use.
          </span>
        )}
      </div>

      {!owner && (
        <div className="flex flex-col gap-2">
          <SectionLabel>Chats they see</SectionLabel>
          <div role="radiogroup" aria-label="Chats they see" className="flex flex-wrap gap-2">
            {[{ value: '' as const, label: 'Role default' }, ...SCOPES].map((s) => {
              const current = (member.chat_scope ?? '') === s.value
              return (
                <RadioOption
                  key={s.value || 'default'}
                  name="chat-scope"
                  checked={current}
                  disabled={!editable || save.isPending}
                  onSelect={() =>
                    !current &&
                    change(
                      { chat_scope: s.value as ChatScope | '' },
                      `Chat access set to ${s.label.toLowerCase()}`,
                    )
                  }
                  className={cn(
                    'inline-flex h-[34px] items-center rounded-full border px-3 text-[13px] font-bold',
                    current
                      ? 'border-primary bg-brand-soft text-brand-text'
                      : 'bg-card text-foreground hover:bg-muted',
                  )}
                >
                  {s.value === ''
                    ? `Role default (${SCOPES.find((x) => x.value === member.role_chat_scope)?.label})`
                    : s.label}
                </RadioOption>
              )
            })}
          </div>
        </div>
      )}
    </Panel>
  )
}

// ---------- Extra permissions ----------

function overrideBadge(o: Override): { text: string; tone: 'gray' | 'amber' | 'green' | 'red' } {
  if (o.revoked_at) return { text: 'Revoked', tone: 'gray' }
  if (!o.active) return { text: 'Expired', tone: 'gray' }
  if (o.effect === 'deny') return { text: 'Removed', tone: 'red' }
  if (o.expires_at) return { text: `Expires ${formatShortDate(o.expires_at)}`, tone: 'amber' }
  return { text: 'Active', tone: 'green' }
}

function ExtraCard({
  workspaceId,
  member,
  role,
  catalog,
  label,
  editable,
}: {
  workspaceId: string
  member: MemberDetail
  role?: Role
  catalog: Permission[]
  label: (k: string) => string
  editable: boolean
}) {
  const add = useAddOverride(workspaceId, member.user_id)
  const revoke = useRevokeOverride(workspaceId, member.user_id)
  const [open, setOpen] = useState(false)
  const [key, setKey] = useState('')
  const [until, setUntil] = useState('')
  const [reason, setReason] = useState('')
  const [confirm, setConfirm] = useState<Override | null>(null)
  const first = member.name.split(' ')[0]
  // Grant only what they don't already have.
  const choices = catalog.filter((p) => !member.effective_permissions.includes(p.key))
  const selected = key || choices[0]?.key || ''

  function submit(e: FormEvent) {
    e.preventDefault()
    if (!selected) return
    add.mutate(
      {
        permission_key: selected,
        effect: 'grant',
        reason: reason.trim() || undefined,
        expires_at: until ? new Date(`${until}T23:59:59`).toISOString() : undefined,
      },
      {
        onSuccess: () => {
          toast.success(`${label(selected)} granted to ${first}`)
          setOpen(false)
          setKey('')
          setReason('')
          setUntil('')
        },
      },
    )
  }

  function regrant(o: Override) {
    add.mutate(
      { permission_key: o.permission_key, effect: o.effect, reason: o.reason || undefined },
      {
        onSuccess: () => toast.success(`${label(o.permission_key)} granted again`),
        onError: (e) => toast.error(errorMessage(e)),
      },
    )
  }

  return (
    <Panel aria-labelledby="extra-h" className="gap-3.5">
      <PanelHeader
        id="extra-h"
        title="Extra permissions"
        description="Give one person a permission their role doesn’t have, for a set time if you like."
        action={
          editable &&
          !open && (
            <Btn size="md" onClick={() => setOpen(true)} disabled={choices.length === 0}>
              <PlusIcon className="size-[18px]" aria-hidden="true" />
              Grant a permission
            </Btn>
          )
        }
      />
      {open && (
        <form
          onSubmit={submit}
          noValidate
          className="flex flex-col gap-3 rounded-lg border bg-muted p-4"
        >
          <div className="flex flex-wrap gap-3">
            <label className="flex flex-[1_1_280px] flex-col gap-1.5">
              <span className="text-[13px] font-bold">Permission</span>
              <select
                className={cn(selectClass, 'bg-card')}
                value={selected}
                onChange={(e) => setKey(e.target.value)}
              >
                {choices.map((p) => (
                  <option key={p.key} value={p.key}>
                    {p.label}
                  </option>
                ))}
              </select>
            </label>
            <label className="flex flex-[1_1_180px] flex-col gap-1.5">
              <span className="text-[13px] font-bold">
                Expires <span className="font-medium text-muted-foreground">(optional)</span>
              </span>
              <input
                type="date"
                className={cn(inputClass, 'bg-card')}
                value={until}
                min={new Date().toISOString().slice(0, 10)}
                onChange={(e) => setUntil(e.target.value)}
              />
            </label>
          </div>
          <label className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">
              Reason{' '}
              <span className="font-medium text-muted-foreground">(shown in the access log)</span>
            </span>
            <input
              type="text"
              className={cn(inputClass, 'bg-card')}
              value={reason}
              maxLength={200}
              onChange={(e) => setReason(e.target.value)}
              placeholder="e.g. Covering Returns while Meera is on leave"
            />
          </label>
          <ErrorText>{add.isError && errorMessage(add.error)}</ErrorText>
          <div className="flex flex-wrap items-center gap-2.5">
            <span className="flex-[1_1_240px] text-xs text-muted-foreground">
              {first} gets an email. The permission is removed automatically on the expiry date.
            </span>
            <Btn variant="outline" size="md" onClick={() => setOpen(false)}>
              Cancel
            </Btn>
            <Btn type="submit" size="md" disabled={add.isPending || !selected}>
              {add.isPending ? 'Granting…' : 'Grant permission'}
            </Btn>
          </div>
        </form>
      )}
      <ul className="flex flex-col">
        {member.overrides.map((o) => {
          const badge = overrideBadge(o)
          const off = !o.active
          const inRole = role?.permissions.includes(o.permission_key)
          return (
            <li
              key={o.id}
              className={cn(
                'flex flex-wrap items-center gap-3 border-t py-3.5',
                off && 'opacity-60',
              )}
            >
              <span
                className={cn(
                  'flex size-10 flex-none items-center justify-center rounded-[10px]',
                  off ? 'bg-neutral-soft text-neutral' : 'bg-info-soft text-info',
                )}
              >
                <LinkIcon className="size-[18px]" aria-hidden="true" />
              </span>
              <div className="flex min-w-0 flex-[1_1_240px] flex-col gap-0.5">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={cn('font-bold', off && 'line-through')}>
                    {label(o.permission_key)}
                  </span>
                  <Pill tone={badge.tone} className="px-2 py-px text-[11px]">
                    {badge.text}
                  </Pill>
                </div>
                <span className="text-xs text-muted-foreground">
                  {o.effect === 'grant' ? 'Granted' : 'Removed'} by {o.granted_by_name || 'someone'}
                  , {formatShortDate(o.created_at)}
                  {o.revoked_at
                    ? ` · revoked ${formatShortDate(o.revoked_at)}`
                    : o.expires_at
                      ? ` · expires ${formatShortDate(o.expires_at)}`
                      : ' · no expiry'}
                </span>
                {o.reason && (
                  <span className="text-xs text-muted-foreground">Reason: {o.reason}</span>
                )}
              </div>
              {editable && o.active && (
                <Btn variant="danger-soft" size="sm" className="h-9" onClick={() => setConfirm(o)}>
                  Revoke
                </Btn>
              )}
              {editable &&
                off &&
                !inRole &&
                !member.effective_permissions.includes(o.permission_key) && (
                  <Btn
                    variant="outline"
                    size="sm"
                    className="h-9"
                    disabled={add.isPending}
                    onClick={() => regrant(o)}
                  >
                    Grant again
                  </Btn>
                )}
            </li>
          )
        })}
        {member.overrides.length === 0 && (
          <li className="border-t py-4 text-sm text-muted-foreground">
            No extra permissions. {first} has exactly what the {member.role_name} role gives.
          </li>
        )}
      </ul>

      {confirm && (
        <Modal
          open
          onClose={() => setConfirm(null)}
          title={`Revoke “${label(confirm.permission_key)}” from ${first}?`}
          width="sm:max-w-[480px]"
          icon={
            <span className="flex size-11 flex-none items-center justify-center rounded-lg bg-destructive-soft text-destructive">
              <AlertTriangleIcon className="size-5" aria-hidden="true" />
            </span>
          }
        >
          <ul className="flex list-disc flex-col gap-1.5 pl-5 text-sm text-muted-foreground">
            <li>Takes effect immediately, even if they’re logged in.</li>
            <li>Anything they already did stays, for example exported files or sent messages.</li>
            <li>The change is saved in the access log.</li>
          </ul>
          <ErrorText>{revoke.isError && errorMessage(revoke.error)}</ErrorText>
          <div className="flex justify-end gap-2.5">
            <Btn variant="outline" onClick={() => setConfirm(null)}>
              Keep it
            </Btn>
            <Btn
              variant="danger"
              disabled={revoke.isPending}
              onClick={() =>
                revoke.mutate(confirm.id, {
                  onSuccess: () => {
                    toast.success(`${label(confirm.permission_key)} revoked`)
                    setConfirm(null)
                  },
                })
              }
            >
              {revoke.isPending ? 'Revoking…' : 'Revoke permission'}
            </Btn>
          </div>
        </Modal>
      )}
    </Panel>
  )
}

// ---------- Access actions ----------

function ActionsCard({
  workspaceId,
  member,
  owner,
  onRemoved,
}: {
  workspaceId: string
  member: MemberDetail
  owner: boolean
  onRemoved: () => void
}) {
  const suspend = useSuspendMember(workspaceId, member.user_id)
  const restore = useRestoreMember(workspaceId, member.user_id)
  const [removing, setRemoving] = useState(false)
  const first = member.name.split(' ')[0]
  const suspended = member.status === 'suspended'
  return (
    <section
      aria-labelledby="danger-h"
      className="flex flex-col rounded-xl border border-destructive bg-card px-5 py-5 shadow-card sm:px-[22px]"
    >
      <h2 id="danger-h" className="mb-1.5 text-[17px] font-bold text-destructive">
        Access actions
      </h2>
      {!owner && (
        <>
          <div className="flex flex-wrap items-center gap-3 border-t py-3.5">
            <div className="flex min-w-0 flex-[1_1_300px] flex-col leading-[1.35]">
              <span className="font-bold">Suspend access</span>
              <span className="text-[13px] text-muted-foreground">
                Blocks login right away but keeps their role, teams and history. Good for leave or a
                lost phone.
              </span>
            </div>
            {suspended ? (
              <Btn
                size="md"
                disabled={restore.isPending}
                onClick={() =>
                  restore.mutate(undefined, {
                    onSuccess: () => toast.success(`${first}’s access is restored`),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Restore access
              </Btn>
            ) : (
              <Btn
                variant="danger-outline"
                size="md"
                disabled={suspend.isPending}
                onClick={() =>
                  suspend.mutate('', {
                    onSuccess: () => toast.success(`${first}’s access is suspended`),
                    onError: (e) => toast.error(errorMessage(e)),
                  })
                }
              >
                Suspend
              </Btn>
            )}
          </div>
          <div className="flex flex-wrap items-center gap-3 border-t py-3.5">
            <div className="flex min-w-0 flex-[1_1_300px] flex-col leading-[1.35]">
              <span className="font-bold">Remove from workspace</span>
              <span className="text-[13px] text-muted-foreground">
                Removes {first} for good and frees their seat. Their past messages stay in the chat
                history.
              </span>
            </div>
            <Btn variant="danger" size="md" onClick={() => setRemoving(true)}>
              Remove member
            </Btn>
          </div>
        </>
      )}
      <div className="mt-1.5 flex items-start gap-2.5 rounded-lg bg-muted px-3.5 py-3 text-[13px]">
        <InfoIcon className="mt-0.5 size-4 flex-none text-muted-foreground" aria-hidden="true" />
        <span>
          {owner ? (
            <>
              The Owner can’t be suspended or removed. <strong>Transfer ownership</strong> from
              Settings first; the old Owner becomes an Admin.
            </>
          ) : (
            <>
              <strong>Transfer ownership</strong> only appears on the Owner’s profile. The Owner
              picks an Admin to become the new Owner and confirms with their password; the old Owner
              becomes an Admin.
            </>
          )}
        </span>
      </div>
      {removing && (
        <RemoveMemberDialog
          workspaceId={workspaceId}
          member={member}
          onClose={() => setRemoving(false)}
          onRemoved={onRemoved}
        />
      )}
    </section>
  )
}

function LeaveCard({ workspaceId }: { workspaceId: string }) {
  const navigate = useNavigate()
  const leave = useLeaveWorkspace(workspaceId)
  const [open, setOpen] = useState(false)
  return (
    <section
      aria-labelledby="leave-h"
      className="flex flex-col rounded-xl border border-destructive bg-card px-5 py-5 shadow-card sm:px-[22px]"
    >
      <h2 id="leave-h" className="mb-1.5 text-[17px] font-bold text-destructive">
        Access actions
      </h2>
      <div className="flex flex-wrap items-center gap-3 border-t py-3.5">
        <div className="flex min-w-0 flex-[1_1_300px] flex-col leading-[1.35]">
          <span className="font-bold">Leave this workspace</span>
          <span className="text-[13px] text-muted-foreground">
            You lose access at once. Someone has to invite you to come back.
          </span>
        </div>
        <Btn variant="danger-outline" size="md" onClick={() => setOpen(true)}>
          Leave workspace
        </Btn>
      </div>
      {open && (
        <Modal
          open
          onClose={() => setOpen(false)}
          title="Leave this workspace?"
          description="You’ll need a new invite to come back."
          width="sm:max-w-[480px]"
        >
          <ErrorText>{leave.isError && errorMessage(leave.error)}</ErrorText>
          <div className="flex justify-end gap-2.5">
            <Btn variant="outline" onClick={() => setOpen(false)}>
              Stay
            </Btn>
            <Btn
              variant="danger"
              disabled={leave.isPending}
              onClick={() => leave.mutate(undefined, { onSuccess: () => navigate({ to: '/' }) })}
            >
              {leave.isPending ? 'Leaving…' : 'Leave workspace'}
            </Btn>
          </div>
        </Modal>
      )}
    </section>
  )
}

// ---------- Right column ----------

function EffectiveCard({
  member,
  role,
  label,
  first,
}: {
  member: MemberDetail
  role?: Role
  label: (k: string) => string
  first: string
}) {
  const owner = member.role_key === 'owner'
  const suspended = member.status === 'suspended'
  const extra = new Map(
    member.overrides
      .filter((o) => o.active && o.effect === 'grant')
      .map((o) => [o.permission_key, o]),
  )
  const rows = member.effective_permissions.map((k) => {
    const o = extra.get(k)
    const fromRole = owner || role?.permissions.includes(k)
    return {
      key: k,
      src: fromRole
        ? 'Role'
        : o?.expires_at
          ? `Extra · until ${formatShortDate(o.expires_at)}`
          : 'Extra',
    }
  })
  return (
    <Panel aria-labelledby="eff-h" className="gap-2.5">
      <PanelHeader
        id="eff-h"
        title={first === 'You' ? 'What you can do now' : `What ${first} can do now`}
        description="Role + extra permissions, updated live"
      />
      <ul className={cn('flex flex-col', suspended && 'opacity-60')}>
        {rows.map((r) => (
          <li key={r.key} className="flex items-center gap-2.5 py-1.5">
            <span className="flex size-6 flex-none items-center justify-center rounded-full bg-brand-soft text-brand-text">
              <CheckIcon className="size-3.5" aria-hidden="true" />
            </span>
            <span className="flex-1 text-sm font-semibold">{label(r.key)}</span>
            <span className="text-[11px] font-bold text-muted-foreground">{r.src}</span>
          </li>
        ))}
        {rows.length === 0 && <li className="text-sm text-muted-foreground">Nothing yet.</li>}
      </ul>
      {suspended && (
        <span className="rounded-md bg-destructive-soft px-2.5 py-1.5 text-xs font-bold text-destructive">
          Paused while access is suspended
        </span>
      )}
    </Panel>
  )
}

function ActivityCard({
  workspaceId,
  userId,
  label,
}: {
  workspaceId: string
  userId: string
  label: (k: string) => string
}) {
  const log = useInfiniteQuery(accessLogQueryOptions(workspaceId, { member: userId }, 5))
  const entries = log.data?.pages[0]?.entries.slice(0, 5) ?? []
  return (
    <Panel aria-labelledby="act-h" className="gap-2.5">
      <div className="flex items-center justify-between gap-3">
        <h2 id="act-h" className="text-[17px] font-bold">
          Recent activity
        </h2>
        <Link to="/team/log" search={{ member: userId }} className="text-[13px] font-bold">
          Full access log
        </Link>
      </div>
      {log.isPending && <Skeleton className="h-32 w-full" />}
      <ol className="flex flex-col">
        {entries.map((e) => (
          <li key={e.id} className="flex items-start gap-2.5 border-b py-2.5 last:border-b-0">
            <span
              aria-hidden="true"
              className={cn(
                'mt-[7px] size-2 flex-none rounded-full',
                TONE[LOG_ACTIONS[e.action]?.tone ?? 'gray'].solid,
              )}
            />
            <span className="flex flex-col gap-0.5 leading-[1.35]">
              <span className="text-[13px]">
                <strong>{e.actor_name || 'Relay'}</strong> {describeLog(e, label)}
              </span>
              <span className="text-xs text-muted-foreground">{formatDateTime(e.created_at)}</span>
            </span>
          </li>
        ))}
        {!log.isPending && entries.length === 0 && (
          <li className="text-sm text-muted-foreground">No changes recorded yet.</li>
        )}
      </ol>
    </Panel>
  )
}

// ---------- Removed ----------

function RemovedState({ workspaceId, name }: { workspaceId: string; name: string }) {
  const seats = useQuery(seatsQueryOptions(workspaceId))
  const taken = seats.data ? seats.data.used + seats.data.reserved : 0
  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:px-7 sm:pt-6 sm:pb-10">
      <Breadcrumb current={name} />
      <section
        aria-label="Member removed"
        className="mx-auto my-6 flex w-full max-w-[640px] flex-col items-center gap-4 rounded-[18px] border bg-card px-7 py-9 text-center shadow-card"
      >
        <span className="flex size-14 items-center justify-center rounded-full bg-neutral-soft text-neutral">
          <UserMinusIcon className="size-6" aria-hidden="true" />
        </span>
        <h1 className="text-2xl font-extrabold tracking-[-0.02em]">{name} was removed</h1>
        <p className="text-muted-foreground">
          They can no longer log in. Their past messages stay in the chat history under their name.
        </p>
        <div className="flex w-full flex-col border-t text-left">
          <div className="flex justify-between gap-3 border-b py-3">
            <span className="text-muted-foreground">Devices</span>
            <span className="font-bold">Signed out everywhere</span>
          </div>
          {seats.data && (
            <div className="flex justify-between gap-3 border-b py-3">
              <span className="text-muted-foreground">Seats</span>
              <span className="font-bold">
                {taken} of {seats.data.limit} used
              </span>
            </div>
          )}
        </div>
        <div className="flex flex-wrap justify-center gap-2.5">
          <Link
            to="/team"
            className="inline-flex h-11 items-center rounded-lg bg-primary px-[18px] font-bold text-primary-foreground no-underline hover:bg-primary/90 hover:text-primary-foreground"
          >
            Back to Team
          </Link>
          <Link
            to="/team/log"
            className="inline-flex h-11 items-center rounded-lg border bg-card px-[18px] font-bold text-foreground no-underline hover:bg-muted"
          >
            View access log
          </Link>
          <Link
            to="/team/invite"
            className="inline-flex h-11 items-center rounded-lg px-[18px] font-bold text-brand-text no-underline hover:bg-muted"
          >
            Re-invite {name.split(' ')[0]}
          </Link>
        </div>
      </section>
    </main>
  )
}
