import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { useDeferredValue, useState } from 'react'
import { toast } from 'sonner'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { Skeleton } from '#/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '#/components/ui/table'
import { errorMessage } from '#/lib/api'
import {
  catalogQueryOptions,
  resourcesQueryOptions,
  useEndGrant,
  useRestoreGrant,
  workspacesQueryOptions,
} from '../../api/admin.api'
import type { AdminWorkspace, Resource } from '../../types'
import { FREE_PLAN, limitsSummary } from '../kit/limits'
import { LoadError } from '../kit/load-error'
import { GrantDialog } from './grant-dialog'
import { OverridesDialog } from './overrides-dialog'

const day = (s: string) => new Date(s).toLocaleDateString()

function PlanCell({ w, resources }: { w: AdminWorkspace; resources: Resource[] }) {
  const g = w.grant
  const custom = g && Object.keys(g.overrides).length > 0
  return (
    <div className="flex flex-col gap-0.5">
      {g ? (
        <>
          <div className="flex flex-wrap items-center gap-1.5">
            <span className="font-bold">{g.plan_name}</span>
            <Badge variant={g.status === 'active' ? 'success' : 'warning'}>{g.status}</Badge>
            {custom && <Badge variant="neutral">own limits</Badge>}
          </div>
          <span className="text-xs text-muted-foreground">
            {g.source}
            {g.ends_at && ` · until ${day(g.ends_at)}`}
          </span>
        </>
      ) : (
        <Badge variant="neutral">free</Badge>
      )}
      <span className="text-xs text-muted-foreground">{limitsSummary(resources, w.limits)}</span>
    </div>
  )
}

export function WorkspacesPanel() {
  const [search, setSearch] = useState('')
  const deferred = useDeferredValue(search.trim())
  const workspaces = useQuery({
    ...workspacesQueryOptions(deferred),
    placeholderData: keepPreviousData,
  })
  const catalog = useQuery(catalogQueryOptions())
  const resources = useQuery(resourcesQueryOptions())
  const end = useEndGrant()
  const restore = useRestoreGrant()
  const [granting, setGranting] = useState<AdminWorkspace | null>(null)
  const [limiting, setLimiting] = useState<AdminWorkspace | null>(null)

  const act = (run: Promise<unknown>, done: string) =>
    run.then(
      () => toast.success(done),
      (e) => toast.error(errorMessage(e)),
    )
  const plans = (catalog.data?.plans ?? []).filter((p) => p.is_active && p.code !== FREE_PLAN)

  return (
    <div className="flex flex-col gap-4">
      <Input
        type="search"
        aria-label="Search workspaces"
        placeholder="Search by workspace name or creator's email"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
        className="sm:max-w-[420px]"
      />
      {workspaces.isPending ? (
        <Skeleton className="h-[240px] rounded-xl" />
      ) : workspaces.isError ? (
        <LoadError error={workspaces.error} onRetry={() => workspaces.refetch()} />
      ) : workspaces.data.length === 0 ? (
        <p className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          No workspaces match.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Workspace</TableHead>
                <TableHead>Plan</TableHead>
                <TableHead>Created</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {workspaces.data.map((w) => {
                const g = w.grant
                const paid = g?.source === 'subscription' || g?.source === 'order'
                return (
                  <TableRow key={w.id}>
                    <TableCell>
                      <div className="font-bold">{w.name}</div>
                      <div className="text-xs text-muted-foreground">
                        {w.owner_email || 'creator gone'} · <code>{w.id}</code>
                      </div>
                    </TableCell>
                    <TableCell>
                      <PlanCell w={w} resources={resources.data ?? []} />
                    </TableCell>
                    <TableCell className="text-sm">{day(w.created_at)}</TableCell>
                    <TableCell>
                      <div className="flex flex-wrap justify-end gap-1">
                        {g?.status === 'suspended' && (
                          <Button
                            size="xs"
                            variant="outline"
                            disabled={restore.isPending}
                            onClick={() =>
                              act(restore.mutateAsync(w.id), `Restored ${w.name}'s plan`)
                            }
                          >
                            Restore
                          </Button>
                        )}
                        {g && resources.data && (
                          <Button size="xs" variant="outline" onClick={() => setLimiting(w)}>
                            Limits
                          </Button>
                        )}
                        {!paid && (
                          <Button size="xs" variant="outline" onClick={() => setGranting(w)}>
                            Change plan
                          </Button>
                        )}
                        {g && !paid && (
                          <Button
                            size="xs"
                            variant="ghost"
                            disabled={end.isPending}
                            onClick={() => {
                              if (window.confirm(`Move ${w.name} to the free plan?`)) {
                                act(end.mutateAsync(w.id), `${w.name} is on the free plan`)
                              }
                            }}
                          >
                            End plan
                          </Button>
                        )}
                        {paid && (
                          <span className="text-xs text-muted-foreground">
                            Paid: manage in billing
                          </span>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })}
            </TableBody>
          </Table>
        </div>
      )}
      {granting && (
        <GrantDialog workspace={granting} plans={plans} onClose={() => setGranting(null)} />
      )}
      {limiting && resources.data && (
        <OverridesDialog
          workspace={limiting}
          resources={resources.data}
          onClose={() => setLimiting(null)}
        />
      )}
    </div>
  )
}
