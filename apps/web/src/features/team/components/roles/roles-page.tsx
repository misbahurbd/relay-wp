import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import {
  AlertTriangleIcon,
  BotIcon,
  CreditCardIcon,
  FileTextIcon,
  InboxIcon,
  InfoIcon,
  LockIcon,
  MegaphoneIcon,
  PlugIcon,
  PlusIcon,
  RotateCcwIcon,
  ShieldIcon,
  Trash2Icon,
  UserIcon,
  UsersIcon,
} from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { NativeSelect } from '#/components/ui/native-select'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import {
  fetchRolePreview,
  useCreateRole,
  useDeleteRole,
  useResetRole,
  useUpdateRole,
} from '../../api/team.mutations'
import {
  accessLogQueryOptions,
  membersQueryOptions,
  permissionsQueryOptions,
  rolesQueryOptions,
} from '../../api/team.queries'
import { useAccess } from '../../hooks/use-access'
import {
  COLORS,
  GROUPS,
  PERMISSION_HINTS,
  roleSchema,
  SCOPES,
  TONE,
} from '../../schemas/team.schema'
import type { ChatScope, Permission, Role, RoleChange, RoleColor } from '../../types'
import { PersonAvatar } from '../kit/avatar'
import { Btn } from '../kit/btn'
import { ErrorText, inputClass, selectClass } from '../kit/field'
import { formatDateTime } from '../kit/format'
import { describeLog } from '../kit/log-format'
import { Modal } from '../kit/modal'
import { Breadcrumb, SectionLabel } from '../kit/panel'
import { RadioOption } from '../kit/radio'
import { Toggle } from '../kit/toggle'

const GROUP_ICONS: Record<string, typeof InboxIcon> = {
  inbox: InboxIcon,
  contacts: UserIcon,
  broadcasts: MegaphoneIcon,
  templates: FileTextIcon,
  automations: BotIcon,
  integrations: PlugIcon,
  team: UsersIcon,
  billing: CreditCardIcon,
}

const SCOPE_HELP: Record<ChatScope, string> = {
  assigned: 'They only see a chat once it’s assigned to them.',
  team: 'They see chats assigned to anyone in their teams, plus their teams’ unassigned queue.',
  all: 'They see every chat on the numbers they can use.',
}

type Draft = {
  name: string
  description: string
  color: RoleColor
  chat_scope: ChatScope
  permissions: string[]
  copyFrom: string
}

const NEW = 'new'

function draftOf(r: Role): Draft {
  return {
    name: r.name,
    description: r.description,
    color: r.color,
    chat_scope: r.chat_scope,
    permissions: [...r.permissions],
    copyFrom: '',
  }
}

const sameSet = (a: string[], b: string[]) => a.length === b.length && a.every((x) => b.includes(x))

export function RolesPage({ workspaceId }: { workspaceId: string }) {
  const { can } = useAccess(workspaceId)
  const roles = useQuery(rolesQueryOptions(workspaceId))
  const catalog = useQuery(permissionsQueryOptions())
  const [selected, setSelected] = useState<string>()
  const [draft, setDraft] = useState<Draft>()
  const editable = can('team.roles')

  if (!roles.data || !catalog.data) {
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-[18px] p-4 sm:px-7 sm:pt-6">
        <Skeleton className="h-24 w-full rounded-xl" />
        <Skeleton className="h-96 w-full rounded-xl" />
      </main>
    )
  }

  const list = roles.data
  const currentId = selected ?? list.find((r) => r.system_key === 'agent')?.id ?? list[0]?.id
  const creating = currentId === NEW
  const role = creating ? undefined : list.find((r) => r.id === currentId)
  const base: Draft | undefined = creating
    ? {
        name: '',
        description: '',
        color: 'blue',
        chat_scope: 'assigned',
        permissions: [],
        copyFrom: '',
      }
    : role && draftOf(role)
  const d = draft ?? base
  const dirty =
    !!d &&
    !!base &&
    (creating ||
      d.name !== base.name ||
      d.description !== base.description ||
      d.color !== base.color ||
      d.chat_scope !== base.chat_scope ||
      !sameSet(d.permissions, base.permissions))

  function pick(id: string) {
    if (id === currentId) return
    if (dirty && !window.confirm('Discard your unsaved changes to this role?')) return
    setSelected(id)
    setDraft(undefined)
  }

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-[18px] p-4 pb-0 sm:px-7 sm:pt-6">
      <Breadcrumb current="Roles & permissions" />
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Roles &amp; permissions</h1>
          <p className="max-w-[640px] text-[15px] text-muted-foreground">
            A role is a set of permissions. Everyone with the role gets the same access, so changing
            a role updates all its members at once.
          </p>
        </div>
        {editable && (
          <Btn onClick={() => pick(NEW)}>
            <PlusIcon className="size-[18px]" aria-hidden="true" />
            Create custom role
          </Btn>
        )}
      </div>

      <ol
        aria-label="How changing access works"
        className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-2.5"
      >
        {[
          ['Pick a role', 'or create a custom one'],
          ['Turn access on or off', 'on gives, off revokes'],
          ['Review who’s affected', 'see gains and losses'],
          ['Save', 'applies right away'],
        ].map(([title, sub], i) => (
          <li
            key={title}
            className="flex items-center gap-2.5 rounded-lg border bg-card px-3.5 py-3"
          >
            <span className="flex size-[26px] flex-none items-center justify-center rounded-full bg-brand-soft text-xs font-extrabold text-brand-text">
              {i + 1}
            </span>
            <span className="leading-[1.3]">
              <strong>{title}</strong>
              <br />
              <span className="text-xs text-muted-foreground">{sub}</span>
            </span>
          </li>
        ))}
      </ol>

      <div className="flex flex-wrap items-start gap-5">
        <div className="flex min-w-0 flex-[1_1_260px] flex-col gap-4">
          <RoleList
            roles={list}
            currentId={currentId}
            creating={creating}
            draftName={creating ? d?.name : undefined}
            draftColor={creating ? d?.color : undefined}
            dirtyId={dirty && !creating ? currentId : undefined}
            editable={editable}
            onPick={pick}
          />
          {editable && <RecentChanges workspaceId={workspaceId} catalog={catalog.data} />}
        </div>
        <div className="flex min-w-0 flex-[3_1_560px] flex-col gap-4">
          {d && (
            <Editor
              key={currentId}
              workspaceId={workspaceId}
              role={role}
              roles={list}
              catalog={catalog.data}
              draft={d}
              base={base as Draft}
              dirty={dirty}
              editable={editable}
              onDraft={setDraft}
              onSaved={(id) => {
                setSelected(id)
                setDraft(undefined)
              }}
              onDeleted={() => {
                setSelected(undefined)
                setDraft(undefined)
              }}
            />
          )}
        </div>
      </div>
    </main>
  )
}

function RoleList({
  roles,
  currentId,
  creating,
  draftName,
  draftColor,
  dirtyId,
  editable,
  onPick,
}: {
  roles: Role[]
  currentId?: string
  creating: boolean
  draftName?: string
  draftColor?: RoleColor
  dirtyId?: string
  editable: boolean
  onPick: (id: string) => void
}) {
  const item = (r: Role) => {
    const on = r.id === currentId
    return (
      <Button
        variant="unstyled"
        key={r.id}
        type="button"
        aria-current={on ? 'true' : undefined}
        onClick={() => onPick(r.id)}
        className={cn(
          'flex min-h-12 w-full items-center gap-2.5 rounded-lg border px-2.5 py-2 text-left text-foreground',
          on ? 'border-primary bg-brand-soft' : 'border-transparent bg-transparent hover:bg-muted',
        )}
      >
        <span className={cn('size-2.5 flex-none rounded-[3px]', TONE[r.color].solid)} />
        <span className="flex min-w-0 flex-1 flex-col leading-[1.3]">
          <span className="font-bold">{r.name}</span>
          <span className="text-xs text-muted-foreground">
            {r.member_count === 0
              ? 'No members'
              : `${r.member_count} ${r.member_count === 1 ? 'member' : 'members'}`}
          </span>
        </span>
        {r.locked && <LockIcon className="size-4 text-muted-foreground" aria-label="Locked" />}
        {dirtyId === r.id && (
          <span className="rounded-md bg-warning-soft px-[7px] py-px text-[11px] font-extrabold text-warning">
            Unsaved
          </span>
        )}
      </Button>
    )
  }
  const system = roles.filter((r) => r.system_key)
  const custom = roles.filter((r) => !r.system_key)
  return (
    <section
      aria-label="Roles"
      className="flex flex-col gap-1.5 rounded-xl border bg-card p-4 shadow-card"
    >
      <SectionLabel className="mb-1 ml-1.5">System roles</SectionLabel>
      {system.map(item)}
      <SectionLabel className="mt-3 mb-1 ml-1.5">Custom roles</SectionLabel>
      {custom.map(item)}
      {custom.length === 0 && !creating && (
        <span className="ml-1.5 text-[13px] text-muted-foreground">None yet.</span>
      )}
      {creating && (
        <div className="flex min-h-12 items-center gap-2.5 rounded-lg border border-brand bg-brand-soft px-2.5 py-2">
          <span className={cn('size-2.5 rounded-[3px]', TONE[draftColor ?? 'blue'].solid)} />
          <span className="flex flex-1 flex-col leading-[1.3]">
            <span className="font-bold">{draftName || 'New role'}</span>
            <span className="text-xs text-muted-foreground">New · not saved yet</span>
          </span>
        </div>
      )}
      {editable && (
        <Button
          variant="unstyled"
          type="button"
          onClick={() => onPick(NEW)}
          className="mt-1.5 flex min-h-11 items-center justify-center gap-1.5 rounded-lg border border-dashed bg-transparent font-bold text-brand-text hover:bg-muted"
        >
          <PlusIcon className="size-4" aria-hidden="true" />
          Create custom role
        </Button>
      )}
    </section>
  )
}

const ROLE_ACTIONS = 'role_created,role_updated,role_deleted'

function RecentChanges({ workspaceId, catalog }: { workspaceId: string; catalog: Permission[] }) {
  const log = useInfiniteQuery(accessLogQueryOptions(workspaceId, {}, 50))
  const label = (k: string) => catalog.find((p) => p.key === k)?.label ?? k
  const entries = (log.data?.pages[0]?.entries ?? [])
    .filter((e) => ROLE_ACTIONS.includes(e.action))
    .slice(0, 4)
  return (
    <section
      aria-label="Recent permission changes"
      className="flex flex-col gap-2.5 rounded-xl border bg-card p-4 shadow-card"
    >
      <h2 className="text-[15px] font-bold">Recent changes</h2>
      {entries.map((e) => (
        <div key={e.id} className="flex gap-2.5 text-[13px]">
          <PersonAvatar
            name={e.actor_name || 'System'}
            seed={e.actor_id ?? 'system'}
            size={26}
            className="text-[10px]"
          />
          <span className="flex flex-col leading-[1.35]">
            <span>
              {e.actor_name || 'Relay'} {describeLog(e, label)}
              {e.note && ` · ${e.note}`}
            </span>
            <span className="text-xs text-muted-foreground">{formatDateTime(e.created_at)}</span>
          </span>
        </div>
      ))}
      {!log.isPending && entries.length === 0 && (
        <span className="text-[13px] text-muted-foreground">No role changes yet.</span>
      )}
      <Link to="/team/log" className="text-[13px] font-bold">
        Full access log
      </Link>
    </section>
  )
}

function Editor({
  workspaceId,
  role,
  roles,
  catalog,
  draft: d,
  base,
  dirty,
  editable,
  onDraft,
  onSaved,
  onDeleted,
}: {
  workspaceId: string
  role?: Role
  roles: Role[]
  catalog: Permission[]
  draft: Draft
  base: Draft
  dirty: boolean
  editable: boolean
  onDraft: (d: Draft | undefined) => void
  onSaved: (id: string) => void
  onDeleted: () => void
}) {
  const creating = !role
  const locked = !editable || !!role?.locked
  const members = useInfiniteQuery({
    ...membersQueryOptions(workspaceId, { role_id: role?.id }),
    enabled: !!role,
  })
  const reset = useResetRole(workspaceId)
  const [nameError, setNameError] = useState<string>()
  const [confirm, setConfirm] = useState<RoleChange | null>(null)
  const [deleting, setDeleting] = useState(false)
  const [previewError, setPreviewError] = useState<string>()
  const people = members.data?.pages.flatMap((p) => p.members) ?? []
  const set = (patch: Partial<Draft>) => onDraft({ ...d, ...patch })
  const owner = role?.system_key === 'owner'
  const on = (k: string) => owner || d.permissions.includes(k)
  const added = d.permissions.filter((k) => !base.permissions.includes(k))
  const removed = base.permissions.filter((k) => !d.permissions.includes(k))
  const scope = SCOPES.find((s) => s.value === d.chat_scope)
  const tone = TONE[d.color]

  async function review() {
    const parsed = roleSchema.safeParse({ name: d.name, description: d.description })
    if (!parsed.success) {
      setNameError(parsed.error.flatten().fieldErrors.name?.[0])
      document.getElementById('role-name')?.focus()
      return
    }
    setNameError(undefined)
    setPreviewError(undefined)
    if (creating) {
      const sensitive = d.permissions.some((k) => catalog.find((p) => p.key === k)?.sensitive)
      setConfirm({ gains: d.permissions, losses: [], sensitive_gain: sensitive, affected: [] })
      return
    }
    try {
      setConfirm(
        await fetchRolePreview(workspaceId, role.id, {
          permissions: d.permissions,
          chat_scope: d.chat_scope,
        }),
      )
    } catch (e) {
      setPreviewError(errorMessage(e))
    }
  }

  function startFrom(id: string) {
    const src = roles.find((r) => r.id === id)
    set({
      copyFrom: id,
      permissions: src ? [...src.permissions] : [],
      chat_scope: src?.chat_scope ?? 'assigned',
    })
  }

  return (
    <>
      <section
        aria-label="Role details"
        className="flex flex-col gap-4 rounded-xl border bg-card px-5 py-5 shadow-card sm:px-[22px]"
      >
        <div className="flex flex-wrap items-center gap-3">
          <span
            className={cn(
              'flex size-11 flex-none items-center justify-center rounded-lg',
              tone.soft,
              tone.text,
            )}
          >
            <ShieldIcon className="size-5" aria-hidden="true" />
          </span>
          <div className="flex flex-[1_1_200px] flex-col gap-0.5">
            <div className="flex flex-wrap items-center gap-2">
              <h2 className="text-xl font-extrabold">{d.name || 'New role'}</h2>
              <span className="rounded-full bg-neutral-soft px-2 py-0.5 text-[11px] font-extrabold text-neutral">
                {creating ? 'New' : role.system_key ? 'System role' : 'Custom role'}
              </span>
            </div>
            <span className="text-[13px] text-muted-foreground">
              {owner ? catalog.length : d.permissions.length} of {catalog.length} permissions on ·{' '}
              {scope?.label.toLowerCase()}
            </span>
          </div>
          {role && (
            <div className="flex flex-wrap items-center gap-2.5">
              <div className="flex items-center pl-1.5">
                {people.slice(0, 4).map((m) => (
                  <PersonAvatar
                    key={m.user_id}
                    name={m.name}
                    seed={m.user_id}
                    size={32}
                    className="-ml-1.5 border-2 border-card"
                  />
                ))}
                <span className="ml-2 text-[13px] text-muted-foreground">
                  {role.member_count} {role.member_count === 1 ? 'member' : 'members'}
                </span>
              </div>
              <Link to="/team" search={{ tab: 'members' }} className="text-[13px] font-bold">
                Manage members
              </Link>
            </div>
          )}
        </div>
        {owner && (
          <div
            role="note"
            className="flex items-center gap-2.5 rounded-lg bg-neutral-soft px-3.5 py-3 text-[13px]"
          >
            <LockIcon className="size-4 flex-none" aria-hidden="true" />
            <span>
              <strong>Owner always has full access.</strong> This role can’t be edited or removed.
              To hand over ownership, go to Settings › Security.
            </span>
          </div>
        )}
        <div className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3.5">
          <label className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Role name</span>
            <Input
              unstyled
              id="role-name"
              type="text"
              className={inputClass}
              value={d.name}
              disabled={locked || !!role?.system_key}
              maxLength={50}
              placeholder="e.g. Returns specialist"
              aria-invalid={!!nameError}
              aria-describedby={nameError ? 'role-name-error' : undefined}
              onChange={(e) => set({ name: e.target.value })}
            />
            <ErrorText id="role-name-error">{nameError}</ErrorText>
          </label>
          <label className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Description</span>
            <Input
              unstyled
              type="text"
              className={inputClass}
              value={d.description}
              disabled={locked}
              maxLength={200}
              placeholder="What is this role for?"
              onChange={(e) => set({ description: e.target.value })}
            />
          </label>
        </div>
        <div className="flex flex-wrap items-end gap-5">
          <div role="radiogroup" aria-label="Role color" className="flex flex-col gap-1.5">
            <span className="text-[13px] font-bold">Color</span>
            <div className="flex gap-2">
              {COLORS.map((c) => (
                <RadioOption
                  key={c.value}
                  name="role-color"
                  checked={d.color === c.value}
                  label={c.label}
                  disabled={locked}
                  onSelect={() => set({ color: c.value })}
                  className={cn(
                    'flex size-11 items-center justify-center rounded-lg border-2 bg-card has-[:disabled]:opacity-60',
                    d.color === c.value ? 'border-foreground' : 'border-border',
                  )}
                >
                  <span className={cn('size-6 rounded-[7px]', TONE[c.value].solid)} />
                </RadioOption>
              ))}
            </div>
          </div>
          {creating && (
            <label className="flex min-w-[220px] flex-col gap-1.5">
              <span className="text-[13px] font-bold">Duplicate from</span>
              <NativeSelect
                unstyled
                className={selectClass}
                value={d.copyFrom}
                onChange={(e) => startFrom(e.target.value)}
              >
                <option value="">Start from scratch (no access)</option>
                {roles
                  .filter((r) => r.system_key !== 'owner')
                  .map((r) => (
                    <option key={r.id} value={r.id}>
                      {r.name}
                    </option>
                  ))}
              </NativeSelect>
            </label>
          )}
          {role && editable && !owner && (
            <div className="ml-auto flex flex-wrap gap-2.5">
              {role.system_key && (
                <Btn
                  variant="outline"
                  className="px-3.5"
                  disabled={reset.isPending}
                  onClick={() =>
                    reset.mutate(role.id, {
                      onSuccess: () => {
                        toast.success(`${role.name} reset to default`)
                        onDraft(undefined)
                      },
                      onError: (e) => toast.error(errorMessage(e)),
                    })
                  }
                >
                  <RotateCcwIcon className="size-4" aria-hidden="true" />
                  Reset to default
                </Btn>
              )}
              {!role.system_key && (
                <Btn variant="danger-outline" className="px-3.5" onClick={() => setDeleting(true)}>
                  <Trash2Icon className="size-4" aria-hidden="true" />
                  Delete role
                </Btn>
              )}
            </div>
          )}
        </div>
      </section>

      <section
        aria-label="Chat access"
        className="flex flex-col gap-3.5 rounded-xl border bg-card px-5 py-5 shadow-card sm:px-[22px]"
      >
        <div className="flex flex-col gap-0.5">
          <h2 className="text-base font-bold">Chat access</h2>
          <span className="text-[13px] text-muted-foreground">
            Which conversations people with this role can see in the inbox.
          </span>
        </div>
        <div
          role="radiogroup"
          aria-label="Chat access"
          className="flex flex-wrap gap-1 self-start rounded-lg bg-muted p-1"
        >
          {SCOPES.map((s) => (
            <RadioOption
              key={s.value}
              name="role-scope"
              checked={(owner ? 'all' : d.chat_scope) === s.value}
              disabled={locked}
              onSelect={() => set({ chat_scope: s.value })}
              className={cn(
                'min-h-10 rounded-[9px] px-3.5 text-[13px] font-bold text-foreground',
                (owner ? 'all' : d.chat_scope) === s.value
                  ? 'bg-card shadow-[0_1px_3px_rgba(0,0,0,.12)]'
                  : 'bg-transparent hover:bg-card/60',
              )}
            >
              {s.label}
            </RadioOption>
          ))}
        </div>
        <span className="text-[13px] text-muted-foreground">
          {SCOPE_HELP[owner ? 'all' : d.chat_scope]}
        </span>
        <span className="text-xs text-muted-foreground">
          Which WhatsApp numbers someone uses is set per person, on their member page.
        </span>
      </section>

      <div className="grid grid-cols-[repeat(auto-fit,minmax(320px,1fr))] gap-4">
        {GROUPS.map((g) => {
          const items = catalog.filter((p) => p.group === g.key)
          if (items.length === 0) return null
          const Icon = GROUP_ICONS[g.key] ?? ShieldIcon
          return (
            <section
              key={g.key}
              aria-label={`${g.title} permissions`}
              className="flex flex-col gap-1 rounded-xl border bg-card px-[18px] py-4 shadow-card"
            >
              <div className="mb-1 flex items-center gap-2.5">
                <span
                  className={cn(
                    'flex size-[34px] flex-none items-center justify-center rounded-[10px]',
                    TONE[g.tone].soft,
                    TONE[g.tone].text,
                  )}
                >
                  <Icon className="size-[18px]" aria-hidden="true" />
                </span>
                <h3 className="flex-1 text-[15px] font-bold">{g.title}</h3>
                <span className="text-xs font-bold text-muted-foreground">
                  {items.filter((p) => on(p.key)).length} of {items.length} on
                </span>
              </div>
              {items.map((p) => {
                const isOn = on(p.key)
                const change = added.includes(p.key)
                  ? 'Added'
                  : removed.includes(p.key)
                    ? 'Removed'
                    : null
                return (
                  <div key={p.key} className="flex min-h-[52px] items-center gap-3 border-t py-1.5">
                    <div className="flex min-w-0 flex-1 flex-col leading-[1.35]">
                      <span className="flex flex-wrap items-center gap-1.5">
                        <span className="text-[13px] font-bold">{p.label}</span>
                        {p.sensitive && (
                          <span className="inline-flex items-center gap-[3px] rounded-md bg-destructive-soft px-1.5 text-[11px] font-extrabold text-destructive">
                            <AlertTriangleIcon className="size-3" aria-hidden="true" />
                            Sensitive
                          </span>
                        )}
                        {change && (
                          <span
                            className={cn(
                              'rounded-md px-1.5 text-[11px] font-extrabold',
                              change === 'Added'
                                ? 'bg-brand-soft text-brand-text'
                                : 'bg-destructive-soft text-destructive',
                            )}
                          >
                            {change}
                          </span>
                        )}
                      </span>
                      <span className="text-xs text-muted-foreground">
                        {PERMISSION_HINTS[p.key]}
                      </span>
                    </div>
                    <Toggle
                      on={isOn}
                      label={p.label}
                      disabled={locked}
                      title={owner ? 'Owner always has full access' : undefined}
                      onChange={(v) =>
                        set({
                          permissions: v
                            ? [...d.permissions, p.key]
                            : d.permissions.filter((k) => k !== p.key),
                        })
                      }
                    />
                  </div>
                )
              })}
            </section>
          )
        })}
      </div>

      {!locked && (
        <div className="sticky bottom-0 z-[5] -mx-2 bg-background px-2 pt-3 pb-[18px]">
          <div
            role="status"
            className={cn(
              'flex flex-wrap items-center gap-3 rounded-[14px] border bg-card px-4 py-3 shadow-[0_8px_30px_rgba(0,0,0,.12)]',
              dirty && 'border-brand',
            )}
          >
            {dirty ? (
              <>
                <span className="inline-flex flex-wrap items-center gap-2 font-bold">
                  {added.length > 0 && (
                    <span className="rounded-full bg-brand-soft px-2 py-0.5 text-brand-text">
                      +{added.length} {added.length === 1 ? 'permission' : 'permissions'} added
                    </span>
                  )}
                  {removed.length > 0 && (
                    <span className="rounded-full bg-destructive-soft px-2 py-0.5 text-destructive">
                      −{removed.length} removed
                    </span>
                  )}
                  <span className="font-semibold text-muted-foreground">
                    {creating
                      ? '· new role'
                      : `· affects ${role.member_count} ${role.member_count === 1 ? 'member' : 'members'}`}
                  </span>
                </span>
                {previewError && (
                  <span className="text-[13px] text-destructive">{previewError}</span>
                )}
                <div className="ml-auto flex gap-2.5">
                  <Btn variant="outline" className="px-4" onClick={() => onDraft(undefined)}>
                    Discard
                  </Btn>
                  <Btn onClick={review}>{creating ? 'Create role' : 'Save changes'}</Btn>
                </div>
              </>
            ) : (
              <span className="inline-flex items-center gap-2 text-muted-foreground">
                <InfoIcon className="size-[18px]" aria-hidden="true" />
                No unsaved changes. Turn a switch on to give access, off to revoke it.
              </span>
            )}
          </div>
        </div>
      )}

      {confirm && (
        <ConfirmSave
          workspaceId={workspaceId}
          role={role}
          draft={d}
          change={confirm}
          catalog={catalog}
          onClose={() => setConfirm(null)}
          onSaved={(id) => {
            setConfirm(null)
            onSaved(id)
          }}
        />
      )}
      {deleting && role && (
        <DeleteRole
          workspaceId={workspaceId}
          role={role}
          roles={roles}
          onClose={() => setDeleting(false)}
          onDeleted={onDeleted}
        />
      )}
    </>
  )
}

function ConfirmSave({
  workspaceId,
  role,
  draft: d,
  change,
  catalog,
  onClose,
  onSaved,
}: {
  workspaceId: string
  role?: Role
  draft: Draft
  change: RoleChange
  catalog: Permission[]
  onClose: () => void
  onSaved: (id: string) => void
}) {
  const create = useCreateRole(workspaceId)
  const update = useUpdateRole(workspaceId)
  const [notify, setNotify] = useState(true)
  const label = (k: string) => catalog.find((p) => p.key === k)?.label ?? k
  const pending = create.isPending || update.isPending
  const error = create.error ?? update.error

  function save() {
    const body = {
      name: d.name.trim(),
      description: d.description.trim(),
      color: d.color,
      chat_scope: d.chat_scope,
      permissions: d.permissions,
      notify_members: notify,
    }
    if (role) {
      update.mutate(
        { roleId: role.id, body: { ...body, expected_version: role.version } },
        {
          onSuccess: () => {
            toast.success(`${body.name} saved`)
            onSaved(role.id)
          },
        },
      )
    } else {
      create.mutate(
        { ...body, copy_from_role_id: d.copyFrom || undefined },
        {
          onSuccess: (r) => {
            toast.success(`${body.name} created`)
            onSaved(r.id)
          },
        },
      )
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      title={role ? `Save changes to ${role.name}?` : `Create “${d.name.trim()}”?`}
      description="Changes apply right away. Anyone using a feature they lose is moved out of it."
      width="sm:max-w-[600px]"
    >
      <div className="flex flex-col gap-2">
        <SectionLabel>Who is affected</SectionLabel>
        {change.affected.length > 0 ? (
          <div className="flex flex-wrap gap-1.5">
            {change.affected.map((m) => (
              <span
                key={m.user_id}
                className="inline-flex items-center gap-1.5 rounded-full border bg-muted py-1 pr-2.5 pl-1 text-[13px] font-semibold"
              >
                <PersonAvatar name={m.name} seed={m.user_id} size={22} className="text-[9px]" />
                {m.name}
              </span>
            ))}
          </div>
        ) : (
          <span className="text-[13px] text-muted-foreground">
            No one has this role yet. Assign it from Team › Members.
          </span>
        )}
      </div>
      <div className="grid gap-2.5 sm:grid-cols-2">
        <div className="flex flex-col gap-1.5 rounded-lg bg-brand-soft px-3.5 py-3">
          <span className="text-xs font-extrabold text-brand-text">They gain</span>
          {change.gains.map((g) => (
            <span key={g} className="text-[13px]">
              <span aria-hidden="true">+ </span>
              {label(g)}
            </span>
          ))}
          {change.gains.length === 0 && (
            <span className="text-[13px] text-muted-foreground">Nothing new</span>
          )}
        </div>
        <div className="flex flex-col gap-1.5 rounded-lg bg-destructive-soft px-3.5 py-3">
          <span className="text-xs font-extrabold text-destructive">They lose</span>
          {change.losses.map((l) => (
            <span key={l} className="text-[13px]">
              <span aria-hidden="true">− </span>
              {label(l)}
            </span>
          ))}
          {change.losses.length === 0 && (
            <span className="text-[13px] text-muted-foreground">Nothing removed</span>
          )}
        </div>
      </div>
      {change.sensitive_gain && (
        <div className="flex items-start gap-2.5 rounded-lg bg-warning-soft px-3.5 py-3 text-[13px] text-warning">
          <AlertTriangleIcon className="mt-0.5 size-4 flex-none" aria-hidden="true" />
          You’re giving a sensitive permission. Only give it to people you trust with customer data.
        </div>
      )}
      {role && (
        <label className="flex items-center gap-2.5 text-sm">
          <Input
            unstyled
            type="checkbox"
            className="size-[18px] accent-primary"
            checked={notify}
            onChange={(e) => setNotify(e.target.checked)}
          />
          Notify affected members by email
        </label>
      )}
      <ErrorText>{error && errorMessage(error)}</ErrorText>
      <div className="flex justify-end gap-2.5">
        <Btn variant="outline" onClick={onClose}>
          Back to editing
        </Btn>
        <Btn disabled={pending} onClick={save}>
          {pending ? 'Saving…' : role ? 'Save changes' : 'Create role'}
        </Btn>
      </div>
    </Modal>
  )
}

function DeleteRole({
  workspaceId,
  role,
  roles,
  onClose,
  onDeleted,
}: {
  workspaceId: string
  role: Role
  roles: Role[]
  onClose: () => void
  onDeleted: () => void
}) {
  const remove = useDeleteRole(workspaceId)
  const [moveTo, setMoveTo] = useState('')
  const [error, setError] = useState<string>()
  const targets = roles.filter((r) => r.id !== role.id && r.system_key !== 'owner')
  return (
    <Modal
      open
      onClose={onClose}
      title={`Delete “${role.name}”?`}
      description={
        role.member_count > 0
          ? `${role.member_count} ${role.member_count === 1 ? 'person has' : 'people have'} this role. Choose where they go.`
          : 'No one has this role. Pending invites that use it still need a new role.'
      }
      width="sm:max-w-[480px]"
      icon={
        <span className="flex size-11 flex-none items-center justify-center rounded-lg bg-destructive-soft text-destructive">
          <Trash2Icon className="size-5" aria-hidden="true" />
        </span>
      }
    >
      <label className="flex flex-col gap-1.5">
        <span className="text-[13px] font-bold">Move its members and pending invites to</span>
        <NativeSelect
          unstyled
          className={selectClass}
          value={moveTo}
          aria-invalid={!!error}
          aria-describedby="move-help"
          onChange={(e) => {
            setMoveTo(e.target.value)
            setError(undefined)
          }}
        >
          <option value="">Choose a role…</option>
          {targets.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name}
            </option>
          ))}
        </NativeSelect>
        <span id="move-help" className="text-xs text-muted-foreground">
          Required. Nobody is left without a role.
        </span>
      </label>
      <ErrorText>{error ?? (remove.isError ? errorMessage(remove.error) : undefined)}</ErrorText>
      <div className="flex justify-end gap-2.5">
        <Btn variant="outline" onClick={onClose}>
          Cancel
        </Btn>
        <Btn
          variant="danger"
          disabled={!moveTo || remove.isPending}
          onClick={() =>
            remove.mutate(
              { roleId: role.id, moveTo },
              {
                onSuccess: () => {
                  toast.success(`${role.name} deleted`)
                  onDeleted()
                },
              },
            )
          }
        >
          {remove.isPending ? 'Deleting…' : 'Delete role'}
        </Btn>
      </div>
    </Modal>
  )
}
