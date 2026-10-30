import { useQuery } from '@tanstack/react-query'
import { LockIcon } from 'lucide-react'
import { Skeleton } from '#/components/ui/skeleton'
import { overviewQueryOptions } from '../../api/billing.queries'
import { useBillingPreview } from '../../hooks/use-billing-preview'
import { EmptyState, ErrorState } from '../kit/states'
import { ComparePlans } from '../pricing/compare-plans'
import { Invoices } from '../settings/invoices'
import { LtdOrder } from '../settings/ltd-order'
import { CurrentPlanCard } from './current-plan-card'
import { MetaCostsCard } from './meta-costs-card'
import { PaymentDetailsCard } from './payment-details-card'
import { UsageCard } from './usage-card'

type Props = { workspaceId: string; canView: boolean; canPay: boolean }

/** The billing page: plan, usage, plans, Meta costs (preview), payment details and invoices. */
export function BillingPage({ workspaceId, canView, canPay }: Props) {
  const preview = useBillingPreview()
  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-col gap-1">
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Billing</h1>
        <p className="text-[15px] text-muted-foreground">
          Your Relay plan, usage, Meta message costs and invoices in one place.
        </p>
      </div>
      {canView ? (
        <Overview workspaceId={workspaceId} canPay={canPay} preview={preview} />
      ) : (
        <EmptyState
          icon={<LockIcon className="size-5" />}
          title="Billing is limited to some members"
        >
          Ask the workspace owner for the “View billing” permission to see plans and invoices.
        </EmptyState>
      )}
    </main>
  )
}

function Overview({
  workspaceId,
  canPay,
  preview,
}: {
  workspaceId: string
  canPay: boolean
  preview: boolean
}) {
  const overview = useQuery(overviewQueryOptions(workspaceId))
  if (overview.isPending) {
    return (
      <div
        className="flex flex-col gap-5"
        role="status"
        aria-busy="true"
        aria-label="Loading billing"
      >
        <div className="flex flex-wrap gap-5">
          <Skeleton className="h-[300px] flex-[1_1_340px] rounded-xl" />
          <Skeleton className="h-[300px] flex-[2_1_480px] rounded-xl" />
        </div>
        <Skeleton className="h-[420px] rounded-xl" />
      </div>
    )
  }
  if (overview.isError) {
    return <ErrorState error={overview.error} onRetry={() => void overview.refetch()} />
  }
  const { order } = overview.data
  return (
    <>
      <div className="flex flex-wrap gap-5">
        <CurrentPlanCard
          workspaceId={workspaceId}
          overview={overview.data}
          canPay={canPay}
          className="min-w-0 flex-[1_1_340px]"
        />
        <UsageCard
          workspaceId={workspaceId}
          overview={overview.data}
          className="min-w-0 flex-[2_1_480px]"
        />
      </div>
      {order && <LtdOrder workspaceId={workspaceId} order={order} canPay={canPay} />}
      <ComparePlans workspaceId={workspaceId} canPay={canPay} />
      <div className="flex flex-wrap gap-5">
        {preview && (
          <MetaCostsCard workspaceId={workspaceId} className="min-w-0 flex-[3_1_520px]" />
        )}
        <PaymentDetailsCard
          workspaceId={workspaceId}
          canPay={canPay}
          className="min-w-0 flex-[2_1_320px]"
        />
      </div>
      <Invoices workspaceId={workspaceId} />
    </>
  )
}
