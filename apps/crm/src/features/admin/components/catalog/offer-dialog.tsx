import { useState } from 'react'
import { toast } from 'sonner'
import { useCreateInstallmentOffer } from '../../api/admin.api'
import type { Price } from '../../types'
import { FormDialog } from '../kit/form-dialog'
import { formatMoney, toMinor } from '../kit/money'
import { TextField } from '../kit/text-field'

export function OfferDialog({ price, onClose }: { price: Price; onClose: () => void }) {
  const create = useCreateInstallmentOffer()
  const [count, setCount] = useState('3')
  const [total, setTotal] = useState(String(price.amount_minor / 100))
  const [graceDays, setGraceDays] = useState('7')
  const [maxMissed, setMaxMissed] = useState('2')

  const submit = () =>
    create.mutate(
      {
        plan_price_id: price.id,
        count: Number(count),
        total_minor: toMinor(total),
        grace_days: Number(graceDays),
        max_missed: Number(maxMissed),
      },
      {
        onSuccess: () => {
          toast.success('Installment offer added. Link it to a provider to sell it.')
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title="Add an installment offer"
      description={`Splits the ${formatMoney(price.amount_minor, price.currency)} one-time price into monthly payments. The total may include a markup.`}
      submitLabel="Add offer"
      pending={create.isPending}
      error={create.error}
      onSubmit={submit}
      onClose={onClose}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField
          id="offer-count"
          label="Payments"
          type="number"
          min={2}
          max={24}
          value={count}
          onChange={setCount}
        />
        <TextField
          id="offer-total"
          label={`Total (${price.currency})`}
          inputMode="decimal"
          value={total}
          onChange={setTotal}
          hint="The last payment absorbs rounding"
        />
        <TextField
          id="offer-grace"
          label="Grace days"
          type="number"
          min={0}
          max={60}
          value={graceDays}
          onChange={setGraceDays}
          hint="Before a missed payment suspends access"
        />
        <TextField
          id="offer-missed"
          label="Missed payments allowed"
          type="number"
          min={1}
          max={24}
          value={maxMissed}
          onChange={setMaxMissed}
          hint="Then the order defaults"
        />
      </div>
    </FormDialog>
  )
}
