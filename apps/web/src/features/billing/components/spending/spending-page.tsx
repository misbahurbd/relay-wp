import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronRightIcon, LockIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { cn } from '#/lib/utils'
import {
  campaignSpendPreviewQueryOptions,
  spendingBudgetPreviewQueryOptions,
  spendingPreviewQueryOptions,
  walletLedgerPreviewQueryOptions,
  walletOutlookPreviewQueryOptions,
  walletPreviewQueryOptions,
} from '../../api/billing.preview'
import type { SpendingPeriod } from '../../types'
import { PreviewBadge } from '../kit/preview-badge'
import { EmptyState, ErrorState } from '../kit/states'
import { BudgetCard } from './budget-card'
import { ByCategory } from './by-category'
import { ByCountry } from './by-country'
import { ByNumber } from './by-number'
import { CampaignsTable } from './campaigns-table'
import { HowMetaCharges } from './how-meta-charges'
import { LowBalanceAlert } from './low-balance-alert'
import { SpendChart } from './spend-chart'
import { SummaryCards } from './summary-cards'
import { TopUpDialog } from './top-up-dialog'
import { WalletLedger } from './wallet-ledger'
import { WalletPanel } from './wallet-panel'

const PERIODS: { id: SpendingPeriod; label: string }[] = [
  { id: 'this_month', label: 'This month' },
  { id: 'last_month', label: 'Last month' },
  { id: 'last_3_months', label: 'Last 3 months' },
]

/**
 * The Spending page's sample data (docs/PHASE-7-BILLING.md). Switching period keeps the previous
 * numbers on screen until the next ones arrive.
 */
function useSpendingPreview(workspaceId: string, period: SpendingPeriod) {
  const queries = {
    spending: useQuery({
      ...spendingPreviewQueryOptions(workspaceId, period),
      placeholderData: keepPreviousData,
    }),
    budget: useQuery(spendingBudgetPreviewQueryOptions(workspaceId)),
    campaigns: useQuery(campaignSpendPreviewQueryOptions(workspaceId)),
    wallet: useQuery(walletPreviewQueryOptions(workspaceId)),
    outlook: useQuery(walletOutlookPreviewQueryOptions(workspaceId)),
    ledger: useQuery(walletLedgerPreviewQueryOptions(workspaceId)),
  }
  const all = Object.values(queries)
  const failed = all.find((q) => q.isError)
  const { spending, budget, campaigns, wallet, outlook, ledger } = queries
  const data =
    spending.data && budget.data && campaigns.data && wallet.data && outlook.data && ledger.data
      ? {
          spending: spending.data,
          budget: budget.data,
          campaigns: campaigns.data,
          wallet: wallet.data,
          outlook: outlook.data,
          ledger: ledger.data,
        }
      : undefined
  return {
    data,
    error: failed?.error,
    retry: () => {
      for (const q of all) if (q.isError) void q.refetch()
    },
  }
}

export function SpendingPage({ workspaceId, canView }: { workspaceId: string; canView: boolean }) {
  const [period, setPeriod] = useState<SpendingPeriod>('this_month')

  return (
    <main className="mx-auto box-border flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1.5">
          <nav
            aria-label="Breadcrumb"
            className="flex items-center gap-1.5 text-[13px] text-muted-foreground"
          >
            <Link
              to="/billing"
              className="font-bold text-brand-text underline hover:text-foreground"
            >
              Billing
            </Link>
            <ChevronRightIcon className="size-3.5" aria-hidden="true" />
            <span aria-current="page">Spending</span>
          </nav>
          <h1 className="flex flex-wrap items-center gap-2.5 text-[28px] font-extrabold tracking-[-0.02em]">
            Spending &amp; wallet <PreviewBadge />
          </h1>
          <p className="text-[15px] text-muted-foreground">
            Everything you pay: your Relay plan plus Meta’s per-message charges.
          </p>
        </div>
        {canView && (
          <div
            role="tablist"
            aria-label="Period"
            className="inline-flex flex-wrap rounded-xl border bg-card p-1"
          >
            {PERIODS.map((p) => {
              const on = p.id === period
              return (
                <Button
                  variant="unstyled"
                  key={p.id}
                  type="button"
                  role="tab"
                  aria-selected={on}
                  onClick={() => setPeriod(p.id)}
                  className={cn(
                    'h-[38px] rounded-[9px] px-3.5 text-[13px] font-bold',
                    on ? 'bg-primary text-primary-foreground' : 'text-foreground',
                  )}
                >
                  {p.label}
                </Button>
              )
            })}
          </div>
        )}
      </div>

      {canView ? (
        <SpendingBody workspaceId={workspaceId} period={period} />
      ) : (
        <EmptyState
          icon={<LockIcon className="size-5" />}
          title="Billing is limited to some members"
        >
          Ask the workspace owner for the “View billing” permission to see spending and the wallet.
        </EmptyState>
      )}
    </main>
  )
}

function SpendingBody({ workspaceId, period }: { workspaceId: string; period: SpendingPeriod }) {
  const { data, error, retry } = useSpendingPreview(workspaceId, period)
  const [topUpOpen, setTopUpOpen] = useState(false)
  const openTopUp = () => setTopUpOpen(true)

  if (error) return <ErrorState error={error} onRetry={retry} />
  if (!data) {
    return (
      <div
        className="flex flex-col gap-5"
        role="status"
        aria-busy="true"
        aria-label="Loading spending"
      >
        <div className="grid grid-cols-[repeat(auto-fit,minmax(200px,1fr))] gap-3.5">
          {[1, 2, 3, 4, 5].map((n) => (
            <Skeleton key={n} className="h-[112px] rounded-xl" />
          ))}
        </div>
        <Skeleton className="h-[340px] rounded-xl" />
        <Skeleton className="h-[280px] rounded-xl" />
      </div>
    )
  }
  return (
    <>
      <LowBalanceAlert wallet={data.wallet} outlook={data.outlook} onTopUp={openTopUp} />
      <SummaryCards spending={data.spending} />

      <div className="flex flex-wrap gap-5">
        <BudgetCard budget={data.budget} className="min-w-0 flex-[1_1_320px]" />
        <SpendChart spending={data.spending} className="min-w-0 flex-[2_1_560px]" />
      </div>

      <div className="flex flex-wrap gap-5">
        <ByCategory spending={data.spending} className="min-w-0 flex-[3_1_560px]" />
        <HowMetaCharges className="flex-[1_1_300px]" />
      </div>

      <CampaignsTable campaigns={data.campaigns} />

      <div className="grid grid-cols-[repeat(auto-fit,minmax(min(320px,100%),1fr))] gap-5">
        <ByNumber spending={data.spending} />
        <ByCountry spending={data.spending} />
      </div>

      <div className="flex flex-wrap gap-5">
        <WalletPanel
          wallet={data.wallet}
          outlook={data.outlook}
          onTopUp={openTopUp}
          className="min-w-0 flex-[1_1_320px]"
        />
        <WalletLedger
          entries={data.ledger}
          currency={data.wallet.currency}
          className="min-w-0 flex-[2_1_560px]"
        />
      </div>

      <TopUpDialog
        open={topUpOpen}
        onClose={() => setTopUpOpen(false)}
        wallet={data.wallet}
        outlook={data.outlook}
      />
    </>
  )
}
