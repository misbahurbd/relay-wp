import { useNavigate } from '@tanstack/react-router'
import { useEffect } from 'react'
import { Skeleton } from '#/components/ui/skeleton'
import { usePayInstallment, useResumeOrder } from '../../api/billing.mutations'
import { useIdempotencyKey } from '../../hooks/use-idempotency-key'
import { PaymentStep } from '../checkout/payment-step'
import { Modal } from '../kit/modal'
import { formatMoney } from '../kit/money'
import { ErrorState } from '../kit/states'

export type PayTarget =
  | {
      kind: 'installment'
      orderId: string
      installmentId: string
      seq: number
      amount: number
      currency: string
    }
  | { kind: 'resume'; orderId: string; amount: number; currency: string }

/** Starts a checkout for one installment, or the whole balance of a defaulted deal, and pays it. */
export function PayDialog({
  workspaceId,
  target,
  onClose,
}: {
  workspaceId: string
  target: PayTarget
  onClose: () => void
}) {
  const navigate = useNavigate()
  const installment = usePayInstallment(workspaceId)
  const resume = useResumeOrder(workspaceId)
  const mutation = target.kind === 'installment' ? installment : resume
  const scope = target.kind === 'installment' ? target.installmentId : `resume:${target.orderId}`
  const { key } = useIdempotencyKey(scope)

  const start = () => {
    if (target.kind === 'installment') {
      installment.mutate({
        idempotencyKey: key,
        orderId: target.orderId,
        installmentId: target.installmentId,
      })
    } else {
      resume.mutate({ idempotencyKey: key, orderId: target.orderId })
    }
  }

  // Start once when the dialog opens; the same key makes a retry return the same checkout.
  // biome-ignore lint/correctness/useExhaustiveDependencies: run on open only
  useEffect(start, [])

  const amount = formatMoney(target.amount, target.currency)
  return (
    <Modal
      onClose={onClose}
      title={
        target.kind === 'installment'
          ? `Pay installment ${target.seq}`
          : 'Pay the remaining balance'
      }
      description={
        target.kind === 'installment'
          ? `${amount} now. Paying restores full access if it was suspended.`
          : `${amount} now restores your lifetime plan.`
      }
    >
      {mutation.isPending && (
        <div
          className="flex flex-col gap-2"
          role="status"
          aria-busy="true"
          aria-label="Starting payment"
        >
          <Skeleton className="h-11 w-full" />
          <Skeleton className="h-11 w-full" />
        </div>
      )}
      {mutation.isError && (
        <ErrorState title="Couldn’t start the payment" error={mutation.error} onRetry={start} />
      )}
      {mutation.data && (
        <PaymentStep
          started={mutation.data}
          payLabel={`Pay ${formatMoney(mutation.data.session.amount_minor, mutation.data.session.currency)}`}
          onPaid={(session) => {
            onClose()
            void navigate({ to: '/billing/checkout', search: { session } })
          }}
        />
      )}
    </Modal>
  )
}
