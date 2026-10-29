import { CreditCardIcon } from 'lucide-react'
import { useState } from 'react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Progress } from '#/components/ui/progress'
import { INSTALLMENT_STATUS, ORDER_STATUS, PAYABLE } from '../../schemas/billing.schema'
import type { Order } from '../../types'
import { formatDate } from '../kit/format'
import { formatMoney } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'
import { NoPayPermission } from '../kit/states'
import { StatusBadge } from '../kit/status-badge'
import { PayDialog, type PayTarget } from './pay-dialog'

/** The lifetime deal: what is paid, what is owed, and each installment with Pay now. */
export function LtdOrder({
  workspaceId,
  order,
  canPay,
}: {
  workspaceId: string
  order: Order
  canPay: boolean
}) {
  const [paying, setPaying] = useState<PayTarget>()
  const money = (n: number) => formatMoney(n, order.currency)
  const paidShare =
    order.total_minor > 0
      ? Math.round(((order.total_minor - order.balance_minor) * 100) / order.total_minor)
      : 100

  return (
    <Section aria-labelledby="ltd-heading">
      <SectionHeader
        id="ltd-heading"
        title="Lifetime deal"
        description={`${money(order.total_minor)} in total`}
        action={<StatusBadge status={ORDER_STATUS[order.status]} />}
      />

      {order.status === 'defaulted' && (
        <Alert variant="danger">
          <AlertTitle>Payments stopped</AlertTitle>
          <AlertDescription>
            <p>
              Too many installments were missed
              {order.defaulted_at && ` (since ${formatDate(order.defaulted_at)})`}, so the lifetime
              plan is paused. What you paid is kept: pay the remaining {money(order.balance_minor)}{' '}
              to restore it.
            </p>
            {canPay ? (
              <Button
                size="sm"
                className="mt-2"
                onClick={() =>
                  setPaying({
                    kind: 'resume',
                    orderId: order.id,
                    amount: order.balance_minor,
                    currency: order.currency,
                  })
                }
              >
                Pay {money(order.balance_minor)}
              </Button>
            ) : (
              <NoPayPermission />
            )}
          </AlertDescription>
        </Alert>
      )}

      <div className="flex flex-col gap-2">
        <div className="flex flex-wrap justify-between gap-2 text-sm">
          <span>
            <span className="font-bold">{money(order.total_minor - order.balance_minor)}</span> paid
          </span>
          <span className="text-muted-foreground">{money(order.balance_minor)} left</span>
        </div>
        <Progress value={paidShare} aria-label={`${paidShare}% paid`} />
        {(order.discount_minor > 0 || order.credit_minor > 0) && (
          <p className="text-[13px] text-muted-foreground">
            Includes{' '}
            {[
              order.discount_minor > 0 && `${money(order.discount_minor)} discount`,
              order.credit_minor > 0 && `${money(order.credit_minor)} credit`,
            ]
              .filter(Boolean)
              .join(' and ')}
            .
          </p>
        )}
      </div>

      {order.installments.length === 0 ? (
        <p className="text-sm text-muted-foreground">
          {order.paid_in_full_at
            ? `Paid in full on ${formatDate(order.paid_in_full_at)}.`
            : 'Paid in one payment.'}
        </p>
      ) : (
        <ol className="flex flex-col divide-y rounded-lg border">
          {order.installments.map((inst) => {
            const payable = PAYABLE.includes(inst.status) && order.status !== 'defaulted'
            return (
              <li
                key={inst.id}
                className="flex flex-wrap items-center justify-between gap-3 px-3.5 py-3"
              >
                <div className="flex min-w-0 flex-col gap-0.5">
                  <span className="text-sm font-bold">
                    Payment {inst.seq} · {money(inst.amount_minor)}
                  </span>
                  <span className="text-[13px] text-muted-foreground">
                    {inst.paid_at
                      ? `Paid ${formatDate(inst.paid_at)}`
                      : `Due ${formatDate(inst.due_at)}`}
                  </span>
                </div>
                <div className="flex items-center gap-2">
                  <StatusBadge status={INSTALLMENT_STATUS[inst.status]} />
                  {payable && canPay && (
                    <Button
                      size="sm"
                      onClick={() =>
                        setPaying({
                          kind: 'installment',
                          orderId: order.id,
                          installmentId: inst.id,
                          seq: inst.seq,
                          amount: inst.amount_minor,
                          currency: order.currency,
                        })
                      }
                    >
                      <CreditCardIcon aria-hidden="true" />
                      Pay now
                    </Button>
                  )}
                </div>
              </li>
            )
          })}
        </ol>
      )}
      {!canPay && order.installments.some((i) => PAYABLE.includes(i.status)) && (
        <NoPayPermission>
          A payment is due. Only the owner or members with the “Pay and change plan” permission can
          pay it.
        </NoPayPermission>
      )}

      {paying && (
        <PayDialog workspaceId={workspaceId} target={paying} onClose={() => setPaying(undefined)} />
      )}
    </Section>
  )
}
