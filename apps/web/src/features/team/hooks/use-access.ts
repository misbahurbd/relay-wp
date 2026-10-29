import { client } from '@relay/api-client'
import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect } from 'react'
import { invalidateTeam, teamKeys } from '../api/team.keys'
import { accessQueryOptions } from '../api/team.queries'

/**
 * What the signed-in member can do in the workspace. While it loads, nothing is allowed, so
 * buttons appear rather than vanish. The API checks every request anyway; this only hides
 * what would fail.
 */
export function useAccess(workspaceId: string) {
  const { data } = useQuery(accessQueryOptions(workspaceId))
  const can = (...perms: string[]) =>
    !!data && (data.is_owner || perms.every((p) => data.permissions.includes(p)))
  return { access: data, can, isOwner: data?.is_owner ?? false }
}

const ACCESS_VERSION = 'X-Access-Version'
const WORKSPACE_PATH = /^\/v1\/workspaces\/([0-9a-f-]{36})\//

/**
 * Refetches the member's access when the API says it changed (docs/PHASE-3-TEAM-ACCESS.md,
 * "Telling the web app"): a new X-Access-Version, or a 403 permission_denied.
 */
export function useAccessVersionWatcher() {
  const queryClient = useQueryClient()
  useEffect(() => {
    const seen = new Map<string, string>()
    const id = client.interceptors.response.use((response) => {
      const match = WORKSPACE_PATH.exec(new URL(response.url).pathname)
      if (!match) return response
      const workspaceId = match[1]
      const version = response.headers.get(ACCESS_VERSION)
      const previous = seen.get(workspaceId)
      if (version) seen.set(workspaceId, version)
      if ((version && previous && version !== previous) || response.status === 403) {
        void queryClient.invalidateQueries({ queryKey: teamKeys.access(workspaceId) })
        if (response.status === 403) void invalidateTeam(queryClient, workspaceId)
      }
      return response
    })
    return () => client.interceptors.response.eject(id)
  }, [queryClient])
}
