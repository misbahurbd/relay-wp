import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Link, useNavigate } from '@tanstack/react-router'
import {
  CheckIcon,
  ChevronDownIcon,
  MinusIcon,
  MoreVerticalIcon,
  PencilIcon,
  PlusIcon,
  SearchIcon,
  UserPlusIcon,
  UsersIcon,
} from 'lucide-react'
import { useDeferredValue, useState } from 'react'
import { toast } from 'sonner'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import { Skeleton } from '#/components/ui/skeleton'
import { numbersQueryOptions } from '#/features/onboarding'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import {
  invitationsQueryOptions,
  membersQueryOptions,
  permissionsQueryOptions,
  rolesQueryOptions,
  seatsQueryOptions,
  teamsQueryOptions,
  useResendInvitation,
  useRestoreMember,
  useRevokeInvitation,
  useSuspendMember,
} from '../api/team'
import { useAccess } from '../hooks/use-access'
import { GROUPS, PERMISSION_HINTS, ROUTING, TONE } from '../schemas/team'
import type { Invitation, Member, Role, Team } from '../types'
import { ChangeRoleDialog, RemoveMemberDialog } from './member-dialogs'
import { Btn, Panel, PanelHeader, PersonAvatar, Pill, relativeTime } from './parts'
import { TeamDialog } from './team-dialog'

export type TeamTab = 'members' | 'teams' | 'roles'

const TABS: { id: TeamTab; label: string }[] = [
  { id: 'members', label: 'Members' },
  { id: 'teams', label: 'Teams' },
  { id: 'roles', label: 'Roles & permissions' },
]

type Props = { workspaceId: string; meId: string; tab: TeamTab; onTab: (tab: TeamTab) => void }

export function TeamPage({ workspaceId, meId, tab, onTab }: Props) {
  const { can } = useAccess(workspaceId)
  const seats = useQuery(seatsQueryOptions(workspaceId))
  const taken = seats.data ? seats.data.used + seats.data.reserved : 0

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Team</h1>
          <p className="text-[15px] text-muted-foreground">
            Invite teammates, group them into teams and decide who gets which chats.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3">
          {seats.data && (
            <div className="flex min-w-[200px] flex-col gap-1.5 rounded-lg border bg-card px-3.5 py-2.5">
              <div className="flex justify-between gap-3 text-[13px]">
                <span className="font-bold">
                  {taken} of {seats.data.limit} seats used
                </span>
              </div>
              <div aria-hidden="true" className="h-1.5 rounded-[3px] bg-neutral-soft">
                <div
                  className="h-1.5 rounded-[3px] bg-brand"
                  style={{
                    width: `${Math.min(100, (taken / Math.max(1, seats.data.limit)) * 100)}%`,
                  }}
                />
              </div>
              <span className="text-xs text-muted-foreground">Growth plan · Viewers are free</span>
            </div>
          )}
          {can('team.invite') && (
            <Link
              to="/w/$workspaceId/team/invite"
              params={{ workspaceId }}
              className="flex h-11 items-center gap-2 rounded-lg bg-primary px-[18px] font-bold text-primary-foreground no-underline hover:bg-primary/90 hover:text-primary-foreground"
            >
              <UserPlusIcon className="size-[18px]" aria-hidden="true" />
              Invite member
            </Link>
          )}
        </div>
      </div>

      <div role="tablist" aria-label="Team sections" className="flex flex-wrap gap-1 border-b">
        {TABS.map((t) => (
          <button
            key={t.id}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            onClick={() => onTab(t.id)}
            className={cn(
              '-mb-px h-11 border-b-2 px-3.5 text-sm font-bold',
              tab === t.id
                ? 'border-primary text-foreground'
                : 'border-transparent text-muted-foreground hover:text-foreground',
            )}
          >
            {t.label}
          </button>
        ))}
      </div>

      <nav aria-label="Team tools" className="flex flex-wrap gap-2 text-[13px] font-bold">
        {can('team.invite') && (
          <ToolLink to="/w/$workspaceId/team/invite" workspaceId={workspaceId}>
            Full invite flow
          </ToolLink>
        )}
        <ToolLink to="/w/$workspaceId/team/roles" workspaceId={workspaceId}>
          Roles &amp; permissions editor
        </ToolLink>
        {can('team.roles') && (
          <ToolLink to="/w/$workspaceId/team/log" workspaceId={workspaceId}>
            Access log
          </ToolLink>
        )}
      </nav>

      {tab === 'members' && <MembersTab workspaceId={workspaceId} meId={meId} />}
      {tab === 'teams' && <TeamsTab workspaceId={workspaceId} />}
      {tab === 'roles' && <RolesTab workspaceId={workspaceId} />}
    </main>
  )
}

function ToolLink({
  to,
  workspaceId,
  children,
}: {
  to: '/w/$workspaceId/team/invite' | '/w/$workspaceId/team/roles' | '/w/$workspaceId/team/log'
  workspaceId: string
  children: React.ReactNode
}) {
  return (
    <Link
      to={to}
      params={{ workspaceId }}
      className="rounded-full bg-brand-soft px-3 py-1.5 text-brand-text no-underline hover:text-foreground"
    >
      {children}
    </Link>
  )
}

// ---------- Members ----------

const ALL = ''

function FilterMenu({
  label,
  value,
  options,
  onChange,
}: {
  label: string
  value: string
  options: { value: string; label: string }[]
  onChange: (v: string) => void
}) {
  const current = options.find((o) => o.value === value)?.label ?? label
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <button
          type="button"
          className="inline-flex h-10 items-center gap-1.5 rounded-[10px] border bg-card px-3 text-[13px] font-semibold text-foreground"
        >
          {current}
          <ChevronDownIcon className="size-4" aria-hidden="true" />
        </button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="start">
        <DropdownMenuRadioGroup value={value} onValueChange={onChange}>
          <DropdownMenuRadioItem value={ALL}>{label}</DropdownMenuRadioItem>
          {options.map((o) => (
            <DropdownMenuRadioItem key={o.value} value={o.value}>
              {o.label}
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

type Dialog =
  | { kind: 'role'; member: Member; roleId: string }
  | { kind: 'remove'; member: Member }
  | null

function MembersTab({ workspaceId, meId }: { workspaceId: string; meId: string }) {
  const navigate = useNavigate()
  const { can } = useAccess(workspaceId)
  const [q, setQ] = useState('')
  const [roleId, setRoleId] = useState(ALL)
  const [teamId, setTeamId] = useState(ALL)
  const [dialog, setDialog] = useState<Dialog>(null)
  const query = useDeferredValue(q.trim())
  const members = useInfiniteQuery(
    membersQueryOptions(workspaceId, {
      q: query || undefined,
      role_id: roleId || undefined,
      team_id: teamId || undefined,
    }),
  )
  const roles = useQuery(rolesQueryOptions(workspaceId))
  const teams = useQuery(teamsQueryOptions(workspaceId))
  const invites = useQuery({ ...invitationsQueryOptions(workspaceId), enabled: can('team.invite') })
  const rows = members.data?.pages.flatMap((p) => p.members) ?? []
  const pending = (invites.data ?? []).filter(
    (i) =>
      (i.status === 'pending' || i.status === 'expired') &&
      (!query || i.email.includes(query.toLowerCase())) &&
      (!roleId || i.role_id === roleId) &&
      (!teamId || i.team_ids.includes(teamId)),
  )
  const roleOf = (id: string) => roles.data?.find((r) => r.id === id)
  const teamNames = (ids: string[]) =>
    ids
      .map((id) => teams.data?.find((t) => t.id === id)?.name)
      .filter(Boolean)
      .join(', ')
  const goTo = (m: Member) =>
    navigate({
      to: '/w/$workspaceId/team/members/$userId',
      params: { workspaceId, userId: m.user_id },
    })

  return (
    <section
      aria-label="Members"
      className="flex flex-col gap-3.5 rounded-xl border bg-card px-5 py-[18px] shadow-card sm:px-[22px]"
    >
      <div className="flex flex-wrap items-center gap-2.5">
        <label className="flex h-10 max-w-[320px] flex-[1_1_220px] items-center gap-2 rounded-[10px] border bg-muted px-3 text-muted-foreground">
          <SearchIcon className="size-[18px] flex-none" aria-hidden="true" />
          <span className="sr-only">Search members</span>
          <input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search by name or email"
            className="min-w-0 flex-1 border-0 bg-transparent text-base text-foreground outline-none md:text-[13px]"
          />
        </label>
        <FilterMenu
          label="All roles"
          value={roleId}
          onChange={setRoleId}
          options={(roles.data ?? []).map((r) => ({ value: r.id, label: r.name }))}
        />
        <FilterMenu
          label="All teams"
          value={teamId}
          onChange={setTeamId}
          options={(teams.data ?? []).map((t) => ({ value: t.id, label: t.name }))}
        />
        <span className="ml-auto text-[13px] text-muted-foreground">
          {rows.length} {rows.length === 1 ? 'member' : 'members'}
          {pending.length > 0 &&
            ` · ${pending.length} pending ${pending.length === 1 ? 'invite' : 'invites'}`}
        </span>
      </div>

      {members.isError && (
        <p className="text-destructive">{errorMessage(members.error, 'Could not load members.')}</p>
      )}

      <div className="-mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
        <table className="w-full min-w-[900px] border-collapse text-sm">
          <thead>
            <tr className="text-left text-xs text-muted-foreground">
              <th className="py-2 pr-2 font-semibold">Member</th>
              <th className="p-2 font-semibold">Role</th>
              <th className="p-2 font-semibold">Team</th>
              <th className="p-2 font-semibold">Status</th>
              <th className="p-2 text-right font-semibold">Open chats</th>
              <th className="p-2 font-semibold">Last active</th>
              <th className="py-2 pl-2 font-semibold">
                <span className="sr-only">Actions</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {members.isPending &&
              [0, 1, 2].map((i) => (
                <tr key={i} className="border-t">
                  <td colSpan={7} className="py-3">
                    <Skeleton className="h-10 w-full" />
                  </td>
                </tr>
              ))}
            {rows.map((m) => (
              <MemberRow
                key={m.user_id}
                workspaceId={workspaceId}
                member={m}
                role={roleOf(m.role_id)}
                roles={roles.data ?? []}
                team={m.role_key === 'owner' ? 'All teams' : teamNames(m.team_ids) || '—'}
                self={m.user_id === meId}
                canRoles={can('team.roles')}
                canRemove={can('team.remove')}
                onOpen={() => goTo(m)}
                onRole={(id) => setDialog({ kind: 'role', member: m, roleId: id })}
                onRemove={() => setDialog({ kind: 'remove', member: m })}
              />
            ))}
            {pending.map((i) => (
              <InviteRow
                key={i.id}
                workspaceId={workspaceId}
                invite={i}
                team={teamNames(i.team_ids) || '—'}
              />
            ))}
            {!members.isPending && rows.length === 0 && pending.length === 0 && (
              <tr className="border-t">
                <td colSpan={7} className="py-10 text-center text-muted-foreground">
                  No one matches these filters.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
      {members.hasNextPage && (
        <Btn
          variant="outline"
          size="md"
          className="self-center"
          onClick={() => members.fetchNextPage()}
          disabled={members.isFetchingNextPage}
        >
          {members.isFetchingNextPage ? 'Loading…' : 'Show more'}
        </Btn>
      )}

      {dialog?.kind === 'role' && (
        <ChangeRoleDialog
          workspaceId={workspaceId}
          member={dialog.member}
          initialRoleId={dialog.roleId}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.kind === 'remove' && (
        <RemoveMemberDialog
          workspaceId={workspaceId}
          member={dialog.member}
          onClose={() => setDialog(null)}
          onRemoved={() => setDialog(null)}
        />
      )}
    </section>
  )
}

const STATUS = {
  online: { label: 'Online', dot: 'bg-brand', ring: 'border-brand-soft' },
  away: { label: 'Away', dot: 'bg-warning', ring: 'border-warning-soft' },
  offline: { label: 'Offline', dot: 'bg-neutral', ring: 'border-neutral-soft' },
} as const

function MemberRow({
  workspaceId,
  member: m,
  role,
  roles,
  team,
  self,
  canRoles,
  canRemove,
  onOpen,
  onRole,
  onRemove,
}: {
  workspaceId: string
  member: Member
  role?: Role
  roles: Role[]
  team: string
  self: boolean
  canRoles: boolean
  canRemove: boolean
  onOpen: () => void
  onRole: (roleId: string) => void
  onRemove: () => void
}) {
  const suspend = useSuspendMember(workspaceId, m.user_id)
  const restore = useRestoreMember(workspaceId, m.user_id)
  const owner = m.role_key === 'owner'
  const editable = canRoles && !owner && !self
  const status = STATUS[m.availability]
  return (
    <tr className="border-t hover:bg-muted">
      <td className="py-3 pr-2">
        <Link
          to="/w/$workspaceId/team/members/$userId"
          params={{ workspaceId, userId: m.user_id }}
          className="flex items-center gap-3 rounded-md text-foreground no-underline"
        >
          <PersonAvatar name={m.name} seed={m.user_id} />
          <span className="flex min-w-0 flex-col leading-[1.3]">
            <span className="font-bold">{m.name}</span>
            <span className="text-xs text-muted-foreground">{m.email}</span>
          </span>
        </Link>
      </td>
      <td className="p-2">
        {editable ? (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={`Change role for ${m.name}, currently ${m.role_name}`}
                className="inline-flex h-[34px] items-center gap-1.5 rounded-[10px] border bg-card pr-2.5 pl-3 text-[13px] font-semibold text-foreground"
              >
                {m.role_name}
                <ChevronDownIcon className="size-4" aria-hidden="true" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start">
              <DropdownMenuRadioGroup
                value={m.role_id}
                onValueChange={(id) => id !== m.role_id && onRole(id)}
              >
                {roles
                  .filter((r) => r.system_key !== 'owner')
                  .map((r) => (
                    <DropdownMenuRadioItem key={r.id} value={r.id}>
                      {r.name}
                    </DropdownMenuRadioItem>
                  ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        ) : (
          <span
            className={cn(
              'inline-flex h-[34px] items-center rounded-[10px] px-3 text-[13px] font-bold',
              TONE[role?.color ?? 'gray'].soft,
              TONE[role?.color ?? 'gray'].text,
            )}
          >
            {m.role_name}
          </span>
        )}
      </td>
      <td className="p-2 text-muted-foreground">{team}</td>
      <td className="p-2">
        {m.status === 'suspended' ? (
          <Pill tone="red">Suspended</Pill>
        ) : (
          <span className="inline-flex items-center gap-1.5 font-semibold whitespace-nowrap">
            <span
              className={cn('box-content size-2 rounded-full border-2', status.dot, status.ring)}
            />
            {status.label}
          </span>
        )}
      </td>
      <td className="p-2 text-right font-bold">—</td>
      <td className="p-2 whitespace-nowrap text-muted-foreground">
        {relativeTime(m.last_active_at)}
      </td>
      <td className="py-3 pl-2">
        <div className="flex items-center justify-end gap-1.5">
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <button
                type="button"
                aria-label={`More actions for ${m.name}`}
                className="flex size-10 items-center justify-center rounded-[10px] text-muted-foreground hover:bg-card hover:text-foreground"
              >
                <MoreVerticalIcon className="size-[18px]" aria-hidden="true" />
              </button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem onSelect={onOpen}>
                {self ? 'View my access' : 'Manage access'}
              </DropdownMenuItem>
              {canRemove && !owner && !self && (
                <>
                  <DropdownMenuSeparator />
                  {m.status === 'suspended' ? (
                    <DropdownMenuItem
                      onSelect={() =>
                        restore.mutate(undefined, {
                          onSuccess: () => toast.success(`${m.name}’s access is restored`),
                          onError: (e) => toast.error(errorMessage(e)),
                        })
                      }
                    >
                      Restore access
                    </DropdownMenuItem>
                  ) : (
                    <DropdownMenuItem
                      onSelect={() =>
                        suspend.mutate('', {
                          onSuccess: () => toast.success(`${m.name}’s access is suspended`),
                          onError: (e) => toast.error(errorMessage(e)),
                        })
                      }
                    >
                      Suspend access
                    </DropdownMenuItem>
                  )}
                  <DropdownMenuItem variant="destructive" onSelect={onRemove}>
                    Remove from workspace
                  </DropdownMenuItem>
                </>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </td>
    </tr>
  )
}

function InviteRow({
  workspaceId,
  invite,
  team,
}: {
  workspaceId: string
  invite: Invitation
  team: string
}) {
  const resend = useResendInvitation(workspaceId)
  const revoke = useRevokeInvitation(workspaceId)
  const name = invite.email.split('@')[0].replace(/[._-]+/g, ' ')
  const expired = invite.status === 'expired'
  return (
    <tr className="border-t hover:bg-muted">
      <td className="py-3 pr-2">
        <div className="flex items-center gap-3">
          <PersonAvatar name={name} tone="gray" />
          <span className="flex min-w-0 flex-col leading-[1.3]">
            <span className="font-bold capitalize">{name}</span>
            <span className="text-xs text-muted-foreground">{invite.email}</span>
          </span>
        </div>
      </td>
      <td className="p-2">
        <span className="inline-flex h-[34px] items-center rounded-[10px] border bg-card px-3 text-[13px] font-semibold">
          {invite.role_name}
        </span>
      </td>
      <td className="p-2 text-muted-foreground">{team}</td>
      <td className="p-2">
        <Pill tone={expired ? 'red' : 'amber'}>
          {expired ? 'Invite expired' : 'Invite pending'}
        </Pill>
        {invite.renewal_requested_at && (
          <span className="mt-1 block text-xs font-bold text-warning">Asked for a new link</span>
        )}
      </td>
      <td className="p-2 text-right font-bold">—</td>
      <td className="p-2 whitespace-nowrap text-muted-foreground">
        Invited{' '}
        {new Date(invite.last_sent_at).toLocaleDateString(undefined, {
          month: 'short',
          day: 'numeric',
        })}
      </td>
      <td className="py-3 pl-2">
        <div className="flex items-center justify-end gap-1.5">
          <Btn
            variant="outline"
            size="sm"
            disabled={resend.isPending}
            onClick={() =>
              resend.mutate(invite.id, {
                onSuccess: () => toast.success(`New link sent to ${invite.email}`),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            Resend
          </Btn>
          <Btn
            variant="danger-soft"
            size="sm"
            disabled={revoke.isPending}
            onClick={() =>
              revoke.mutate(invite.id, {
                onSuccess: () => toast.success(`Invite for ${invite.email} revoked`),
                onError: (e) => toast.error(errorMessage(e)),
              })
            }
          >
            Revoke
          </Btn>
          <span className="size-10" aria-hidden="true" />
        </div>
      </td>
    </tr>
  )
}

// ---------- Teams ----------

function TeamsTab({ workspaceId }: { workspaceId: string }) {
  const { can } = useAccess(workspaceId)
  const teams = useQuery(teamsQueryOptions(workspaceId))
  const members = useInfiniteQuery(membersQueryOptions(workspaceId))
  const invites = useQuery({ ...invitationsQueryOptions(workspaceId), enabled: can('team.invite') })
  const numbers = useQuery(numbersQueryOptions('', workspaceId))
  const [editing, setEditing] = useState<Team | 'new' | null>(null)
  const people = members.data?.pages.flatMap((p) => p.members) ?? []
  const editable = can('team.roles')

  if (teams.isPending) return <Skeleton className="h-60 w-full rounded-xl" />
  return (
    <section
      aria-label="Teams"
      className="grid grid-cols-[repeat(auto-fit,minmax(280px,1fr))] gap-4"
    >
      {teams.data?.map((t) => {
        const tm = people.filter((p) => t.member_ids.includes(p.user_id))
        const pendingCount = (invites.data ?? []).filter(
          (i) => i.status === 'pending' && i.team_ids.includes(t.id),
        ).length
        const number = numbers.data?.find((n) => n.id === t.number_id)
        return (
          <div
            key={t.id}
            className="flex flex-col gap-3.5 rounded-xl border bg-card p-5 shadow-card"
          >
            <div className="flex items-start gap-3">
              <span
                className={cn(
                  'flex size-11 flex-none items-center justify-center rounded-lg',
                  TONE[t.color].soft,
                  TONE[t.color].text,
                )}
              >
                <UsersIcon className="size-[18px]" aria-hidden="true" />
              </span>
              <div className="flex flex-1 flex-col leading-[1.35]">
                <span className="text-base font-extrabold">{t.name}</span>
                <span className="text-[13px] text-muted-foreground">{t.description}</span>
              </div>
              {editable && (
                <button
                  type="button"
                  aria-label={`Edit team ${t.name}`}
                  onClick={() => setEditing(t)}
                  className="flex size-10 items-center justify-center rounded-[10px] text-muted-foreground hover:bg-muted hover:text-foreground"
                >
                  <PencilIcon className="size-[18px]" aria-hidden="true" />
                </button>
              )}
            </div>
            <div className="flex items-center gap-2.5">
              <div className="flex">
                {tm.slice(0, 5).map((p) => (
                  <PersonAvatar
                    key={p.user_id}
                    name={p.name}
                    seed={p.user_id}
                    size={32}
                    className="-mr-2 border-2 border-card"
                  />
                ))}
              </div>
              <span className={cn('text-[13px] text-muted-foreground', tm.length > 0 && 'ml-3')}>
                {t.member_ids.length} {t.member_ids.length === 1 ? 'member' : 'members'}
                {pendingCount > 0 && ` · ${pendingCount} pending`}
              </span>
            </div>
            <div className="flex flex-col">
              <div className="flex justify-between gap-3 border-t py-2 text-[13px]">
                <span className="text-muted-foreground">WhatsApp number</span>
                <span className="font-semibold">
                  {number?.display_phone_number ?? 'Any number'}
                </span>
              </div>
              <div className="flex justify-between gap-3 border-t py-2 text-[13px]">
                <span className="text-muted-foreground">Open chats</span>
                <span className="font-bold">—</span>
              </div>
              <div className="flex justify-between gap-3 border-t py-2 text-[13px]">
                <span className="text-muted-foreground">Routing</span>
                <span className="font-semibold">
                  {ROUTING.find((r) => r.value === t.routing_method)?.label}
                </span>
              </div>
            </div>
          </div>
        )
      })}
      {editable && (
        <button
          type="button"
          onClick={() => setEditing('new')}
          className="flex min-h-[220px] flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed bg-transparent text-sm font-bold text-brand-text hover:bg-card"
        >
          <PlusIcon className="size-[18px]" aria-hidden="true" />
          Create team
        </button>
      )}
      {!editable && teams.data?.length === 0 && (
        <p className="text-muted-foreground">No teams yet.</p>
      )}
      {editing && (
        <TeamDialog
          workspaceId={workspaceId}
          team={editing === 'new' ? undefined : editing}
          people={people}
          numbers={numbers.data ?? []}
          onClose={() => setEditing(null)}
        />
      )}
    </section>
  )
}

// ---------- Roles & permissions ----------

function RolesTab({ workspaceId }: { workspaceId: string }) {
  const roles = useQuery(rolesQueryOptions(workspaceId))
  const catalog = useQuery(permissionsQueryOptions())
  if (!roles.data || !catalog.data) return <Skeleton className="h-96 w-full rounded-xl" />
  const order = new Map(GROUPS.map((g, i) => [g.key, i]))
  const perms = [...catalog.data].sort(
    (a, b) => (order.get(a.group) ?? 0) - (order.get(b.group) ?? 0),
  )
  return (
    <Panel aria-label="Roles and permissions" className="gap-3.5 py-[18px]">
      <PanelHeader
        title="What each role can do"
        description="Agents only see chats assigned to them or their team."
        action={
          <Link
            to="/w/$workspaceId/team/roles"
            params={{ workspaceId }}
            className="inline-flex h-10 items-center rounded-[10px] border bg-card px-3.5 text-[13px] font-bold text-foreground no-underline hover:bg-muted"
          >
            Manage roles &amp; permissions
          </Link>
        }
      />
      <div className="-mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
        <table className="w-full min-w-[640px] border-collapse text-sm">
          <thead>
            <tr className="text-xs text-muted-foreground">
              <th className="py-2.5 pr-2 text-left font-semibold">Permission</th>
              {roles.data.map((r) => (
                <th key={r.id} className="p-2.5 font-bold text-foreground">
                  {r.name}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {perms.map((p) => (
              <tr key={p.key} className="border-t hover:bg-muted">
                <td className="py-3 pr-2">
                  <div className="flex flex-col leading-[1.3]">
                    <span className="font-semibold">{p.label}</span>
                    <span className="text-xs text-muted-foreground">{PERMISSION_HINTS[p.key]}</span>
                  </div>
                </td>
                {roles.data.map((r) => {
                  const yes = r.system_key === 'owner' || r.permissions.includes(p.key)
                  return (
                    <td key={r.id} className="p-2 text-center">
                      {yes ? (
                        <span
                          role="img"
                          aria-label={`${r.name}: yes`}
                          className="inline-flex size-7 items-center justify-center rounded-full bg-brand-soft text-brand-text"
                        >
                          <CheckIcon className="size-4" aria-hidden="true" />
                        </span>
                      ) : (
                        <span
                          role="img"
                          aria-label={`${r.name}: no`}
                          className="inline-flex size-7 items-center justify-center text-muted-foreground"
                        >
                          <MinusIcon className="size-4" aria-hidden="true" />
                        </span>
                      )}
                    </td>
                  )
                })}
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Panel>
  )
}
