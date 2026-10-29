import { useState } from 'react'
import { toast } from 'sonner'
import { useCreateDiscount } from '../../api/admin.api'
import type { Plan } from '../../types'
import { CheckField } from '../kit/check-field'
import { FormDialog } from '../kit/form-dialog'
import { toMinor } from '../kit/money'
import { SelectField } from '../kit/select-field'
import { TextField } from '../kit/text-field'

type Kind = 'percent' | 'amount'
type Duration = 'once' | 'repeating' | 'forever'
type AppliesTo = 'all' | 'recurring' | 'one_time'

const num = (s: string) => (s.trim() === '' ? undefined : Number(s))
/** A local date-time input value as an ISO instant, or undefined when empty. */
const instant = (s: string) => (s ? new Date(s).toISOString() : undefined)

export function DiscountDialog({ plans, onClose }: { plans: Plan[]; onClose: () => void }) {
  const create = useCreateDiscount()
  const [code, setCode] = useState('')
  const [name, setName] = useState('')
  const [kind, setKind] = useState<Kind>('percent')
  const [percent, setPercent] = useState('20')
  const [amount, setAmount] = useState('')
  const [currency, setCurrency] = useState('USD')
  const [duration, setDuration] = useState<Duration>('once')
  const [months, setMonths] = useState('3')
  const [appliesTo, setAppliesTo] = useState<AppliesTo>('all')
  const [planId, setPlanId] = useState('')
  const [installments, setInstallments] = useState(true)
  const [maxRedemptions, setMaxRedemptions] = useState('')
  const [perWorkspace, setPerWorkspace] = useState('1')
  const [startsAt, setStartsAt] = useState('')
  const [expiresAt, setExpiresAt] = useState('')

  const submit = () =>
    create.mutate(
      {
        code: code.trim().toUpperCase(),
        name: name.trim() || undefined,
        kind,
        percent_bps: kind === 'percent' ? Math.round(Number(percent) * 100) : undefined,
        amount_off_minor: kind === 'amount' ? toMinor(amount) : undefined,
        currency: kind === 'amount' ? currency.trim().toUpperCase() : undefined,
        duration,
        duration_months: duration === 'repeating' ? num(months) : undefined,
        applies_to: appliesTo,
        plan_id: planId || undefined,
        allow_installments: installments,
        max_redemptions: num(maxRedemptions),
        per_workspace_limit: num(perWorkspace) ?? 1,
        starts_at: instant(startsAt),
        expires_at: instant(expiresAt),
      },
      {
        onSuccess: () => {
          toast.success(`Added ${code.trim().toUpperCase()}`)
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title="Add a discount code"
      description="Codes can't be edited. Deactivate one and add another to change it."
      submitLabel="Add code"
      pending={create.isPending}
      error={create.error}
      onSubmit={submit}
      onClose={onClose}
    >
      <div className="grid gap-4 sm:grid-cols-2">
        <TextField
          id="d-code"
          label="Code"
          value={code}
          onChange={setCode}
          placeholder="LAUNCH20"
          required
        />
        <TextField id="d-name" label="Name (optional)" value={name} onChange={setName} />
        <SelectField
          id="d-kind"
          label="Takes off"
          value={kind}
          onChange={setKind}
          options={[
            { value: 'percent', label: 'A percentage' },
            { value: 'amount', label: 'A fixed amount' },
          ]}
        />
        {kind === 'percent' ? (
          <TextField
            id="d-percent"
            label="Percent"
            inputMode="decimal"
            value={percent}
            onChange={setPercent}
            hint="0.01 to 100"
          />
        ) : (
          <div className="grid grid-cols-[1fr_80px] gap-2">
            <TextField
              id="d-amount"
              label="Amount"
              inputMode="decimal"
              value={amount}
              onChange={setAmount}
            />
            <TextField
              id="d-currency"
              label="Currency"
              value={currency}
              onChange={setCurrency}
              maxLength={3}
            />
          </div>
        )}
        <SelectField
          id="d-duration"
          label="On subscriptions, lasts"
          value={duration}
          onChange={setDuration}
          options={[
            { value: 'once', label: 'The first payment' },
            { value: 'repeating', label: 'Some months' },
            { value: 'forever', label: 'Forever' },
          ]}
          hint="One-time payments always apply it once"
        />
        {duration === 'repeating' && (
          <TextField
            id="d-months"
            label="Months"
            type="number"
            min={1}
            max={36}
            value={months}
            onChange={setMonths}
          />
        )}
        <SelectField
          id="d-applies"
          label="Applies to"
          value={appliesTo}
          onChange={setAppliesTo}
          options={[
            { value: 'all', label: 'Every price' },
            { value: 'recurring', label: 'Subscriptions only' },
            { value: 'one_time', label: 'Lifetime deals only' },
          ]}
        />
        <SelectField
          id="d-plan"
          label="Plan"
          value={planId}
          onChange={setPlanId}
          options={[
            { value: '', label: 'Any plan' },
            ...plans.map((p) => ({ value: p.id, label: `${p.name} (${p.code} v${p.version})` })),
          ]}
        />
        <TextField
          id="d-max"
          label="Total uses (optional)"
          type="number"
          min={1}
          value={maxRedemptions}
          onChange={setMaxRedemptions}
          hint="Empty: no cap"
        />
        <TextField
          id="d-per-ws"
          label="Uses per workspace"
          type="number"
          min={1}
          value={perWorkspace}
          onChange={setPerWorkspace}
        />
        <TextField
          id="d-starts"
          label="Starts (optional)"
          type="datetime-local"
          value={startsAt}
          onChange={setStartsAt}
          hint="Empty: now"
        />
        <TextField
          id="d-expires"
          label="Expires (optional)"
          type="datetime-local"
          value={expiresAt}
          onChange={setExpiresAt}
        />
      </div>
      <CheckField
        id="d-installments"
        label="Can be used with installment plans"
        checked={installments}
        onChange={setInstallments}
      />
    </FormDialog>
  )
}
