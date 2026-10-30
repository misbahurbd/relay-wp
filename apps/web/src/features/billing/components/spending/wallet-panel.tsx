import { CheckIcon, PlusIcon, WalletIcon } from 'lucide-react'
import { useId, useState } from 'react'
import { Button } from '#/components/ui/button'
import { cn } from '#/lib/utils'
import type { Wallet, WalletOutlook } from '../../types'
import { formatMoney } from '../kit/money'
import { Section } from '../kit/section'
import { SwitchRow } from './switch-row'

/** Balance, how long it lasts, Top up and auto-recharge. */
export function WalletPanel({
  wallet,
  outlook,
  onTopUp,
  className,
}: {
  wallet: Wallet
  outlook: WalletOutlook
  onTopUp: () => void
  className?: string
}) {
  const id = useId()
  const [autoRecharge, setAutoRecharge] = useState(wallet.auto_recharge.enabled)
  const [toast, setToast] = useState<string>()
  const money = (minor: number) => formatMoney(minor, wallet.currency)
  const low = outlook.shortfall !== null && outlook.shortfall.needed_minor > wallet.balance_minor
  const days =
    outlook.avg_daily_spend_minor > 0
      ? Math.max(1, Math.round(wallet.balance_minor / outlook.avg_daily_spend_minor))
      : null

  return (
    <Section aria-labelledby={`${id}-title`} className={cn('sm:py-5', className)}>
      <div className="flex items-center gap-2.5">
        <span
          aria-hidden="true"
          className="flex size-10 items-center justify-center rounded-xl bg-brand-soft text-brand-text"
        >
          <WalletIcon className="size-[18px]" />
        </span>
        <h2 id={`${id}-title`} className="text-[17px] font-bold">
          Message wallet
        </h2>
      </div>

      <div className="flex flex-col gap-0.5">
        <span className="text-[13px] font-semibold text-muted-foreground">Available balance</span>
        <span
          className={cn(
            'text-4xl font-extrabold tracking-[-0.02em] tabular-nums',
            low && 'text-warning',
          )}
        >
          {money(wallet.balance_minor)}
          {low && <span className="sr-only"> (low)</span>}
        </span>
        {days !== null && (
          <span className="text-xs text-muted-foreground">
            About {days} {days === 1 ? 'day' : 'days'} at your current daily spend
          </span>
        )}
      </div>

      <Button type="button" onClick={onTopUp} className="h-[46px] rounded-xl font-bold">
        <PlusIcon aria-hidden="true" />
        Top up
      </Button>

      <SwitchRow
        className="border-t pt-3.5"
        label="Auto-recharge"
        hint={`When balance drops below ${money(wallet.auto_recharge.below_minor)}, add ${money(
          wallet.auto_recharge.add_minor,
        )} from card ending ${outlook.card.last4}`}
        checked={autoRecharge}
        onChange={(on) => {
          setAutoRecharge(on)
          setToast(on ? 'Auto-recharge is on' : 'Auto-recharge is off')
        }}
      />

      {toast && (
        <div
          role="status"
          className="flex items-center gap-2 rounded-[10px] bg-brand-soft px-3 py-2.5 text-[13px] font-bold text-brand-text"
        >
          <CheckIcon className="size-[18px]" aria-hidden="true" />
          {toast}
        </div>
      )}
    </Section>
  )
}
