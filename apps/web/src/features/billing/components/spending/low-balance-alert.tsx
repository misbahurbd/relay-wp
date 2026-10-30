import { TriangleAlertIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
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
    <Alert variant="warning" className="border-warning-bar">
      <TriangleAlertIcon aria-hidden="true" />
      <AlertTitle className="font-bold">
        Wallet balance is low: {money(wallet.balance_minor)}
      </AlertTitle>
      <AlertDescription className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2">
        <p>
          Your scheduled “{short.broadcast}” broadcast needs about {money(short.needed_minor)}. Top
          up so it isn’t paused.
        </p>
        <Button type="button" size="sm" onClick={onTopUp}>
          Top up wallet
        </Button>
      </AlertDescription>
    </Alert>
  )
}
