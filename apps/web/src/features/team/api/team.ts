import {
  acceptInvitation,
  addMemberOverride,
  changeMemberRole,
  createInvitations,
  createRole,
  createTeam,
  deleteRole,
  deleteTeam,
  getAccessLogSummary,
  getMember,
  getMyAccess,
  getSeats,
  inviteSignup,
  leaveWorkspace,
  listAccessLog,
  listInvitations,
  listMembers,
  listPermissions,
  listRoles,
  listTeams,
  type OverrideBody,
  previewInvitation,
  previewMemberRole,
  previewRole,
  removeMember,
  requestInvitationRenewal,
  resendInvitation,
  resetRole,
  restoreMember,
  revokeInvitation,
  revokeMemberOverride,
  setTeamMembers,
  suspendMember,
  updateMemberAccess,
  updateRole,
  updateTeam,
} from '@relay/api-client'
import {
  infiniteQueryOptions,
  type QueryClient,
  queryOptions,
  useMutation,
  useQueryClient,
} from '@tanstack/react-query'
import type {
  Access,
  ChatScope,
  Invitation,
  InviteInput,
  MemberDetail,
  MemberFilter,
  Role,
  RoleChange,
  RoleInput,
  Team,
  TeamInput,
} from '../types'

/** Every team query for one workspace sits under ['team', workspaceId], so one invalidate refreshes all. */
export const teamKeys = {
  workspace: (workspaceId: string) => ['team', workspaceId] as const,
  access: (workspaceId: string) => ['team', workspaceId, 'access'] as const,
  seats: (workspaceId: string) => ['team', workspaceId, 'seats'] as const,
  roles: (workspaceId: string) => ['team', workspaceId, 'roles'] as const,
  members: (workspaceId: string, filter?: MemberFilter) =>
    ['team', workspaceId, 'members', filter ?? {}] as const,
  member: (workspaceId: string, userId: string) => ['team', workspaceId, 'member', userId] as const,
  teams: (workspaceId: string) => ['team', workspaceId, 'teams'] as const,
  invitations: (workspaceId: string) => ['team', workspaceId, 'invitations'] as const,
  log: (workspaceId: string, filter?: LogFilter) =>
    ['team', workspaceId, 'log', filter ?? {}] as const,
  logSummary: (workspaceId: string) => ['team', workspaceId, 'log-summary'] as const,
  permissions: () => ['permissions'] as const,
  invite: (token: string) => ['invite', token] as const,
}

const ids = (v: string[] | null | undefined) => v ?? []

export const accessQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.access(workspaceId),
    queryFn: async ({ signal }): Promise<Access> => {
      const { data } = await getMyAccess({ path: { workspaceId }, signal, throwOnError: true })
      return {
        ...data,
        permissions: ids(data.permissions),
        team_ids: ids(data.team_ids),
        number_ids: ids(data.number_ids),
      }
    },
  })

export const permissionsQueryOptions = () =>
  queryOptions({
    queryKey: teamKeys.permissions(),
    queryFn: async ({ signal }) => {
      const { data } = await listPermissions({ signal, throwOnError: true })
      return data.permissions ?? []
    },
    staleTime: Infinity,
  })

export const seatsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.seats(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await getSeats({ path: { workspaceId }, signal, throwOnError: true })
      return data
    },
  })

export const rolesQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.roles(workspaceId),
    queryFn: async ({ signal }): Promise<Role[]> => {
      const { data } = await listRoles({ path: { workspaceId }, signal, throwOnError: true })
      return (data.roles ?? []).map((r) => ({
        ...r,
        system_key: r.system_key ?? null,
        permissions: ids(r.permissions),
      }))
    },
  })

export const membersQueryOptions = (workspaceId: string, filter: MemberFilter = {}) =>
  infiniteQueryOptions({
    queryKey: teamKeys.members(workspaceId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listMembers({
        path: { workspaceId },
        query: { ...filter, cursor: pageParam, page_size: 100 },
        signal,
        throwOnError: true,
      })
      return {
        members: (data.members ?? []).map((m) => ({
          ...m,
          role_key: m.role_key ?? null,
          team_ids: ids(m.team_ids),
        })),
        next: data.next_cursor ?? undefined,
      }
    },
    getNextPageParam: (last) => last.next,
  })

export const memberQueryOptions = (workspaceId: string, userId: string) =>
  queryOptions({
    queryKey: teamKeys.member(workspaceId, userId),
    queryFn: async ({ signal }): Promise<MemberDetail> => {
      const { data } = await getMember({
        path: { workspaceId, userId },
        signal,
        throwOnError: true,
      })
      return {
        ...data,
        role_key: data.role_key ?? null,
        chat_scope: data.chat_scope ?? null,
        team_ids: ids(data.team_ids),
        number_ids: ids(data.number_ids),
        effective_permissions: ids(data.effective_permissions),
        overrides: data.overrides ?? [],
      }
    },
  })

export const teamsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.teams(workspaceId),
    queryFn: async ({ signal }): Promise<Team[]> => {
      const { data } = await listTeams({ path: { workspaceId }, signal, throwOnError: true })
      return (data.teams ?? []).map((t) => ({
        ...t,
        number_id: t.number_id ?? null,
        member_ids: ids(t.member_ids),
      }))
    },
  })

export const invitationsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.invitations(workspaceId),
    queryFn: async ({ signal }): Promise<Invitation[]> => {
      const { data } = await listInvitations({ path: { workspaceId }, signal, throwOnError: true })
      return (data.invitations ?? []).map((i) => ({ ...i, team_ids: ids(i.team_ids) }))
    },
  })

export type LogFilter = { action?: string; member?: string; since?: string }

export const accessLogQueryOptions = (workspaceId: string, filter: LogFilter = {}, pageSize = 50) =>
  infiniteQueryOptions({
    queryKey: teamKeys.log(workspaceId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listAccessLog({
        path: { workspaceId },
        query: {
          action: filter.action || undefined,
          member: filter.member || undefined,
          since: filter.since || undefined,
          cursor: pageParam,
          page_size: pageSize,
        },
        signal,
        throwOnError: true,
      })
      return { entries: data.entries ?? [], next: data.next_cursor ?? undefined }
    },
    getNextPageParam: (last) => last.next,
  })

export const invitePreviewQueryOptions = (token: string) =>
  queryOptions({
    queryKey: teamKeys.invite(token),
    queryFn: async () => {
      const { data } = await previewInvitation({ body: { token }, throwOnError: true })
      return {
        ...data,
        can_do: ids(data.can_do),
        permissions: ids(data.permissions),
        team_names: ids(data.team_names),
        number_labels: ids(data.number_labels),
      }
    },
    retry: false,
    staleTime: Infinity,
  })

export const accessLogSummaryQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.logSummary(workspaceId),
    queryFn: async ({ signal }) => {
      const { data } = await getAccessLogSummary({
        path: { workspaceId },
        signal,
        throwOnError: true,
      })
      return { ...data, expiring: data.expiring ?? [], alerts: data.alerts ?? [] }
    },
  })

/** Every team change can move access around, so it refreshes the whole workspace's team data. */
function useTeamMutation<TInput, TOutput>(
  workspaceId: string,
  fn: (input: TInput) => Promise<TOutput>,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSuccess: () => invalidateTeam(queryClient, workspaceId),
  })
}

export function invalidateTeam(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: teamKeys.workspace(workspaceId) })
}

// ---------- roles ----------

export function useCreateRole(workspaceId: string) {
  return useTeamMutation(workspaceId, async (body: RoleInput) => {
    const { data } = await createRole({ path: { workspaceId }, body, throwOnError: true })
    return data
  })
}

export function useUpdateRole(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ roleId, body }: { roleId: string; body: RoleInput }) => {
      const { data } = await updateRole({ path: { workspaceId, roleId }, body, throwOnError: true })
      return data
    },
  )
}

export async function fetchRolePreview(
  workspaceId: string,
  roleId: string,
  body: RoleInput,
): Promise<RoleChange> {
  const { data } = await previewRole({ path: { workspaceId, roleId }, body, throwOnError: true })
  return toChange(data)
}

export function useResetRole(workspaceId: string) {
  return useTeamMutation(workspaceId, async (roleId: string) => {
    await resetRole({ path: { workspaceId, roleId }, throwOnError: true })
  })
}

export function useDeleteRole(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ roleId, moveTo }: { roleId: string; moveTo: string }) => {
      await deleteRole({
        path: { workspaceId, roleId },
        query: { move_to_role_id: moveTo },
        throwOnError: true,
      })
    },
  )
}

// ---------- members ----------

function toChange(data: Awaited<ReturnType<typeof previewRole>>['data'] & object): RoleChange {
  return {
    gains: ids(data.gains),
    losses: ids(data.losses),
    sensitive_gain: data.sensitive_gain,
    affected: data.affected ?? [],
  }
}

export async function fetchMemberRolePreview(
  workspaceId: string,
  userId: string,
  roleId: string,
): Promise<RoleChange> {
  const { data } = await previewMemberRole({
    path: { workspaceId, userId },
    body: { role_id: roleId },
    throwOnError: true,
  })
  return toChange(data)
}

export function useChangeMemberRole(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (roleId: string) => {
    await changeMemberRole({
      path: { workspaceId, userId },
      body: { role_id: roleId },
      throwOnError: true,
    })
  })
}

export function useUpdateMemberAccess(workspaceId: string, userId: string) {
  return useTeamMutation(
    workspaceId,
    async (body: { chat_scope?: ChatScope | ''; team_ids?: string[]; number_ids?: string[] }) => {
      await updateMemberAccess({ path: { workspaceId, userId }, body, throwOnError: true })
    },
  )
}

export function useAddOverride(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (body: OverrideBody) => {
    await addMemberOverride({ path: { workspaceId, userId }, body, throwOnError: true })
  })
}

export function useRevokeOverride(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (overrideId: string) => {
    await revokeMemberOverride({ path: { workspaceId, userId, overrideId }, throwOnError: true })
  })
}

export function useSuspendMember(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (reason: string) => {
    await suspendMember({ path: { workspaceId, userId }, body: { reason }, throwOnError: true })
  })
}

export function useRestoreMember(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async () => {
    await restoreMember({ path: { workspaceId, userId }, throwOnError: true })
  })
}

export function useRemoveMember(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (confirmName: string) => {
    await removeMember({
      path: { workspaceId, userId },
      body: { confirm_name: confirmName },
      throwOnError: true,
    })
  })
}

export function useLeaveWorkspace(workspaceId: string) {
  return useMutation({
    mutationFn: async () => {
      await leaveWorkspace({ path: { workspaceId }, throwOnError: true })
    },
  })
}

// ---------- teams ----------

export function useCreateTeam(workspaceId: string) {
  return useTeamMutation(workspaceId, async (body: TeamInput) => {
    const { data } = await createTeam({ path: { workspaceId }, body, throwOnError: true })
    return data
  })
}

export function useUpdateTeam(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ teamId, body }: { teamId: string; body: TeamInput }) => {
      await updateTeam({ path: { workspaceId, teamId }, body, throwOnError: true })
    },
  )
}

export function useSetTeamMembers(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ teamId, userIds }: { teamId: string; userIds: string[] }) => {
      await setTeamMembers({
        path: { workspaceId, teamId },
        body: { user_ids: userIds },
        throwOnError: true,
      })
    },
  )
}

export function useDeleteTeam(workspaceId: string) {
  return useTeamMutation(workspaceId, async (teamId: string) => {
    await deleteTeam({ path: { workspaceId, teamId }, throwOnError: true })
  })
}

// ---------- invitations ----------

export function useInvite(workspaceId: string) {
  return useTeamMutation(workspaceId, async (body: InviteInput) => {
    const { data } = await createInvitations({ path: { workspaceId }, body, throwOnError: true })
    return (data.invitations ?? []).map((i): Invitation => ({ ...i, team_ids: ids(i.team_ids) }))
  })
}

export function useResendInvitation(workspaceId: string) {
  return useTeamMutation(workspaceId, async (invitationId: string) => {
    await resendInvitation({ path: { workspaceId, invitationId }, throwOnError: true })
  })
}

export function useRevokeInvitation(workspaceId: string) {
  return useTeamMutation(workspaceId, async (invitationId: string) => {
    await revokeInvitation({ path: { workspaceId, invitationId }, throwOnError: true })
  })
}

export function useAcceptInvitation() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (token: string) => {
      const { data } = await acceptInvitation({ body: { token }, throwOnError: true })
      return data.workspace_id
    },
    // The workspace list lives in features/workspaces; refetch everything on the next page.
    onSuccess: () => queryClient.invalidateQueries(),
  })
}

export function useRequestRenewal() {
  return useMutation({
    mutationFn: async (token: string) => {
      await requestInvitationRenewal({ body: { token }, throwOnError: true })
    },
  })
}

/** Creates an account from the invite link, signs in and joins (no separate verification). */
export function useInviteSignup() {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (body: {
      token: string
      first_name: string
      last_name?: string
      password: string
      accept_terms: boolean
    }) => {
      const { data } = await inviteSignup({
        body: { ...body, timezone: Intl.DateTimeFormat().resolvedOptions().timeZone },
        throwOnError: true,
      })
      return data
    },
    onSuccess: () => queryClient.invalidateQueries(),
  })
}
