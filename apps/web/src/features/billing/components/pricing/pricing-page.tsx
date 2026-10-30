import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ChevronLeftIcon, TagIcon } from 'lucide-react'
import { useState } from 'react'
import { Logo } from '#/components/layout/logo'
import { ThemeToggle } from '#/components/layout/theme-toggle'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { overviewQueryOptions, plansQueryOptions } from '../../api/billing.queries'
import type { Cadence, Catalog } from '../../types'
import { EmptyState, ErrorState, NoPayPermission } from '../kit/states'
import { ChangePlanDialog } from './change-plan-dialog'
import { cadencesOf, type PlanAction, PricingTable } from './pricing-table'

function useCadence(catalog: Catalog | undefined) {
  const [chosen, setChosen] = useState<Cadence>()
  const available = catalog ? cadencesOf(catalog) : []
  const cadence = chosen && available.includes(chosen) ? chosen : (available[0] ?? 'month')
  return [cadence, setChosen] as const
}

function Catalogue({ action, currentPriceId }: { action: PlanAction; currentPriceId?: string }) {
  const plans = useQuery(plansQueryOptions())
  const [cadence, setCadence] = useCadence(plans.data)

  if (plans.isPending) return <PricingSkeleton />
  if (plans.isError) {
    return (
      <ErrorState
        title="Couldn’t load plans"
        error={plans.error}
        onRetry={() => void plans.refetch()}
      />
    )
  }
  if (plans.data.plans.length === 0) {
    return (
      <EmptyState icon={<TagIcon className="size-5" />} title="No plans on sale right now">
        Check back soon, or keep using Relay on the free plan.
      </EmptyState>
    )
  }
  return (
    <PricingTable
      catalog={plans.data}
      cadence={cadence}
      onCadence={setCadence}
      currentPriceId={currentPriceId}
      action={action}
    />
  )
}

function PricingSkeleton() {
  return (
    <div className="flex flex-col gap-6" role="status" aria-busy="true" aria-label="Loading plans">
      <Skeleton className="h-11 w-72 self-center" />
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
        {[0, 1, 2].map((i) => (
          <Skeleton key={i} className="h-[380px] rounded-xl" />
        ))}
      </div>
    </div>
  )
}

/** The public pricing page. Buying needs an account and a workspace, so checkout asks to sign in. */
export function PublicPricingPage() {
  return (
    <div className="min-h-dvh bg-background">
      <header className="mx-auto flex w-full max-w-[1200px] items-center justify-between gap-3 px-4 py-4 sm:px-7">
        <Logo />
        <div className="flex items-center gap-2">
          <ThemeToggle />
          <Button variant="ghost" asChild>
            <Link to="/login">Log in</Link>
          </Button>
          <Button asChild className="hidden sm:inline-flex">
            <Link to="/signup">Start free</Link>
          </Button>
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-[1200px] flex-col gap-8 px-4 pt-6 pb-16 sm:px-7">
        <div className="flex flex-col items-center gap-2 text-center">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em] sm:text-4xl">
            Plans for every team
          </h1>
          <p className="max-w-prose text-[15px] text-muted-foreground">
            Pay monthly or yearly, or buy a lifetime deal once and spread it over a few months.
            Prices in USD.
          </p>
        </div>
        <Catalogue
          action={({ selection }) => (
            <Button asChild size="lg">
              <Link
                to="/billing/checkout"
                search={{ price: selection.priceId, offer: selection.offerId }}
              >
                Choose plan
              </Link>
            </Button>
          )}
        />
      </main>
    </div>
  )
}

/** Plans inside the app: switch a subscription in place, or check out a new plan. */
export function PlanPicker({ workspaceId, canPay }: { workspaceId: string; canPay: boolean }) {
  const overview = useQuery(overviewQueryOptions(workspaceId))
  const [changeTo, setChangeTo] = useState<string>()
  const subscription = overview.data?.subscription
  const live = subscription && ['trialing', 'active', 'past_due'].includes(subscription.status)
  const currentPriceId = live ? subscription.plan_price_id : overview.data?.order?.plan_price_id

  return (
    <main className="mx-auto flex w-full max-w-[1200px] flex-col gap-6 p-4 sm:p-7">
      <nav aria-label="Breadcrumb">
        <Link
          to="/billing"
          className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-brand-text no-underline hover:text-foreground"
        >
          <ChevronLeftIcon className="size-4" aria-hidden="true" />
          Billing
        </Link>
      </nav>
      <div className="flex flex-col gap-1">
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Change plan</h1>
        <p className="text-[15px] text-muted-foreground">
          Upgrades apply now and are prorated. Downgrades start when your current period ends.
        </p>
      </div>
      {!canPay && <NoPayPermission />}
      {overview.isError && (
        <ErrorState
          title="Couldn’t load your current plan"
          error={overview.error}
          onRetry={() => void overview.refetch()}
        />
      )}
      <Catalogue
        currentPriceId={currentPriceId}
        action={({ price, selection, current }) => {
          if (current) {
            return (
              <Button variant="outline" size="lg" disabled>
                Current plan
              </Button>
            )
          }
          if (!canPay) return null
          if (live && price.billing_type === 'recurring') {
            return (
              <Button size="lg" onClick={() => setChangeTo(price.id)}>
                Switch to this plan
              </Button>
            )
          }
          return (
            <Button asChild size="lg">
              <Link
                to="/billing/checkout"
                search={{ price: selection.priceId, offer: selection.offerId }}
              >
                {price.billing_type === 'one_time' ? 'Buy lifetime deal' : 'Choose plan'}
              </Link>
            </Button>
          )
        }}
      />
      {changeTo && (
        <ChangePlanDialog
          workspaceId={workspaceId}
          priceId={changeTo}
          onClose={() => setChangeTo(undefined)}
        />
      )}
    </main>
  )
}
