import { useQuery } from '@tanstack/react-query'
import { PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { catalogQueryOptions, resourcesQueryOptions } from '../../api/admin.api'
import { CheckField } from '../kit/check-field'
import { LoadError } from '../kit/load-error'
import { OfferDialog } from './offer-dialog'
import { type PlanAction, PlanCard } from './plan-card'
import { PlanSheet } from './plan-sheet'
import { PriceDialog } from './price-dialog'
import { ProviderPriceDialog } from './provider-price-dialog'

type Open = PlanAction | { kind: 'plan' } | null

export function CatalogPanel() {
  const catalog = useQuery(catalogQueryOptions())
  const resources = useQuery(resourcesQueryOptions())
  const [open, setOpen] = useState<Open>(null)
  const [showInactive, setShowInactive] = useState(false)
  const close = () => setOpen(null)

  if (catalog.isPending || resources.isPending) return <Skeleton className="h-[240px] rounded-xl" />
  if (catalog.isError) return <LoadError error={catalog.error} onRetry={() => catalog.refetch()} />
  if (resources.isError) {
    return <LoadError error={resources.error} onRetry={() => resources.refetch()} />
  }
  const { plans, providers } = catalog.data
  const shown = plans.filter((p) => showInactive || p.is_active)

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-sm text-muted-foreground">
          Providers on this server:{' '}
          {providers.length ? providers.join(', ') : 'none (set PAYMENT_PROVIDERS)'}
        </p>
        <div className="flex flex-wrap items-center gap-4">
          <CheckField
            id="show-inactive"
            label="Show inactive"
            checked={showInactive}
            onChange={setShowInactive}
          />
          <Button size="sm" onClick={() => setOpen({ kind: 'plan' })}>
            <PlusIcon aria-hidden="true" />
            Add plan
          </Button>
        </div>
      </div>

      {shown.length === 0 ? (
        <p className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          No plans yet. Every workspace is on the free plan. Add the plan TRIAL_PLAN names (default
          “growth”) so new workspaces get a trial.
        </p>
      ) : (
        shown.map((plan) => (
          <PlanCard
            key={plan.id}
            plan={plan}
            resources={resources.data}
            showInactive={showInactive}
            onAction={setOpen}
          />
        ))
      )}

      {open?.kind === 'plan' && (
        <PlanSheet mode={{ kind: 'create' }} resources={resources.data} onClose={close} />
      )}
      {open?.kind === 'version' && (
        <PlanSheet
          mode={{ kind: 'version', from: open.plan }}
          resources={resources.data}
          onClose={close}
        />
      )}
      {open?.kind === 'edit' && (
        <PlanSheet
          mode={{ kind: 'edit', plan: open.plan }}
          resources={resources.data}
          onClose={close}
        />
      )}
      {open?.kind === 'price' && <PriceDialog plan={open.plan} onClose={close} />}
      {open?.kind === 'offer' && <OfferDialog price={open.price} onClose={close} />}
      {open?.kind === 'provider' && (
        <ProviderPriceDialog
          price={open.price}
          offer={open.offer}
          enabled={providers}
          onClose={close}
        />
      )}
    </div>
  )
}
