import { keepPreviousData, useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronRightIcon, FlaskConicalIcon, LockIcon } from 'lucide-react'
import { useState } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Skeleton } from '#/components/ui/skeleton'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import {
  campaignSpendPreviewQueryOptions,
  spendingBudgetPreviewQueryOptions,
  spendingPreviewQueryOptions,
  walletLedgerPreviewQueryOptions,
  walletOutlookPreviewQueryOptions,
  walletPreviewQueryOptions,
} from '../../api/billing.preview'
import type { SpendingPeriod } from '../../types'
import { PREVIEW_NOTE } from '../kit/preview-badge'
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
    <main className="mx-auto flex w-full max-w-[1200px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1">
          <nav aria-label="Breadcrumb">
            <ol className="flex items-center gap-1.5 text-[13px] font-semibold">
              <li>
                <Link to="/billing" className="text-brand-text underline-offset-4 hover:underline">
                  Billing
                </Link>
              </li>
              <li aria-hidden="true" className="text-muted-foreground">
                <ChevronRightIcon className="size-3.5" />
              </li>
              <li>
                <span aria-current="page" className="text-muted-foreground">
                  Spending
                </span>
              </li>
            </ol>
          </nav>
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Spending &amp; wallet</h1>
          <p className="text-[15px] text-muted-foreground">
            Everything you pay: your Relay plan plus Meta’s per-message charges.
          </p>
        </div>
        {canView && (
          <ToggleGroup
            type="single"
            variant="outline"
            value={period}
            // Radix lets a pressed item be unpressed; keep one period chosen.
            onValueChange={(v) => v && setPeriod(v as SpendingPeriod)}
            aria-label="Period"
          >
            {PERIODS.map((p) => (
              <ToggleGroupItem key={p.id} value={p.id} className="h-11 px-4 font-bold">
                {p.label}
              </ToggleGroupItem>
            ))}
          </ToggleGroup>
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

  return (
    <>
      <Alert variant="warning">
        <FlaskConicalIcon aria-hidden="true" />
        <AlertTitle className="font-bold">Preview</AlertTitle>
        <AlertDescription>
          {PREVIEW_NOTE} The numbers on this page are examples, not this workspace’s spending.
        </AlertDescription>
      </Alert>

      {error ? (
        <ErrorState error={error} onRetry={retry} />
      ) : !data ? (
        <div
          className="flex flex-col gap-5"
          role="status"
          aria-busy="true"
          aria-label="Loading spending"
        >
          <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
            {[1, 2, 3, 4, 5].map((n) => (
              <Skeleton key={n} className="h-[112px] rounded-xl" />
            ))}
          </div>
          <Skeleton className="h-[340px] rounded-xl" />
          <Skeleton className="h-[280px] rounded-xl" />
        </div>
      ) : (
        <>
          <LowBalanceAlert wallet={data.wallet} outlook={data.outlook} onTopUp={openTopUp} />
          <SummaryCards spending={data.spending} />

          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <BudgetCard budget={data.budget} />
            <SpendChart spending={data.spending} />
          </div>

          <div className="grid gap-5 xl:grid-cols-[minmax(0,3fr)_minmax(0,1fr)]">
            <ByCategory spending={data.spending} />
            <HowMetaCharges />
          </div>

          <CampaignsTable campaigns={data.campaigns} />

          <div className="grid gap-5 md:grid-cols-2">
            <ByNumber spending={data.spending} />
            <ByCountry spending={data.spending} />
          </div>

          <div className="grid gap-5 lg:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
            <WalletPanel wallet={data.wallet} outlook={data.outlook} onTopUp={openTopUp} />
            <WalletLedger entries={data.ledger} currency={data.wallet.currency} />
          </div>

          <TopUpDialog
            open={topUpOpen}
            onClose={() => setTopUpOpen(false)}
            wallet={data.wallet}
            outlook={data.outlook}
          />
        </>
      )}
    </>
  )
}
