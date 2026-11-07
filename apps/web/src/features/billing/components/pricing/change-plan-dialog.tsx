import { useQuery } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { TriangleAlertIcon } from 'lucide-react'
import { toast } from 'sonner'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { useChangeSubscription } from '../../api/billing.mutations'
import {
  changePreviewQueryOptions,
  contactsUsageQueryOptions,
  monthlyUsageQueryOptions,
  numbersUsageQueryOptions,
  plansQueryOptions,
  resourcesQueryOptions,
  seatsUsageQueryOptions,
} from '../../api/billing.queries'
import type { ChangeQuote, Plan, Resource } from '../../types'
import { formatDate } from '../kit/format'
import { Modal } from '../kit/modal'
import { formatMoney } from '../kit/money'
import { ErrorState } from '../kit/states'

const MINUTE = 60 * 1000

/**
 * Whether the change waits for the period end (a downgrade, or yearly to monthly): its effective_at
 * is then in the future, while one that applies now has the time it was quoted.
 */
function startsLater(quote: ChangeQuote, quotedAt: number) {
  return Date.parse(quote.effective_at) > quotedAt + MINUTE
}

/** What the change does and when. */
function describe(quote: ChangeQuote, later: boolean) {
  if (later) {
    return `Your plan changes on ${formatDate(quote.effective_at)}, when the current period ends. You keep your current plan until then.`
  }
  const amount = formatMoney(Math.abs(quote.proration_minor), quote.currency)
  if (quote.proration_minor > 0) {
    return `The new plan starts now. We charge ${amount} today for the rest of this period.`
  }
  if (quote.proration_minor < 0) {
    return `The new plan starts now. ${amount} of unused time is credited to your next invoice.`
  }
  return 'The new plan starts now, with nothing extra to pay today.'
}

const count = new Intl.NumberFormat()
const units = (n: number, r: Resource) => `${count.format(n)} ${n === 1 ? r.unit : `${r.unit}s`}`

/** "You have 8 team seats; Starter allows 3.": each counted limit the workspace is already past. */
function overLimits(plan: Plan, resources: Resource[], used: Record<string, number | undefined>) {
  return resources.flatMap((r) => {
    const limit = plan.limits[r.key] ?? 0
    const n = used[r.key]
    if (!r.enforced || r.kind === 'access' || n === undefined || limit < 0 || n <= limit) return []
    const allows = limit === 0 ? 'doesn’t include any' : `allows ${count.format(limit)}`
    return [
      r.kind === 'monthly'
        ? `You’ve used ${units(n, r)} this month; ${plan.name} ${allows}${limit === 0 ? '' : ' a month'}.`
        : `You have ${units(n, r)}; ${plan.name} ${allows}.`,
    ]
  })
}

/** The workspace's current use of every counted resource, keyed like plan limits. */
function useCurrentUsage(workspaceId: string) {
  const seats = useQuery(seatsUsageQueryOptions(workspaceId))
  const numbers = useQuery(numbersUsageQueryOptions(workspaceId))
  const contacts = useQuery(contactsUsageQueryOptions(workspaceId))
  const monthly = useQuery(monthlyUsageQueryOptions(workspaceId))
  const used: Record<string, number | undefined> = {
    members: seats.data?.used,
    whatsapp_numbers: numbers.data,
    contacts: contacts.data,
  }
  for (const [key, m] of Object.entries(monthly.data?.resources ?? {})) used[key] = m.used
  return used
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
  const plans = useQuery(plansQueryOptions())
  const resources = useQuery(resourcesQueryOptions())
  const used = useCurrentUsage(workspaceId)
  const change = useChangeSubscription(workspaceId)
  const navigate = useNavigate()
  const deferred = !!preview.data && startsLater(preview.data, preview.dataUpdatedAt)
  const target = plans.data?.plans.find((p) => p.prices.some((pr) => pr.id === priceId))
  // A change at the period end lands in a later month, when monthly counts have started over.
  const checked = resources.data?.filter((r) => !deferred || r.kind !== 'monthly')
  const over = target && checked ? overLimits(target, checked, used) : []

  const confirm = () =>
    change.mutate(priceId, {
      onSuccess: () => {
        toast.success(deferred ? 'Plan change scheduled' : 'Plan changed')
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
      {preview.data && <p className="text-sm">{describe(preview.data, deferred)}</p>}
      {target && over.length > 0 && (
        <Alert variant="warning">
          <TriangleAlertIcon aria-hidden="true" />
          <AlertTitle>You’re over {target.name}’s limits</AlertTitle>
          <AlertDescription>
            <ul className="list-disc pl-4">
              {over.map((line) => (
                <li key={line}>{line}</li>
              ))}
            </ul>
            <p>
              Nothing is removed: what you have stays. You can’t add more until you’re under the
              limit.
            </p>
          </AlertDescription>
        </Alert>
      )}
      {change.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(change.error)}</AlertDescription>
        </Alert>
      )}
    </Modal>
  )
}
