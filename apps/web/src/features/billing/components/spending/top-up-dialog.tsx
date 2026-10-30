import { CreditCardIcon } from 'lucide-react'
import { useId, useState } from 'react'
import { Button } from '#/components/ui/button'
import { ToggleGroup, ToggleGroupItem } from '#/components/ui/toggle-group'
import type { Wallet, WalletOutlook } from '../../types'
import { Modal } from '../kit/modal'
import { formatMoney } from '../kit/money'
import { minorDigits, parseMajor } from './amounts'

const OTHER = 'other'

/**
 * Top up, as a preview: amounts and the summary work, but paying is disabled until wallets exist.
 * Nothing here takes or sends payment details.
 */
export function TopUpDialog({
  open,
  onClose,
  wallet,
  outlook,
}: {
  open: boolean
  onClose: () => void
  wallet: Wallet
  outlook: WalletOutlook
}) {
  const id = useId()
  const options = outlook.top_up_options_minor
  const [choice, setChoice] = useState(String(options[Math.min(1, options.length - 1)] ?? OTHER))
  const [custom, setCustom] = useState('')
  const money = (minor: number) => formatMoney(minor, wallet.currency)

  const amount =
    choice === OTHER ? (parseMajor(custom, wallet.currency) ?? 0) : Number.parseInt(choice, 10)
  const tooSmall = amount < outlook.min_top_up_minor
  const { card } = outlook

  return (
    <Modal
      open={open}
      onClose={onClose}
      title="Top up wallet"
      description="Add money for Meta’s message charges."
      footer={
        <>
          <Button type="button" variant="outline" onClick={onClose}>
            Cancel
          </Button>
          <Button type="button" disabled aria-describedby={`${id}-soon`}>
            Pay {money(tooSmall ? 0 : amount)}
            <span className="text-xs font-semibold">Coming soon</span>
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-2">
        <span id={`${id}-amount-label`} className="text-[13px] font-semibold">
          Amount
        </span>
        <ToggleGroup
          type="single"
          variant="outline"
          value={choice}
          onValueChange={(v) => v && setChoice(v)}
          aria-labelledby={`${id}-amount-label`}
          className="grid w-full grid-cols-2 sm:grid-cols-4"
        >
          {options.map((o) => (
            <ToggleGroupItem key={o} value={String(o)} className="h-11 font-bold">
              {money(o)}
            </ToggleGroupItem>
          ))}
          <ToggleGroupItem value={OTHER} className="h-11 font-bold">
            Other
          </ToggleGroupItem>
        </ToggleGroup>
      </div>

      {choice === OTHER && (
        <div className="flex flex-col gap-1.5">
          <label htmlFor={`${id}-custom`} className="text-[13px] font-semibold">
            Custom amount
          </label>
          <div className="flex h-11 items-center gap-1 rounded-lg border border-input bg-background px-3 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring">
            <span aria-hidden="true" className="font-bold text-muted-foreground">
              $
            </span>
            <input
              id={`${id}-custom`}
              type="number"
              inputMode="decimal"
              min={outlook.min_top_up_minor / 10 ** minorDigits(wallet.currency)}
              step={10}
              placeholder="75"
              value={custom}
              onChange={(e) => setCustom(e.target.value)}
              aria-describedby={`${id}-min`}
              aria-invalid={custom !== '' && tooSmall}
              className="min-w-0 flex-1 bg-transparent font-bold tabular-nums outline-none"
            />
          </div>
          <p
            id={`${id}-min`}
            className={
              custom !== '' && tooSmall
                ? 'text-xs font-semibold text-destructive'
                : 'text-xs text-muted-foreground'
            }
          >
            The smallest top up is {money(outlook.min_top_up_minor)}.
          </p>
        </div>
      )}

      <div className="flex flex-col gap-2">
        <span className="text-[13px] font-semibold">Pay with</span>
        <div className="flex items-center gap-3 rounded-lg border p-3">
          <span
            aria-hidden="true"
            className="flex h-8 w-11 items-center justify-center rounded-md bg-muted text-muted-foreground"
          >
            <CreditCardIcon className="size-5" />
          </span>
          <span className="flex flex-col leading-snug">
            <span className="font-bold">
              {card.brand} ending {card.last4}
            </span>
            <span className="text-xs text-muted-foreground">
              Expires {String(card.exp_month).padStart(2, '0')}/{String(card.exp_year).slice(-2)}
            </span>
          </span>
        </div>
      </div>

      <dl className="flex flex-col gap-2 rounded-lg border p-4 text-sm">
        <div className="flex justify-between gap-3">
          <dt className="text-muted-foreground">Current balance</dt>
          <dd className="tabular-nums">{money(wallet.balance_minor)}</dd>
        </div>
        <div className="flex justify-between gap-3">
          <dt className="text-muted-foreground">Top up</dt>
          <dd className="tabular-nums">+{money(tooSmall ? 0 : amount)}</dd>
        </div>
        <div className="flex justify-between gap-3 border-t pt-2 font-bold">
          <dt>New balance</dt>
          <dd className="tabular-nums">{money(wallet.balance_minor + (tooSmall ? 0 : amount))}</dd>
        </div>
      </dl>

      <p id={`${id}-soon`} className="text-xs text-muted-foreground">
        Wallet top ups are coming soon. This is a preview, so nothing is charged.
      </p>
    </Modal>
  )
}
