import { createWorkspace, getWorkspace, listWorkspaces, type WorkspaceDto } from '@relay/api-client'
import { type QueryClient, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { FirstGoal, StartMode, Workspace } from '../types'

export const workspaceKeys = {
  all: (userId: string) => ['workspaces', userId] as const,
  detail: (userId: string, id: string) => ['workspaces', userId, id] as const,
}

export const workspacesQueryOptions = (userId: string) =>
  queryOptions({
    queryKey: workspaceKeys.all(userId),
    queryFn: async ({ signal }) => {
      const { data } = await listWorkspaces({ signal, throwOnError: true })
      return data.workspaces ?? []
    },
  })

export const workspaceQueryOptions = (userId: string, id: string) =>
  queryOptions({
    queryKey: workspaceKeys.detail(userId, id),
    queryFn: async ({ signal }) => {
      const { data } = await getWorkspace({ headers: inWorkspace(id), signal, throwOnError: true })
      return toWorkspace(data)
    },
  })

export function toWorkspace(data: WorkspaceDto): Workspace {
  return { ...data, goals: data.goals ?? [] }
}

export function useCreateWorkspace(userId: string) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: async (input: { first_goal: FirstGoal; start_mode: StartMode; name: string }) => {
      const { data } = await createWorkspace({ body: input, throwOnError: true })
      return data
    },
    onSuccess: (ws) => {
      rememberWorkspace(ws.id)
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.all(userId) })
    },
  })
}

/** Whole days left in the trial, or null when there is no trial running. */
export function trialDaysLeft(endsAt: string | null): number | null {
  if (!endsAt) return null
  return Math.max(0, Math.ceil((Date.parse(endsAt) - Date.now()) / 86_400_000))
}

const LAST_WORKSPACE_KEY = 'relay-last-workspace'

export function rememberWorkspace(id: string) {
  try {
    localStorage.setItem(LAST_WORKSPACE_KEY, id)
  } catch {
    // Not remembered; the most recently used workspace is opened next time.
  }
}

function lastWorkspace() {
  try {
    return localStorage.getItem(LAST_WORKSPACE_KEY)
  } catch {
    return null
  }
}

export type HomePath =
  | { to: '/welcome' }
  | { to: '/w/$workspaceId/setup'; params: { workspaceId: string } }
  | { to: '/w/$workspaceId'; params: { workspaceId: string } }

/**
 * Where a signed-in, verified user belongs (docs/PHASE-3-ONBOARDING.md, "Where to send a
 * signed-in user"): no workspace → Welcome; unfinished setup → Setup; otherwise the dashboard.
 * The API lists workspaces most recently used first.
 */
export async function resolveHomePath(queryClient: QueryClient, userId: string): Promise<HomePath> {
  const workspaces = await queryClient.fetchQuery(workspacesQueryOptions(userId))
  if (workspaces.length === 0) return { to: '/welcome' }
  const last = lastWorkspace()
  const ws = workspaces.find((w) => w.id === last) ?? workspaces[0]
  const params = { workspaceId: ws.id }
  return ws.completed ? { to: '/w/$workspaceId', params } : { to: '/w/$workspaceId/setup', params }
}
