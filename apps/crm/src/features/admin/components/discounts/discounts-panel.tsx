import { useQuery } from '@tanstack/react-query'
import { PlusIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '#/components/ui/table'
import {
  catalogQueryOptions,
  discountsQueryOptions,
  useDeactivateDiscount,
} from '../../api/admin.api'
import type { AdminDiscount } from '../../types'
import { ActiveBadge } from '../kit/active-badge'
import { DeactivateButton } from '../kit/deactivate-button'
import { LoadError } from '../kit/load-error'
import { formatMoney } from '../kit/money'
import { DiscountDialog } from './discount-dialog'

function off(d: AdminDiscount) {
  if (d.kind === 'percent') return `${(d.percent_bps ?? 0) / 100}%`
  return formatMoney(d.amount_off_minor ?? 0, d.currency ?? 'USD')
}

function lasts(d: AdminDiscount) {
  if (d.duration === 'repeating') return `${d.duration_months} months`
  return d.duration === 'forever' ? 'forever' : 'once'
}

const day = (s: string) => new Date(s).toLocaleDateString()

export function DiscountsPanel() {
  const discounts = useQuery(discountsQueryOptions())
  const catalog = useQuery(catalogQueryOptions())
  const deactivate = useDeactivateDiscount()
  const [adding, setAdding] = useState(false)

  if (discounts.isPending) return <Skeleton className="h-[240px] rounded-xl" />
  if (discounts.isError) {
    return <LoadError error={discounts.error} onRetry={() => discounts.refetch()} />
  }
  const plans = catalog.data?.plans ?? []
  const planName = (id: string | null) => {
    const p = plans.find((x) => x.id === id)
    return p ? `${p.code} v${p.version}` : 'any'
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex justify-end">
        <Button size="sm" onClick={() => setAdding(true)}>
          <PlusIcon aria-hidden="true" />
          Add code
        </Button>
      </div>
      {discounts.data.length === 0 ? (
        <p className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          No discount codes yet.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Code</TableHead>
                <TableHead>Off</TableHead>
                <TableHead>Applies to</TableHead>
                <TableHead>Used</TableHead>
                <TableHead>Valid</TableHead>
                <TableHead>Status</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {discounts.data.map((d) => (
                <TableRow key={d.id}>
                  <TableCell>
                    <div className="font-bold">{d.code}</div>
                    {d.name !== d.code && (
                      <div className="text-xs text-muted-foreground">{d.name}</div>
                    )}
                  </TableCell>
                  <TableCell>
                    {off(d)} <span className="text-xs text-muted-foreground">{lasts(d)}</span>
                  </TableCell>
                  <TableCell className="text-sm">
                    {d.applies_to.replace('_', ' ')} · plan {planName(d.plan_id)}
                    {!d.allow_installments && ' · no installments'}
                  </TableCell>
                  <TableCell className="tabular-nums">
                    {d.redeemed_count}
                    {d.max_redemptions != null && ` / ${d.max_redemptions}`}
                    <div className="text-xs text-muted-foreground">
                      {d.per_workspace_limit} per workspace
                    </div>
                  </TableCell>
                  <TableCell className="text-sm">
                    {day(d.starts_at)} – {d.expires_at ? day(d.expires_at) : 'no end'}
                  </TableCell>
                  <TableCell>
                    <ActiveBadge active={d.is_active} />
                  </TableCell>
                  <TableCell className="text-right">
                    {d.is_active && (
                      <DeactivateButton
                        what={d.code}
                        run={() => deactivate.mutateAsync(d.code)}
                        pending={deactivate.isPending}
                      />
                    )}
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {adding && (
        <DiscountDialog plans={plans.filter((p) => p.is_active)} onClose={() => setAdding(false)} />
      )}
    </div>
  )
}
