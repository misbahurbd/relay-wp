import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Label } from '#/components/ui/label'
import { RadioGroup, RadioGroupItem } from '#/components/ui/radio-group'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useCancelSubscription } from '../../api/billing.mutations'
import { formatDate } from '../kit/format'
import { Modal } from '../kit/modal'

/** Ask when a subscription should end: at the period end (default) or right away. */
export function CancelDialog({
  workspaceId,
  periodEnd,
  onClose,
}: {
  workspaceId: string
  periodEnd: string | null
  onClose: () => void
}) {
  const cancel = useCancelSubscription(workspaceId)
  const [when, setWhen] = useState<'end' | 'now'>('end')
  const options = [
    {
      value: 'end' as const,
      title: 'At the end of the period',
      body: periodEnd
        ? `Keep the plan until ${formatDate(periodEnd)}, then move to the free plan.`
        : 'Keep the plan until the period ends, then move to the free plan.',
    },
    {
      value: 'now' as const,
      title: 'Right away',
      body: 'Move to the free plan now. Unused time isn’t refunded.',
    },
  ]

  return (
    <Modal
      onClose={onClose}
      title="Cancel subscription?"
      description="Your workspace moves to the free plan’s limits when the subscription ends. You can subscribe again at any time."
      footer={
        <>
          <Button variant="outline" onClick={onClose}>
            Keep subscription
          </Button>
          <Button
            variant="destructive"
            disabled={cancel.isPending}
            onClick={() =>
              cancel.mutate(when === 'end', {
                onSuccess: () => {
                  toast.success(when === 'end' ? 'Subscription will end' : 'Subscription canceled')
                  onClose()
                },
              })
            }
          >
            {cancel.isPending ? 'Canceling…' : 'Cancel subscription'}
          </Button>
        </>
      }
    >
      <fieldset>
        <legend className="mb-2 text-sm font-medium">When should it end?</legend>
        <RadioGroup
          value={when}
          onValueChange={(v) => setWhen(v as 'end' | 'now')}
          className="gap-2"
        >
          {options.map((o) => (
            <Label
              key={o.value}
              htmlFor={`cancel-${o.value}`}
              className={cn(
                'flex cursor-pointer items-start gap-3 rounded-lg border border-input p-3 font-normal leading-snug',
                when === o.value && 'border-primary bg-muted',
              )}
            >
              <RadioGroupItem id={`cancel-${o.value}`} value={o.value} className="mt-0.5" />
              <span className="flex flex-col gap-0.5">
                <span className="font-bold">{o.title}</span>
                <span className="text-[13px] text-muted-foreground">{o.body}</span>
              </span>
            </Label>
          ))}
        </RadioGroup>
      </fieldset>
      {cancel.isError && (
        <Alert variant="danger">
          <AlertDescription>{errorMessage(cancel.error)}</AlertDescription>
        </Alert>
      )}
    </Modal>
  )
}
