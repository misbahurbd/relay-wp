import type { QueryClient } from '@tanstack/react-query'
import type { LogFilter, MemberFilter } from '../types'

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

export function invalidateTeam(queryClient: QueryClient, workspaceId: string) {
  return queryClient.invalidateQueries({ queryKey: teamKeys.workspace(workspaceId) })
}
