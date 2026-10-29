import {
  acceptInvitation,
  addMemberOverride,
  changeMemberRole,
  createInvitations,
  createRole,
  createTeam,
  deleteRole,
  deleteTeam,
  inviteSignup,
  leaveWorkspace,
  type OverrideBody,
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
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { ChatScope, Invitation, InviteInput, RoleChange, RoleInput, TeamInput } from '../types'
import { invalidateTeam } from './team.keys'
import { ids } from './team.queries'

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

// ---------- roles ----------

export function useCreateRole(workspaceId: string) {
  return useTeamMutation(workspaceId, async (body: RoleInput) => {
    const { data } = await createRole({
      headers: inWorkspace(workspaceId),
      body,
      throwOnError: true,
    })
    return data
  })
}

export function useUpdateRole(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ roleId, body }: { roleId: string; body: RoleInput }) => {
      const { data } = await updateRole({
        headers: inWorkspace(workspaceId),
        path: { roleId },
        body,
        throwOnError: true,
      })
      return data
    },
  )
}

export async function fetchRolePreview(
  workspaceId: string,
  roleId: string,
  body: RoleInput,
): Promise<RoleChange> {
  const { data } = await previewRole({
    headers: inWorkspace(workspaceId),
    path: { roleId },
    body,
    throwOnError: true,
  })
  return toChange(data)
}

export function useResetRole(workspaceId: string) {
  return useTeamMutation(workspaceId, async (roleId: string) => {
    await resetRole({ headers: inWorkspace(workspaceId), path: { roleId }, throwOnError: true })
  })
}

export function useDeleteRole(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ roleId, moveTo }: { roleId: string; moveTo: string }) => {
      await deleteRole({
        headers: inWorkspace(workspaceId),
        path: { roleId },
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
    headers: inWorkspace(workspaceId),
    path: { userId },
    body: { role_id: roleId },
    throwOnError: true,
  })
  return toChange(data)
}

export function useChangeMemberRole(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (roleId: string) => {
    await changeMemberRole({
      headers: inWorkspace(workspaceId),
      path: { userId },
      body: { role_id: roleId },
      throwOnError: true,
    })
  })
}

export function useUpdateMemberAccess(workspaceId: string, userId: string) {
  return useTeamMutation(
    workspaceId,
    async (body: { chat_scope?: ChatScope | ''; team_ids?: string[]; number_ids?: string[] }) => {
      await updateMemberAccess({
        headers: inWorkspace(workspaceId),
        path: { userId },
        body,
        throwOnError: true,
      })
    },
  )
}

export function useAddOverride(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (body: OverrideBody) => {
    await addMemberOverride({
      headers: inWorkspace(workspaceId),
      path: { userId },
      body,
      throwOnError: true,
    })
  })
}

export function useRevokeOverride(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (overrideId: string) => {
    await revokeMemberOverride({
      headers: inWorkspace(workspaceId),
      path: { userId, overrideId },
      throwOnError: true,
    })
  })
}

export function useSuspendMember(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (reason: string) => {
    await suspendMember({
      headers: inWorkspace(workspaceId),
      path: { userId },
      body: { reason },
      throwOnError: true,
    })
  })
}

export function useRestoreMember(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async () => {
    await restoreMember({ headers: inWorkspace(workspaceId), path: { userId }, throwOnError: true })
  })
}

export function useRemoveMember(workspaceId: string, userId: string) {
  return useTeamMutation(workspaceId, async (confirmName: string) => {
    await removeMember({
      headers: inWorkspace(workspaceId),
      path: { userId },
      body: { confirm_name: confirmName },
      throwOnError: true,
    })
  })
}

export function useLeaveWorkspace(workspaceId: string) {
  return useMutation({
    mutationFn: async () => {
      await leaveWorkspace({ headers: inWorkspace(workspaceId), throwOnError: true })
    },
  })
}

// ---------- teams ----------

export function useCreateTeam(workspaceId: string) {
  return useTeamMutation(workspaceId, async (body: TeamInput) => {
    const { data } = await createTeam({
      headers: inWorkspace(workspaceId),
      body,
      throwOnError: true,
    })
    return data
  })
}

export function useUpdateTeam(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ teamId, body }: { teamId: string; body: TeamInput }) => {
      await updateTeam({
        headers: inWorkspace(workspaceId),
        path: { teamId },
        body,
        throwOnError: true,
      })
    },
  )
}

export function useSetTeamMembers(workspaceId: string) {
  return useTeamMutation(
    workspaceId,
    async ({ teamId, userIds }: { teamId: string; userIds: string[] }) => {
      await setTeamMembers({
        headers: inWorkspace(workspaceId),
        path: { teamId },
        body: { user_ids: userIds },
        throwOnError: true,
      })
    },
  )
}

export function useDeleteTeam(workspaceId: string) {
  return useTeamMutation(workspaceId, async (teamId: string) => {
    await deleteTeam({ headers: inWorkspace(workspaceId), path: { teamId }, throwOnError: true })
  })
}

// ---------- invitations ----------

export function useInvite(workspaceId: string) {
  return useTeamMutation(workspaceId, async (body: InviteInput) => {
    const { data } = await createInvitations({
      headers: inWorkspace(workspaceId),
      body,
      throwOnError: true,
    })
    return (data.invitations ?? []).map((i): Invitation => ({ ...i, team_ids: ids(i.team_ids) }))
  })
}

export function useResendInvitation(workspaceId: string) {
  return useTeamMutation(workspaceId, async (invitationId: string) => {
    await resendInvitation({
      headers: inWorkspace(workspaceId),
      path: { invitationId },
      throwOnError: true,
    })
  })
}

export function useRevokeInvitation(workspaceId: string) {
  return useTeamMutation(workspaceId, async (invitationId: string) => {
    await revokeInvitation({
      headers: inWorkspace(workspaceId),
      path: { invitationId },
      throwOnError: true,
    })
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
