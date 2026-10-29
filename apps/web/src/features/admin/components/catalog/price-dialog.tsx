import { useState } from 'react'
import { toast } from 'sonner'
import { useCreatePrice } from '../../api/admin.api'
import type { Plan } from '../../types'
import { FormDialog } from '../kit/form-dialog'
import { toMinor } from '../kit/money'
import { SelectField } from '../kit/select-field'
import { TextField } from '../kit/text-field'

type Kind = 'month' | 'year' | 'one_time'

export function PriceDialog({ plan, onClose }: { plan: Plan; onClose: () => void }) {
  const create = useCreatePrice()
  const [kind, setKind] = useState<Kind>('month')
  const [currency, setCurrency] = useState('USD')
  const [amount, setAmount] = useState('')

  const submit = () =>
    create.mutate(
      {
        plan_id: plan.id,
        billing_type: kind === 'one_time' ? 'one_time' : 'recurring',
        interval: kind === 'one_time' ? undefined : kind,
        currency: currency.trim().toUpperCase(),
        amount_minor: toMinor(amount),
      },
      {
        onSuccess: () => {
          toast.success('Price added. Link it to a provider to put it on sale.')
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title={`Add a price to ${plan.name} v${plan.version}`}
      description="Prices are never edited: deactivate one and add its replacement."
      submitLabel="Add price"
      pending={create.isPending}
      error={create.error}
      onSubmit={submit}
      onClose={onClose}
    >
      <SelectField
        id="price-kind"
        label="Billing"
        value={kind}
        onChange={setKind}
        options={[
          { value: 'month', label: 'Monthly subscription' },
          { value: 'year', label: 'Yearly subscription' },
          { value: 'one_time', label: 'One time (lifetime deal)' },
        ]}
      />
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField
          id="price-currency"
          label="Currency"
          value={currency}
          onChange={setCurrency}
          maxLength={3}
          required
        />
        <TextField
          id="price-amount"
          label="Amount"
          inputMode="decimal"
          value={amount}
          onChange={setAmount}
          placeholder="29.00"
          hint="In the currency's main unit"
          required
        />
      </div>
    </FormDialog>
  )
}
