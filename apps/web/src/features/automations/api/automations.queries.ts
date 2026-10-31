import { listAutomations } from '@relay/api-client'
import { queryOptions } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type { Automation } from '../types'
import { automationsKeys } from './automations.keys'

export const automationsQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: automationsKeys.list(workspaceId),
    retry: false,
    queryFn: async ({ signal }): Promise<Automation[]> => {
      const { data } = await listAutomations({
        headers: inWorkspace(workspaceId),
        signal,
        throwOnError: true,
      })
      return (data.items ?? []) as Automation[]
    },
  })
