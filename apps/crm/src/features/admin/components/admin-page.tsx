import { useQuery } from '@tanstack/react-query'
import { ShieldAlertIcon } from 'lucide-react'
import { Skeleton } from '#/components/ui/skeleton'
import { isApiError } from '#/lib/api'
import { cn } from '#/lib/utils'
import { catalogQueryOptions } from '../api/admin.api'
import { CatalogPanel } from './catalog/catalog-panel'
import { DiscountsPanel } from './discounts/discounts-panel'
import { LoadError } from './kit/load-error'
import { WorkspacesPanel } from './workspaces/workspaces-panel'

export type AdminTab = 'plans' | 'discounts' | 'workspaces'

const TABS: { id: AdminTab; label: string }[] = [
  { id: 'plans', label: 'Plans & prices' },
  { id: 'discounts', label: 'Discount codes' },
  { id: 'workspaces', label: 'Workspaces' },
]

/** Relay staff's admin tools: the plan catalog, discount codes, and each workspace's plan. */
export function AdminPage({ tab, onTab }: { tab: AdminTab; onTab: (tab: AdminTab) => void }) {
  // The catalog doubles as the access check: staff_only means this account isn't in ADMIN_EMAILS.
  const access = useQuery(catalogQueryOptions())
  return (
    <main className="mx-auto flex w-full max-w-[1080px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-col gap-1">
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Relay admin</h1>
        <p className="text-[15px] text-muted-foreground">
          Staff only. Build the plans customers can buy, issue discount codes and move workspaces
          between plans.
        </p>
      </div>
      {access.isPending ? (
        <Skeleton className="h-[240px] rounded-xl" />
      ) : access.isError && isApiError(access.error) && access.error.code === 'staff_only' ? (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed px-4 py-10 text-center">
          <ShieldAlertIcon aria-hidden="true" className="size-6 text-muted-foreground" />
          <p className="font-bold">This page is for Relay staff</p>
          <p className="max-w-[440px] text-sm text-muted-foreground">
            Add your verified email to ADMIN_EMAILS in the API's environment and restart it.
          </p>
        </div>
      ) : access.isError ? (
        <LoadError error={access.error} onRetry={() => access.refetch()} />
      ) : (
        <>
          <div role="tablist" aria-label="Admin sections" className="flex flex-wrap gap-1 border-b">
            {TABS.map((t) => (
              <button
                key={t.id}
                id={`admin-tab-${t.id}`}
                type="button"
                role="tab"
                aria-selected={tab === t.id}
                aria-controls="admin-panel"
                onClick={() => onTab(t.id)}
                className={cn(
                  '-mb-px h-11 border-b-2 px-3.5 text-sm font-bold',
                  tab === t.id
                    ? 'border-primary text-foreground'
                    : 'border-transparent text-muted-foreground hover:text-foreground',
                )}
              >
                {t.label}
              </button>
            ))}
          </div>
          <div id="admin-panel" role="tabpanel" aria-labelledby={`admin-tab-${tab}`}>
            {tab === 'plans' && <CatalogPanel />}
            {tab === 'discounts' && <DiscountsPanel />}
            {tab === 'workspaces' && <WorkspacesPanel />}
          </div>
        </>
      )}
    </main>
  )
}
