import { useQuery } from '@tanstack/react-query'
import { LockIcon } from 'lucide-react'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'
import { overviewQueryOptions } from '../../api/billing.queries'
import { EmptyState, ErrorState } from '../kit/states'
import { CurrentPlan } from './current-plan'
import { Invoices } from './invoices'
import { LtdOrder } from './ltd-order'
import { ProfileSection } from './profile-form'
import { SubscriptionCard } from './subscription-card'

export type BillingTab = 'overview' | 'invoices' | 'details'

const TABS: { id: BillingTab; label: string }[] = [
  { id: 'overview', label: 'Plan' },
  { id: 'invoices', label: 'Invoices' },
  { id: 'details', label: 'Billing details' },
]

type Props = {
  workspaceId: string
  canView: boolean
  canPay: boolean
  tab: BillingTab
  onTab: (tab: BillingTab) => void
}

export function BillingPage({ workspaceId, canView, canPay, tab, onTab }: Props) {
  return (
    <main className="mx-auto flex w-full max-w-[960px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-col gap-1">
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Billing</h1>
        <p className="text-[15px] text-muted-foreground">
          Your plan, payments and the details on your invoices.
        </p>
      </div>
      {!canView ? (
        <EmptyState
          icon={<LockIcon className="size-5" />}
          title="Billing is limited to some members"
        >
          Ask the workspace owner for the “View billing” permission to see plans and invoices.
        </EmptyState>
      ) : (
        <>
          <div
            role="tablist"
            aria-label="Billing sections"
            className="flex flex-wrap gap-1 border-b"
          >
            {TABS.map((t) => (
              <button
                key={t.id}
                id={`billing-tab-${t.id}`}
                type="button"
                role="tab"
                aria-selected={tab === t.id}
                aria-controls="billing-panel"
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
          <div
            id="billing-panel"
            role="tabpanel"
            aria-labelledby={`billing-tab-${tab}`}
            className="flex flex-col gap-5"
          >
            {tab === 'overview' && <OverviewTab workspaceId={workspaceId} canPay={canPay} />}
            {tab === 'invoices' && <Invoices workspaceId={workspaceId} />}
            {tab === 'details' && <ProfileSection workspaceId={workspaceId} canPay={canPay} />}
          </div>
        </>
      )}
    </main>
  )
}

function OverviewTab({ workspaceId, canPay }: { workspaceId: string; canPay: boolean }) {
  const overview = useQuery(overviewQueryOptions(workspaceId))
  if (overview.isPending) {
    return (
      <div
        className="flex flex-col gap-5"
        role="status"
        aria-busy="true"
        aria-label="Loading your plan"
      >
        <Skeleton className="h-[220px] rounded-xl" />
        <Skeleton className="h-[160px] rounded-xl" />
      </div>
    )
  }
  if (overview.isError) {
    return <ErrorState error={overview.error} onRetry={() => void overview.refetch()} />
  }
  const { subscription, order } = overview.data
  return (
    <>
      <CurrentPlan overview={overview.data} canPay={canPay} />
      {order && <LtdOrder workspaceId={workspaceId} order={order} canPay={canPay} />}
      {subscription && (
        <SubscriptionCard workspaceId={workspaceId} subscription={subscription} canPay={canPay} />
      )}
    </>
  )
}
