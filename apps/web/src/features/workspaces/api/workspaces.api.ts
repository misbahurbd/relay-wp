import { createWorkspace, getWorkspace, listWorkspaces, type WorkspaceDto } from '@relay/api-client'
import { type QueryClient, queryOptions, useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { FirstGoal, StartMode, Workspace, WorkspaceSummary } from '../types'

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
      selectWorkspace(ws.id)
      return queryClient.invalidateQueries({ queryKey: workspaceKeys.all(userId) })
    },
  })
}

/** Whole days left in the trial, or null when there is no trial running. */
export function trialDaysLeft(endsAt: string | null): number | null {
  if (!endsAt) return null
  return Math.max(0, Math.ceil((Date.parse(endsAt) - Date.now()) / 86_400_000))
}

const WORKSPACE_KEY = 'relay-last-workspace'

/**
 * Makes id the workspace this tab works in. Each tab keeps its own choice (sessionStorage), and
 * the latest one is what new tabs open (localStorage). The API gets it as X-Workspace-ID.
 */
export function selectWorkspace(id: string) {
  for (const store of [sessionStorage, localStorage]) {
    try {
      store.setItem(WORKSPACE_KEY, id)
    } catch {
      // Not remembered; the most recently used workspace is opened next time.
    }
  }
}

function selectedWorkspace() {
  for (const store of [sessionStorage, localStorage]) {
    try {
      const id = store.getItem(WORKSPACE_KEY)
      if (id) return id
    } catch {
      // Storage blocked: fall through to the most recently used workspace.
    }
  }
  return null
}

/**
 * The workspace to open: the one this tab picked, else the last one picked anywhere, else the
 * most recently used (the API lists those first). Null when the user has none.
 */
export async function resolveWorkspace(
  queryClient: QueryClient,
  userId: string,
): Promise<WorkspaceSummary | null> {
  const selected = selectedWorkspace()
  let workspaces = await queryClient.ensureQueryData(workspacesQueryOptions(userId))
  // A workspace joined since the list was cached: refetch once before falling back.
  if (selected && !workspaces.some((w) => w.id === selected)) {
    workspaces = await queryClient.fetchQuery(workspacesQueryOptions(userId))
  }
  return workspaces.find((w) => w.id === selected) ?? workspaces[0] ?? null
}

export type HomePath = { to: '/welcome' } | { to: '/setup' } | { to: '/' }

/**
 * Where a signed-in, verified user belongs (docs/PHASE-3-ONBOARDING.md, "Where to send a
 * signed-in user"): no workspace → Welcome; unfinished setup → Setup; otherwise the dashboard.
 */
export async function resolveHomePath(queryClient: QueryClient, userId: string): Promise<HomePath> {
  const ws = await resolveWorkspace(queryClient, userId)
  if (!ws) return { to: '/welcome' }
  return ws.completed ? { to: '/' } : { to: '/setup' }
}
