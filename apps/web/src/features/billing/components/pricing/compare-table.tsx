import { CheckIcon, ChevronDownIcon } from 'lucide-react'
import { useId, useState } from 'react'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '#/components/ui/table'
import { cn } from '#/lib/utils'
import type { Plan } from '../../types'
import type { MatrixCell, MatrixGroup } from '../kit/plan-highlights'

type Props = {
  plans: Plan[]
  matrix: MatrixGroup[]
  /** The plan this workspace is on, whichever price; its column is marked. */
  currentPlanId?: string
  /** Open on the public page; in the app it sits behind a "Compare all features" button. */
  expanded: boolean
}

/** Every limit and feature, one column per plan. */
export function CompareTable({ plans, matrix, currentPlanId, expanded }: Props) {
  const [open, setOpen] = useState(false)
  const panelId = useId()
  if (matrix.length === 0) return null

  const table = (
    <div id={panelId} className="overflow-hidden rounded-xl border bg-card">
      <Table className="min-w-[540px] table-fixed">
        <TableHeader>
          <TableRow className="hover:bg-transparent">
            <TableHead className="sticky left-0 z-10 w-[124px] bg-card px-4 py-3 text-[13px] text-muted-foreground sm:w-[180px]">
              <span className="sr-only">Feature</span>
            </TableHead>
            {plans.map((plan) => (
              <TableHead
                key={plan.id}
                scope="col"
                className={cn('px-4 py-3 align-top', plan.recommended && 'bg-brand-soft/40')}
              >
                <span className="flex flex-col items-start gap-1.5">
                  <span className="text-[15px] font-extrabold">{plan.name}</span>
                  {plan.id === currentPlanId ? (
                    <Badge variant="success" className="rounded-full px-2.5">
                      Current plan
                    </Badge>
                  ) : (
                    plan.recommended && (
                      <Badge variant="info" className="rounded-full px-2.5">
                        Recommended
                      </Badge>
                    )
                  )}
                </span>
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        {matrix.map((group) => (
          <TableBody key={group.title}>
            <TableRow className="bg-muted hover:bg-muted">
              <TableHead
                scope="rowgroup"
                className="sticky left-0 z-10 h-9 bg-muted px-4 text-xs font-bold tracking-wide text-muted-foreground uppercase"
              >
                {group.title}
              </TableHead>
              <TableCell colSpan={plans.length} />
            </TableRow>
            {group.rows.map((row) => (
              <TableRow key={row.key} className="hover:bg-transparent">
                <TableHead
                  scope="row"
                  className="sticky left-0 z-10 bg-card px-4 py-3 text-[13px] font-semibold whitespace-normal"
                >
                  {row.label}
                </TableHead>
                {row.cells.map((cell, i) => (
                  <TableCell
                    key={plans[i].id}
                    className={cn(
                      'px-4 py-3 text-[13px]',
                      plans[i].recommended && 'bg-brand-soft/40',
                    )}
                  >
                    <Cell cell={cell} />
                  </TableCell>
                ))}
              </TableRow>
            ))}
          </TableBody>
        ))}
      </Table>
    </div>
  )

  if (expanded) {
    return (
      <section aria-labelledby={`${panelId}-title`} className="mt-6 flex flex-col gap-4">
        <h2 id={`${panelId}-title`} className="text-xl font-extrabold tracking-[-0.01em]">
          Compare all features
        </h2>
        {table}
      </section>
    )
  }

  return (
    <section className="flex flex-col gap-4">
      <Button
        variant="outline"
        className="self-start"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen((o) => !o)}
      >
        Compare all features
        <ChevronDownIcon
          className={cn('transition-transform', open && 'rotate-180')}
          aria-hidden="true"
        />
      </Button>
      {open && table}
    </section>
  )
}

function Cell({ cell }: { cell: MatrixCell }) {
  if (cell.kind === 'text') return <span className="font-semibold">{cell.text}</span>
  if (cell.kind === 'included') {
    return (
      <>
        <CheckIcon className="size-4 text-brand" aria-hidden="true" />
        <span className="sr-only">Included</span>
      </>
    )
  }
  return (
    <>
      <span aria-hidden="true" className="text-muted-foreground">
        —
      </span>
      <span className="sr-only">Not included</span>
    </>
  )
}
