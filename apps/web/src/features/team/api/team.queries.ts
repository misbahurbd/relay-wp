import {
  getAccessLogSummary,
  getMember,
  getMyAccess,
  getSeats,
  listAccessLog,
  listInvitations,
  listMembers,
  listPermissions,
  listRoles,
  listTeams,
  previewInvitation,
} from '@relay/api-client'
import { infiniteQueryOptions, queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  Access,
  Invitation,
  LogFilter,
  MemberDetail,
  MemberFilter,
  Role,
  Team,
} from '../types'
import { teamKeys } from './team.keys'

export const ids = (v: string[] | null | undefined) => v ?? []

export const accessQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.access(workspaceId),
    queryFn: async ({ signal }): Promise<Access> => {
      const { data } = await getMyAccess({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
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
      const { data } = await getSeats({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return data
    },
  })

export const rolesQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: teamKeys.roles(workspaceId),
    queryFn: async ({ signal }): Promise<Role[]> => {
      const { data } = await listRoles({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
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
        headers: inWorkspace(workspaceId),
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
        headers: inWorkspace(workspaceId),
        path: { userId },
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
      const { data } = await listTeams({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
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
      const { data } = await listInvitations({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return (data.invitations ?? []).map((i) => ({ ...i, team_ids: ids(i.team_ids) }))
    },
  })

export const accessLogQueryOptions = (workspaceId: string, filter: LogFilter = {}, pageSize = 50) =>
  infiniteQueryOptions({
    queryKey: teamKeys.log(workspaceId, filter),
    initialPageParam: undefined as string | undefined,
    queryFn: async ({ signal, pageParam }) => {
      const { data } = await listAccessLog({
        headers: inWorkspace(workspaceId),
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
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return { ...data, expiring: data.expiring ?? [], alerts: data.alerts ?? [] }
    },
  })
