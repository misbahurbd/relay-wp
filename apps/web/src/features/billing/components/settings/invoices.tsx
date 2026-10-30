import { useInfiniteQuery, useQuery } from '@tanstack/react-query'
import { DownloadIcon, ExternalLinkIcon, ReceiptIcon } from 'lucide-react'
import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Separator } from '#/components/ui/separator'
import { Skeleton } from '#/components/ui/skeleton'
import { invoiceQueryOptions, invoicesQueryOptions } from '../../api/billing.queries'
import { useBillingPreview } from '../../hooks/use-billing-preview'
import { INVOICE_STATUS, PROVIDER_LABEL } from '../../schemas/billing.schema'
import type { Invoice } from '../../types'
import { formatDate } from '../kit/format'
import { Modal } from '../kit/modal'
import { formatMoney } from '../kit/money'
import { PreviewBadge } from '../kit/preview-badge'
import { Section, SectionHeader } from '../kit/section'
import { EmptyState, ErrorState } from '../kit/states'
import { StatusBadge } from '../kit/status-badge'

export function Invoices({ workspaceId }: { workspaceId: string }) {
  const preview = useBillingPreview()
  const invoices = useInfiniteQuery(invoicesQueryOptions(workspaceId))
  const [open, setOpen] = useState<string>()
  const rows = invoices.data?.pages.flatMap((p) => p.invoices) ?? []

  return (
    <Section aria-labelledby="invoices-heading">
      <SectionHeader
        id="invoices-heading"
        title="Invoice history"
        action={
          preview && (
            <span className="inline-flex items-center gap-2">
              <PreviewBadge />
              <Button variant="outline" size="sm" disabled>
                Download all
                <span className="sr-only">(coming soon)</span>
              </Button>
            </span>
          )
        }
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
          <div className="-mx-5 overflow-x-auto px-5 sm:-mx-[22px] sm:px-[22px]">
            <table className="w-full min-w-[680px] text-sm">
              <caption className="sr-only">Invoices, newest first</caption>
              <thead className="text-left text-xs text-muted-foreground">
                <tr>
                  <th scope="col" className="py-2 pr-2 font-semibold">
                    Date
                  </th>
                  <th scope="col" className="px-2 py-2 font-semibold">
                    Invoice
                  </th>
                  <th scope="col" className="px-2 py-2 font-semibold">
                    Description
                  </th>
                  <th scope="col" className="px-2 py-2 text-right font-semibold">
                    Amount
                  </th>
                  <th scope="col" className="px-2 py-2 font-semibold">
                    Status
                  </th>
                  <th scope="col" className="py-2 pl-2 text-right font-semibold">
                    <span className="sr-only">Receipt</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {rows.map((inv) => (
                  <tr key={inv.id} className="border-t hover:bg-muted/50">
                    <td className="py-2.5 pr-2 whitespace-nowrap">
                      {formatDate(inv.paid_at ?? inv.created_at)}
                    </td>
                    <td className="px-2 py-2.5 whitespace-nowrap">
                      <button
                        type="button"
                        onClick={() => setOpen(inv.id)}
                        className="text-muted-foreground underline-offset-4 hover:text-foreground hover:underline"
                      >
                        {inv.number ?? 'View invoice'}
                      </button>
                    </td>
                    <td className="px-2 py-2.5 font-semibold">{period(inv)}</td>
                    <td className="px-2 py-2.5 text-right font-bold tabular-nums">
                      {formatMoney(inv.total_minor, inv.currency)}
                    </td>
                    <td className="px-2 py-2.5">
                      <StatusBadge status={INVOICE_STATUS[inv.status]} />
                    </td>
                    <td className="py-1.5 pl-2 text-right">
                      {inv.receipt_url ? (
                        <Button asChild variant="ghost" size="icon">
                          <a href={inv.receipt_url} target="_blank" rel="noreferrer noopener">
                            <DownloadIcon aria-hidden="true" />
                            <span className="sr-only">
                              Receipt for {inv.number ?? formatDate(inv.created_at)} (opens in a new
                              tab)
                            </span>
                          </a>
                        </Button>
                      ) : (
                        <span className="sr-only">No receipt</span>
                      )}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
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
    const short = (iso: string) =>
      new Date(iso).toLocaleDateString(undefined, { month: 'short', day: 'numeric' })
    return `Plan · ${short(inv.period_start)} – ${short(inv.period_end)}`
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
