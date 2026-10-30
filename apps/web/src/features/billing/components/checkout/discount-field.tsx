import { CircleCheckIcon, XIcon } from 'lucide-react'
import { useId, useState } from 'react'
import { describedBy, FormField } from '#/components/form'
import { Button } from '#/components/ui/button'
import { Input } from '#/components/ui/input'
import { errorMessage } from '#/lib/api'
import { usePreviewDiscount } from '../../api/billing.mutations'
import { discountCodeSchema } from '../../schemas/billing.schema'
import type { DiscountPreview } from '../../types'
import { formatMoney } from '../kit/money'

function durationText(d: DiscountPreview, oneTime: boolean) {
  // On a lifetime deal the discount comes off the order total (Q18).
  if (oneTime) return 'off the total'
  if (d.duration === 'forever') return 'on every payment'
  if (d.duration === 'repeating' && d.duration_months) {
    return `for the first ${d.duration_months} month${d.duration_months === 1 ? '' : 's'}`
  }
  return 'on this payment'
}

/**
 * A discount code, checked with the API before checkout. The applied preview goes up to the
 * summary; problems show below the field.
 */
export function DiscountField({
  workspaceId,
  priceId,
  offerId,
  oneTime,
  applied,
  onApplied,
}: {
  workspaceId: string
  priceId: string
  offerId?: string
  oneTime: boolean
  applied?: DiscountPreview
  onApplied: (preview: DiscountPreview | undefined) => void
}) {
  const id = useId()
  const [code, setCode] = useState('')
  const [problem, setProblem] = useState<string>()
  const preview = usePreviewDiscount(workspaceId)

  const apply = () => {
    const parsed = discountCodeSchema.safeParse(code)
    if (!parsed.success) {
      setProblem(parsed.error.issues[0]?.message)
      return
    }
    setProblem(undefined)
    preview.mutate(
      { code: parsed.data, priceId, offerId },
      {
        onSuccess: onApplied,
        onError: (e) =>
          setProblem(errorMessage(e, 'We couldn’t check this code. Please try again.')),
      },
    )
  }

  if (applied) {
    return (
      <div
        className="flex items-start justify-between gap-3 rounded-lg bg-brand-soft p-3"
        role="status"
      >
        <div className="flex items-start gap-2">
          <CircleCheckIcon className="mt-0.5 size-4 flex-none text-brand-text" aria-hidden="true" />
          <p className="text-sm">
            <span className="font-bold">{applied.code}</span> saves{' '}
            {formatMoney(applied.discount_minor, applied.currency)} {durationText(applied, oneTime)}
            .
          </p>
        </div>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`Remove discount ${applied.code}`}
          onClick={() => {
            onApplied(undefined)
            setCode('')
          }}
        >
          <XIcon aria-hidden="true" />
        </Button>
      </div>
    )
  }

  return (
    <form
      noValidate
      onSubmit={(e) => {
        e.preventDefault()
        apply()
      }}
    >
      <FormField id={id} label="Discount code (optional)" error={problem}>
        <div className="flex gap-2">
          <Input
            id={id}
            value={code}
            autoComplete="off"
            autoCapitalize="characters"
            spellCheck={false}
            placeholder="e.g. LAUNCH20"
            onChange={(e) => {
              setCode(e.target.value)
              if (problem) setProblem(undefined)
            }}
            aria-invalid={!!problem}
            aria-describedby={describedBy(id, { error: problem })}
          />
          <Button
            type="submit"
            variant="outline"
            className="text-foreground"
            disabled={preview.isPending}
          >
            {preview.isPending ? 'Checking…' : 'Apply'}
          </Button>
        </div>
      </FormField>
    </form>
  )
}
