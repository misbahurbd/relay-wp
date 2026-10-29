import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { useChangeSubscription } from '../../api/billing.mutations'
import { changePreviewQueryOptions } from '../../api/billing.queries'
import type { ChangeQuote } from '../../types'
import { formatDate } from '../kit/format'
import { Modal } from '../kit/modal'
import { formatMoney } from '../kit/money'
import { ErrorState } from '../kit/states'

function describe(quote: ChangeQuote) {
  const amount = formatMoney(Math.abs(quote.proration_minor), quote.currency)
  if (quote.kind === 'downgrade') {
    return `Your plan changes on ${formatDate(quote.effective_at)}, when the current period ends. You keep your current plan until then.`
  }
  if (quote.proration_minor > 0) {
    return `The new plan starts now. We charge ${amount} today for the rest of this period.`
  }
  if (quote.proration_minor < 0) {
    return `The new plan starts now. ${amount} of unused time is credited to your next invoice.`
  }
  return 'The new plan starts now, with nothing extra to pay today.'
}

/** Preview, then confirm, a switch between recurring prices. */
export function ChangePlanDialog({
  workspaceId,
  priceId,
  onClose,
}: {
  workspaceId: string
  priceId: string
  onClose: () => void
}) {
  const preview = useQuery(changePreviewQueryOptions(workspaceId, priceId))
  const change = useChangeSubscription(workspaceId)
  const navigate = useNavigate()

  const confirm = () =>
    change.mutate(priceId, {
      onSuccess: () => {
        toast.success(preview.data?.kind === 'downgrade' ? 'Plan change scheduled' : 'Plan changed')
        onClose()
        void navigate({ to: '/billing' })
      },
    })

  return (
    <Modal
      onClose={onClose}
      title="Change plan"
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Keep current plan
          </Button>
          <Button onClick={confirm} disabled={!preview.data || change.isPending}>
            {change.isPending ? 'Changing…' : 'Confirm change'}
          </Button>
        </>
      }
    >
      {preview.isPending && (
        <div
          className="flex flex-col gap-2"
          role="status"
          aria-busy="true"
          aria-label="Loading the change"
        >
          <Skeleton className="h-5 w-full" />
          <Skeleton className="h-5 w-2/3" />
        </div>
      )}
      {preview.isError && (
        <ErrorState
          title="Couldn’t preview this change"
          error={preview.error}
          onRetry={() => void preview.refetch()}
        />
      )}
      {preview.data && <p className="text-sm">{describe(preview.data)}</p>}
      {change.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(change.error)}</AlertDescription>
        </Alert>
      )}
    </Modal>
  )
}
