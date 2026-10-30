import { Link } from '@tanstack/react-router'
import { ChevronLeftIcon } from 'lucide-react'
import { Logo } from '#/components/layout/logo'
import { ThemeToggle } from '#/components/layout/theme-toggle'
import { Button } from '#/components/ui/button'
import { NoPayPermission } from '../kit/states'
import { Catalogue } from './catalogue'
import { PlanChoices } from './plan-choices'

/** The public pricing page. Buying needs an account and a workspace, so checkout asks to sign in. */
export function PublicPricingPage() {
  return (
    <div className="min-h-dvh bg-background">
      <header className="mx-auto flex w-full max-w-[1200px] items-center justify-between gap-3 px-4 py-4 sm:px-7">
        <Logo />
        <div className="flex items-center gap-2">
          <ThemeToggle />
          <Button variant="ghost" asChild>
            <Link to="/login">Log in</Link>
          </Button>
          <Button asChild className="hidden sm:inline-flex">
            <Link to="/signup">Start free</Link>
          </Button>
        </div>
      </header>
      <main className="mx-auto flex w-full max-w-[1200px] flex-col gap-8 px-4 pt-6 pb-16 sm:px-7">
        <div className="flex flex-col items-center gap-2 text-center">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em] sm:text-4xl">
            Plans for every team
          </h1>
          <p className="max-w-prose text-[15px] text-muted-foreground">
            Pay monthly or yearly, or buy a lifetime deal once and spread it over a few months.
            Prices in USD.
          </p>
        </div>
        <Catalogue
          action={({ selection }) => (
            <Button asChild size="lg">
              <Link
                to="/billing/checkout"
                search={{ price: selection.priceId, offer: selection.offerId }}
              >
                Choose plan
              </Link>
            </Button>
          )}
        />
      </main>
    </div>
  )
}

/** Plans inside the app: switch a subscription in place, or check out a new plan. */
export function PlanPicker({ workspaceId, canPay }: { workspaceId: string; canPay: boolean }) {
  return (
    <main className="mx-auto flex w-full max-w-[1200px] flex-col gap-6 p-4 sm:p-7">
      <nav aria-label="Breadcrumb">
        <Link
          to="/billing"
          className="inline-flex items-center gap-1.5 text-[13px] font-semibold text-brand-text no-underline hover:text-foreground"
        >
          <ChevronLeftIcon className="size-4" aria-hidden="true" />
          Billing
        </Link>
      </nav>
      <div className="flex flex-col gap-1">
        <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Change plan</h1>
        <p className="text-[15px] text-muted-foreground">
          Upgrades apply now and are prorated. Downgrades start when your current period ends.
        </p>
      </div>
      {!canPay && <NoPayPermission />}
      <PlanChoices workspaceId={workspaceId} canPay={canPay} />
    </main>
  )
}
