import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { ExternalLinkIcon, ReceiptIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Separator } from '#/components/ui/separator'
import { Skeleton } from '#/components/ui/skeleton'
import { invoiceQueryOptions, invoicesQueryOptions } from '../../api/billing.queries'
import { INVOICE_STATUS, PROVIDER_LABEL } from '../../schemas/billing.schema'
import type { Invoice } from '../../types'
import { formatDate } from '../kit/format'
import { Modal } from '../kit/modal'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { EmptyState, ErrorState } from '../kit/states'
import { StatusBadge } from '../kit/status-badge'

export function Invoices({ workspaceId }: { workspaceId: string }) {
  const invoices = useInfiniteQuery(invoicesQueryOptions(workspaceId))
  const [open, setOpen] = useState<string>()
  const rows = invoices.data?.pages.flatMap((p) => p.invoices) ?? []

  return (
    <Section aria-labelledby="invoices-heading">
      <SectionHeader
        id="invoices-heading"
        title="Invoices"
        description="Every payment, newest first."
      />
      {invoices.isPending ? (
        <div
          className="flex flex-col gap-2"
          role="status"
          aria-busy="true"
          aria-label="Loading invoices"
        >
          {[0, 1, 2].map((i) => (
            <Skeleton key={i} className="h-14 w-full" />
          ))}
        </div>
      ) : !invoices.data ? (
        <ErrorState
          title="Couldn’t load invoices"
          error={invoices.error}
          onRetry={() => void invoices.refetch()}
        />
      ) : rows.length === 0 ? (
        <EmptyState icon={<ReceiptIcon className="size-5" />} title="No invoices yet">
          Invoices appear here after your first payment.
        </EmptyState>
      ) : (
        <>
          <ul className="flex flex-col divide-y rounded-lg border">
            {rows.map((inv) => (
              <li key={inv.id}>
                <button
                  type="button"
                  onClick={() => setOpen(inv.id)}
                  className="flex w-full flex-wrap items-center justify-between gap-3 px-3.5 py-3 text-left hover:bg-muted"
                >
                  <span className="flex min-w-0 flex-col gap-0.5">
                    <span className="text-sm font-bold">
                      {inv.number ?? 'Invoice'} · {formatDate(inv.paid_at ?? inv.created_at)}
                    </span>
                    <span className="text-[13px] text-muted-foreground">{period(inv)}</span>
                  </span>
                  <span className="flex items-center gap-3">
                    <span className="text-sm font-bold tabular-nums">
                      {formatMoney(inv.total_minor, inv.currency)}
                    </span>
                    <StatusBadge status={INVOICE_STATUS[inv.status]} />
                  </span>
                </button>
              </li>
            ))}
          </ul>
          {invoices.hasNextPage && (
            <Button
              variant="outline"
              className="self-center"
              disabled={invoices.isFetchingNextPage}
              onClick={() => void invoices.fetchNextPage()}
            >
              {invoices.isFetchingNextPage ? 'Loading…' : 'Show older invoices'}
            </Button>
          )}
          {invoices.isFetchNextPageError && (
            <ErrorState
              title="Couldn’t load older invoices"
              error={invoices.error}
              onRetry={() => void invoices.fetchNextPage()}
            />
          )}
        </>
      )}
      {open && (
        <InvoiceDialog
          workspaceId={workspaceId}
          invoiceId={open}
          onClose={() => setOpen(undefined)}
        />
      )}
    </Section>
  )
}

function period(inv: Invoice) {
  if (inv.period_start && inv.period_end) {
    return `${formatDate(inv.period_start)} – ${formatDate(inv.period_end)}`
  }
  return inv.installment_id
    ? 'Lifetime deal installment'
    : inv.order_id
      ? 'Lifetime deal'
      : 'Payment'
}

function InvoiceDialog({
  workspaceId,
  invoiceId,
  onClose,
}: {
  workspaceId: string
  invoiceId: string
  onClose: () => void
}) {
  const invoice = useQuery(invoiceQueryOptions(workspaceId, invoiceId))
  const inv = invoice.data
  const money = (n: number) => formatMoney(n, inv?.currency ?? 'USD')

  return (
    <Modal onClose={onClose} title={inv?.number ? `Invoice ${inv.number}` : 'Invoice'}>
      {invoice.isPending && (
        <div
          className="flex flex-col gap-2"
          role="status"
          aria-busy="true"
          aria-label="Loading invoice"
        >
          <Skeleton className="h-5 w-full" />
          <Skeleton className="h-5 w-3/4" />
          <Skeleton className="h-5 w-1/2" />
        </div>
      )}
      {invoice.isError && (
        <ErrorState
          title="Couldn’t load this invoice"
          error={invoice.error}
          onRetry={() => void invoice.refetch()}
        />
      )}
      {inv && (
        <div className="flex flex-col gap-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <span className="text-[13px] text-muted-foreground">
              {formatDate(inv.paid_at ?? inv.created_at)} · {PROVIDER_LABEL[inv.provider]}
            </span>
            <StatusBadge status={INVOICE_STATUS[inv.status]} />
          </div>
          {inv.bill_to && (
            <address className="flex flex-col text-sm not-italic">
              <span className="font-bold">{inv.bill_to.legal_name}</span>
              {[
                inv.bill_to.address_line1,
                inv.bill_to.address_line2,
                [inv.bill_to.postal_code, inv.bill_to.city].filter(Boolean).join(' '),
                inv.bill_to.state,
                inv.bill_to.country,
              ]
                .filter(Boolean)
                .map((l) => (
                  <span key={l}>{l}</span>
                ))}
              <span className="text-muted-foreground">{inv.bill_to.email}</span>
              {inv.bill_to.tax_id && (
                <span className="text-muted-foreground">
                  Tax ID: {inv.bill_to.tax_id}
                  {inv.bill_to.tax_id_type && ` (${inv.bill_to.tax_id_type})`}
                </span>
              )}
            </address>
          )}
          <dl className="flex flex-col gap-2 text-sm">
            <Row label="Subtotal" value={money(inv.subtotal_minor)} />
            {inv.discount_minor > 0 && (
              <Row label="Discount" value={`−${money(inv.discount_minor)}`} />
            )}
            {inv.tax_minor > 0 && <Row label="Tax" value={money(inv.tax_minor)} />}
            <Separator />
            <Row label="Total" value={money(inv.total_minor)} strong />
          </dl>
          {inv.receipt_url && (
            <Button asChild variant="outline" className="self-start">
              <a href={inv.receipt_url} target="_blank" rel="noreferrer noopener">
                Open receipt
                <ExternalLinkIcon aria-hidden="true" />
                <span className="sr-only">(opens in a new tab)</span>
              </a>
            </Button>
          )}
        </div>
      )}
    </Modal>
  )
}

function Row({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <div
      className={
        strong ? 'flex justify-between gap-3 font-extrabold' : 'flex justify-between gap-3'
      }
    >
      <dt className={strong ? undefined : 'text-muted-foreground'}>{label}</dt>
      <dd className="tabular-nums">{value}</dd>
    </div>
  )
}
