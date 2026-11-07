import { useQuery } from '@tanstack/react-query'
import { TagIcon } from 'lucide-react'
import { type ReactNode, useState } from 'react'
import { Skeleton } from '#/components/ui/skeleton'
import { onSale, plansQueryOptions, resourcesQueryOptions } from '../../api/billing.queries'
import type { Cadence, Catalog } from '../../types'
import { EmptyState, ErrorState } from '../kit/states'
import { cadencesOf, type PlanAction, PricingTable } from './pricing-table'

function useCadence(catalog: Catalog | undefined) {
  const [chosen, setChosen] = useState<Cadence>()
  const available = catalog ? cadencesOf(catalog) : []
  const cadence = chosen && available.includes(chosen) ? chosen : (available[0] ?? 'month')
  return [cadence, setChosen] as const
}

/** The plan catalog with its cadence toggle; action renders each plan's button. */
export function Catalogue({
  action,
  currentPriceId,
  hadTrial,
  header,
  comparison,
}: {
  action: PlanAction
  currentPriceId?: string
  /** See PricingTable. Leave it out without a workspace. */
  hadTrial?: boolean
  header?: (toggle: ReactNode) => ReactNode
  comparison?: 'expanded' | 'collapsed'
}) {
  const plans = useQuery({
    ...plansQueryOptions(),
    select: (catalog) => onSale(catalog, currentPriceId),
  })
  // Without the resource catalog the cards still show staff's lines; they just lose the limits.
  const resources = useQuery(resourcesQueryOptions())
  const [cadence, setCadence] = useCadence(plans.data)

  if (plans.isPending || resources.isPending) return <PricingSkeleton />
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
      resources={resources.data ?? []}
      cadence={cadence}
      onCadence={setCadence}
      currentPriceId={currentPriceId}
      action={action}
      hadTrial={hadTrial}
      header={header}
      comparison={comparison}
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
