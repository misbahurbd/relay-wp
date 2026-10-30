import { TriangleAlertIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import type { Wallet, WalletOutlook } from '../../types'
import { formatMoney } from '../kit/money'

/** Shown when the wallet can't cover the next scheduled broadcast. */
export function LowBalanceAlert({
  wallet,
  outlook,
  onTopUp,
}: {
  wallet: Wallet
  outlook: WalletOutlook
  onTopUp: () => void
}) {
  const short = outlook.shortfall
  if (!short || short.needed_minor <= wallet.balance_minor) return null
  const money = (minor: number) => formatMoney(minor, wallet.currency)
  return (
    <div
      role="status"
      className="flex flex-wrap items-center gap-x-3.5 gap-y-2.5 rounded-[14px] border border-warning-bar bg-warning-soft px-4 py-3 text-warning"
    >
      <TriangleAlertIcon className="size-[18px] flex-none" aria-hidden="true" />
      <span className="font-bold">Wallet balance is low: {money(wallet.balance_minor)}</span>
      <span className="flex-[1_1_260px] text-foreground">
        Your scheduled “{short.broadcast}” broadcast needs about {money(short.needed_minor)}. Top up
        so it isn’t paused.
      </span>
      <Button type="button" onClick={onTopUp} className="h-10 rounded-[10px] px-3.5 font-bold">
        Top up wallet
      </Button>
    </div>
  )
}
