import { useState } from 'react'
import { useCloseDispute } from '../../api/admin.api'
import type { DisputedInvoice } from '../../types'
import { FormDialog } from '../kit/form-dialog'
import { formatMoney } from '../kit/money'
import { SelectField } from '../kit/select-field'
import { TextField } from '../kit/text-field'

type Outcome = 'won' | 'lost'

const OUTCOMES: { value: Outcome; label: string }[] = [
  { value: 'won', label: 'Won: the money stays with us' },
  { value: 'lost', label: 'Lost: the money went back to the customer' },
]

/** Records how a dispute ended, for providers that never say (Creem). */
export function CloseDisputeDialog({
  invoice,
  onClose,
}: {
  invoice: DisputedInvoice
  onClose: () => void
}) {
  const close = useCloseDispute()
  const [outcome, setOutcome] = useState<Outcome>('won')
  const [reason, setReason] = useState('')

  return (
    <FormDialog
      title="Close dispute"
      description={`${invoice.number || 'Invoice'} · ${invoice.workspace_name} · ${formatMoney(invoice.total_minor, invoice.currency)}`}
      submitLabel={outcome === 'won' ? 'Mark won' : 'Mark lost and stop billing'}
      pending={close.isPending}
      disabled={reason.trim() === ''}
      error={close.error}
      onSubmit={() =>
        close.mutate({ invoiceId: invoice.id, outcome, reason }, { onSuccess: onClose })
      }
      onClose={onClose}
    >
      <SelectField
        id="dispute-outcome"
        label="Outcome"
        value={outcome}
        options={OUTCOMES}
        onChange={setOutcome}
        hint={
          outcome === 'won'
            ? 'The invoice goes back to paid.'
            : 'Final. Relay cancels the subscription this invoice paid for, at the provider too, or stops the lifetime deal’s installments, and suspends the plan. A lifetime deal’s plan can be restored later; a canceled subscription’s can’t, so the customer subscribes again or you grant a plan by hand.'
        }
      />
      <TextField
        id="dispute-reason"
        label="How it was settled"
        hint="Kept on record with your account."
        value={reason}
        maxLength={300}
        onChange={setReason}
      />
    </FormDialog>
  )
}
