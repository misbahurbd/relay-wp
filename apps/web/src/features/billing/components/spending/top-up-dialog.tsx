import { useState } from 'react'
import { Button } from '#/components/ui/button'
import { Dialog, DialogContent, DialogTitle } from '#/components/ui/dialog'
import { Input } from '#/components/ui/input'
import { cn } from '#/lib/utils'
import type { Wallet, WalletOutlook } from '../../types'
import { formatMoney } from '../kit/money'
import { formatWholeMoney, minorDigits, parseMajor } from './amounts'

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
  const options = outlook.top_up_options_minor
  const [choice, setChoice] = useState(String(options[Math.min(1, options.length - 1)] ?? OTHER))
  const [custom, setCustom] = useState('75')
  const money = (minor: number) => formatMoney(minor, wallet.currency)

  const amount =
    choice === OTHER ? (parseMajor(custom, wallet.currency) ?? 0) : Number.parseInt(choice, 10)
  const { card } = outlook
  const chips = [
    ...options.map((o) => ({ value: String(o), label: formatWholeMoney(o, wallet.currency) })),
    { value: OTHER, label: 'Other' },
  ]

  return (
    <Dialog open={open} onOpenChange={(o) => !o && onClose()}>
      <DialogContent
        aria-describedby={undefined}
        className="flex max-h-[calc(100dvh-32px)] flex-col gap-[18px] overflow-y-auto rounded-[18px] p-6 sm:max-w-[460px] [&>button:last-child]:top-6 [&>button:last-child]:right-6"
      >
        <DialogTitle className="text-[19px] font-extrabold">Top up wallet</DialogTitle>

        <div className="flex flex-col gap-2">
          <span className="text-[13px] font-bold">Amount</span>
          <div className="grid grid-cols-4 gap-2">
            {chips.map((c) => {
              const on = c.value === choice
              return (
                <Button
                  variant="unstyled"
                  key={c.value}
                  type="button"
                  aria-pressed={on}
                  onClick={() => setChoice(c.value)}
                  className={cn(
                    'h-[46px] rounded-[10px] border-2 text-[15px] font-extrabold',
                    on ? 'border-brand bg-brand-soft' : 'border-border bg-card',
                  )}
                >
                  {c.label}
                </Button>
              )
            })}
          </div>
          {choice === OTHER && (
            <label className="flex flex-col gap-1.5 text-[13px] font-semibold">
              Custom amount
              <span className="flex h-11 items-center rounded-[10px] border bg-muted px-3 focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring">
                <span className="font-bold text-muted-foreground">$</span>
                <Input
                  unstyled
                  type="number"
                  min={outlook.min_top_up_minor / 10 ** minorDigits(wallet.currency)}
                  step={10}
                  value={custom}
                  onChange={(e) => setCustom(e.target.value)}
                  className="min-w-0 flex-1 border-0 bg-transparent pl-1.5 text-base font-bold text-foreground outline-none"
                />
              </span>
            </label>
          )}
        </div>

        <div className="flex flex-col gap-2">
          <span className="text-[13px] font-bold">Pay with</span>
          <label className="flex cursor-pointer items-center gap-3 rounded-xl border-2 border-brand bg-brand-soft px-3.5 py-3">
            <Input
              unstyled
              type="radio"
              name="card"
              defaultChecked
              className="size-[18px] accent-primary"
            />
            <span
              aria-hidden="true"
              className="flex h-7 w-[42px] items-center justify-center rounded-md bg-foreground text-[10px] font-extrabold text-card"
            >
              CARD
            </span>
            <span className="flex flex-1 flex-col leading-[1.3]">
              <span className="font-bold">Card ending {card.last4}</span>
              <span className="text-xs text-muted-foreground">
                Expires {String(card.exp_month).padStart(2, '0')}/{String(card.exp_year).slice(-2)}
              </span>
            </span>
          </label>
        </div>

        <div className="flex flex-col gap-1.5 rounded-xl bg-muted px-3.5 py-3 text-[13px]">
          <div className="flex justify-between">
            <span className="text-muted-foreground">Current balance</span>
            <span className="font-semibold tabular-nums">{money(wallet.balance_minor)}</span>
          </div>
          <div className="flex justify-between">
            <span className="text-muted-foreground">Top up</span>
            <span className="font-semibold tabular-nums">+{money(amount)}</span>
          </div>
          <div className="flex justify-between border-t pt-1.5">
            <span className="font-bold">New balance</span>
            <span className="font-extrabold tabular-nums">
              {money(wallet.balance_minor + amount)}
            </span>
          </div>
        </div>

        <div className="flex flex-wrap justify-end gap-2.5">
          <Button
            type="button"
            variant="outline"
            onClick={onClose}
            className="h-11 rounded-xl px-4 font-bold"
          >
            Cancel
          </Button>
          {/* Wallets aren't built yet, so paying stays off: this is a preview. */}
          <Button
            type="button"
            disabled
            title="Wallet top ups are coming soon"
            className="h-11 rounded-xl px-[18px] font-bold"
          >
            Pay {money(amount)}
            <span className="sr-only">(coming soon)</span>
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}
