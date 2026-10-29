import { Button } from '#/components/ui/button'
import { useDeactivateCatalogRow } from '../../api/admin.api'
import type { CatalogRowKind, InstallmentOffer, Plan, Price, ProviderPrice } from '../../types'
import { ActiveBadge } from '../kit/active-badge'
import { DeactivateButton } from '../kit/deactivate-button'
import { formatMoney, limitText } from '../kit/money'

export type PlanAction =
  | { kind: 'version'; plan: Plan }
  | { kind: 'price'; plan: Plan }
  | { kind: 'offer'; price: Price }
  | { kind: 'provider'; price: Price; offer?: InstallmentOffer }

function useDeactivate() {
  const m = useDeactivateCatalogRow()
  return {
    pending: m.isPending,
    run: (kind: CatalogRowKind, id: string) => () => m.mutateAsync({ kind, id }),
  }
}

export function PlanCard({
  plan,
  showInactive,
  onAction,
}: {
  plan: Plan
  showInactive: boolean
  onAction: (a: PlanAction) => void
}) {
  const d = useDeactivate()
  const prices = plan.prices.filter((p) => showInactive || p.is_active)
  return (
    <section className="flex flex-col gap-4 rounded-xl border bg-card p-5 shadow-card">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <div className="flex flex-wrap items-center gap-2">
            <h3 className="text-base font-extrabold">{plan.name}</h3>
            <code className="rounded bg-muted px-1.5 text-xs">
              {plan.code} v{plan.version}
            </code>
            <ActiveBadge active={plan.is_active} />
          </div>
          <p className="text-[13px] text-muted-foreground">
            {limitText(plan.limits.members)} members · {limitText(plan.limits.whatsapp_numbers)}{' '}
            WhatsApp numbers ·{' '}
            {plan.trial_days > 0
              ? `${plan.trial_days}-day trial${plan.trial_requires_card ? ' (card)' : ''}`
              : 'no trial'}{' '}
            · sort {plan.sort_order}
          </p>
        </div>
        <div className="flex flex-wrap gap-1">
          {plan.is_active && (
            <Button size="sm" variant="outline" onClick={() => onAction({ kind: 'price', plan })}>
              Add price
            </Button>
          )}
          <Button size="sm" variant="ghost" onClick={() => onAction({ kind: 'version', plan })}>
            New version
          </Button>
          {plan.is_active && (
            <DeactivateButton
              what={`${plan.code} v${plan.version}`}
              run={d.run('plan', plan.id)}
              pending={d.pending}
            />
          )}
        </div>
      </div>

      {prices.length === 0 ? (
        <p className="rounded-lg border border-dashed px-4 py-3 text-sm text-muted-foreground">
          No prices yet. Nobody can buy this plan until it has a price linked to a provider.
        </p>
      ) : (
        <ul className="flex flex-col divide-y rounded-lg border">
          {prices.map((price) => (
            <PriceRow
              key={price.id}
              price={price}
              showInactive={showInactive}
              onAction={onAction}
              deactivate={d}
            />
          ))}
        </ul>
      )}
    </section>
  )
}

function PriceRow({
  price,
  showInactive,
  onAction,
  deactivate: d,
}: {
  price: Price
  showInactive: boolean
  onAction: (a: PlanAction) => void
  deactivate: ReturnType<typeof useDeactivate>
}) {
  const label =
    price.interval === 'month' ? '/ month' : price.interval === 'year' ? '/ year' : 'one time'
  const links = price.provider_prices.filter((p) => showInactive || p.is_active)
  const offers = price.installment_offers.filter((o) => showInactive || o.is_active)
  const onSale = price.is_active && price.provider_prices.some((p) => p.is_active)
  return (
    <li className="flex flex-col gap-2 p-3">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <div className="flex flex-wrap items-center gap-2">
          <span className="font-bold tabular-nums">
            {formatMoney(price.amount_minor, price.currency)}
          </span>
          <span className="text-sm text-muted-foreground">{label}</span>
          <ActiveBadge active={price.is_active} />
          {price.is_active && !onSale && (
            <span className="text-xs font-bold text-warning">Not on sale: link a provider</span>
          )}
        </div>
        {price.is_active && (
          <div className="flex flex-wrap gap-1">
            <Button
              size="xs"
              variant="outline"
              onClick={() => onAction({ kind: 'provider', price })}
            >
              Link provider
            </Button>
            {price.billing_type === 'one_time' && (
              <Button
                size="xs"
                variant="outline"
                onClick={() => onAction({ kind: 'offer', price })}
              >
                Add installments
              </Button>
            )}
            <DeactivateButton
              what="this price"
              run={d.run('price', price.id)}
              pending={d.pending}
            />
          </div>
        )}
      </div>
      <Links links={links} deactivate={d} />
      {offers.map((o) => (
        <div key={o.id} className="ml-3 flex flex-col gap-1 border-l-2 pl-3">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div className="flex flex-wrap items-center gap-2 text-sm">
              <span className="font-bold">
                {o.count} × {formatMoney(o.amounts_minor[0] ?? 0, price.currency)}
              </span>
              <span className="text-muted-foreground">
                total {formatMoney(o.total_minor, price.currency)} · {o.grace_days} grace days ·{' '}
                {o.max_missed} missed allowed
              </span>
              <ActiveBadge active={o.is_active} />
            </div>
            {o.is_active && price.is_active && (
              <div className="flex gap-1">
                <Button
                  size="xs"
                  variant="outline"
                  onClick={() => onAction({ kind: 'provider', price, offer: o })}
                >
                  Link provider
                </Button>
                <DeactivateButton
                  what="this installment offer"
                  run={d.run('installment_offer', o.id)}
                  pending={d.pending}
                />
              </div>
            )}
          </div>
          <Links
            links={o.provider_prices.filter((p) => showInactive || p.is_active)}
            deactivate={d}
          />
        </div>
      ))}
    </li>
  )
}

function Links({
  links,
  deactivate: d,
}: {
  links: ProviderPrice[]
  deactivate: ReturnType<typeof useDeactivate>
}) {
  if (links.length === 0) return null
  return (
    <ul className="flex flex-wrap gap-2">
      {links.map((l) => (
        <li
          key={l.id}
          className="flex items-center gap-1.5 rounded-md border bg-muted/40 py-0.5 pr-0.5 pl-2 text-xs"
        >
          <span className="font-bold">{l.provider}</span>
          {l.external_price_id && <code>{l.external_price_id}</code>}
          {l.collection && <span className="text-muted-foreground">{l.collection}</span>}
          {l.is_active ? (
            <DeactivateButton
              what={`the ${l.provider} link`}
              run={d.run('provider_price', l.id)}
              pending={d.pending}
            />
          ) : (
            <span className="px-1 text-muted-foreground">inactive</span>
          )}
        </li>
      ))}
    </ul>
  )
}
