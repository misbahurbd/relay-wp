import { CheckIcon } from 'lucide-react'
import type { MatrixCell, MatrixGroup } from '../kit/plan-highlights'

/** This plan's column of the pricing page's comparison table, as customers will read it. */
export function PlanColumnPreview({ matrix, index }: { matrix: MatrixGroup[]; index: number }) {
  if (matrix.length === 0) return null
  return (
    <section aria-label="Comparison table preview" className="flex flex-col gap-2">
      <p className="text-xs font-bold tracking-wide text-muted-foreground uppercase">
        In the comparison table
      </p>
      <div className="rounded-xl border bg-card px-4 py-1 shadow-card">
        {matrix.map((group) => (
          <div key={group.title}>
            <p className="pt-2 pb-1 text-[11px] font-bold tracking-wide text-muted-foreground uppercase">
              {group.title}
            </p>
            <ul className="flex flex-col divide-y">
              {group.rows.map((row) => (
                <li
                  key={row.key}
                  className="flex items-center justify-between gap-3 py-2 text-[13px]"
                >
                  <span className="min-w-0 text-muted-foreground">{row.label}</span>
                  <span className="flex-none font-semibold">
                    <Cell cell={row.cells[index]} />
                  </span>
                </li>
              ))}
            </ul>
          </div>
        ))}
      </div>
    </section>
  )
}

function Cell({ cell }: { cell: MatrixCell }) {
  if (cell.kind === 'text') return cell.text
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
