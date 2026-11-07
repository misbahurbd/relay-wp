import { useQuery } from '@tanstack/react-query'
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
import { disputesQueryOptions } from '../../api/admin.api'
import type { DisputedInvoice } from '../../types'
import { LoadError } from '../kit/load-error'
import { formatMoney } from '../kit/money'
import { CloseDisputeDialog } from './close-dispute-dialog'

const day = (s: string) => new Date(s).toLocaleDateString()

/** Open disputes. Creem never says how one ended, so staff record it here. */
export function DisputesPanel() {
  const disputes = useQuery(disputesQueryOptions())
  const [closing, setClosing] = useState<DisputedInvoice | null>(null)

  if (disputes.isPending) return <Skeleton className="h-[240px] rounded-xl" />
  if (disputes.isError) {
    return <LoadError error={disputes.error} onRetry={() => disputes.refetch()} />
  }

  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        Stripe reports how a dispute ends; Creem doesn’t. Once the provider’s dashboard shows the
        outcome, record it here.
      </p>
      {disputes.data.length === 0 ? (
        <p className="rounded-xl border border-dashed px-4 py-8 text-center text-sm text-muted-foreground">
          No open disputes.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-xl border bg-card">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Invoice</TableHead>
                <TableHead>Workspace</TableHead>
                <TableHead>Amount</TableHead>
                <TableHead>Provider</TableHead>
                <TableHead>Disputed since</TableHead>
                <TableHead />
              </TableRow>
            </TableHeader>
            <TableBody>
              {disputes.data.map((d) => (
                <TableRow key={d.id}>
                  <TableCell className="font-bold">{d.number || '—'}</TableCell>
                  <TableCell>{d.workspace_name}</TableCell>
                  <TableCell className="tabular-nums">
                    {formatMoney(d.total_minor, d.currency)}
                  </TableCell>
                  <TableCell className="capitalize">{d.provider}</TableCell>
                  <TableCell className="text-sm">{day(d.disputed_since)}</TableCell>
                  <TableCell className="text-right">
                    <Button size="sm" variant="outline" onClick={() => setClosing(d)}>
                      Close dispute
                    </Button>
                  </TableCell>
                </TableRow>
              ))}
            </TableBody>
          </Table>
        </div>
      )}
      {closing && <CloseDisputeDialog invoice={closing} onClose={() => setClosing(null)} />}
    </div>
  )
}
