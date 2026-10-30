import { Link } from '@tanstack/react-router'
import { InfoIcon } from 'lucide-react'
import { useId } from 'react'
import { cn } from '#/lib/utils'

/** Plain-language rules for Meta's per-message charges. */
export function HowMetaCharges({ className }: { className?: string }) {
  const id = useId()
  return (
    <aside
      aria-labelledby={`${id}-title`}
      className={cn(
        'flex min-w-0 flex-col gap-3 rounded-xl border bg-brand-soft px-5 py-5 sm:px-[22px]',
        className,
      )}
    >
      <div className="flex items-center gap-2.5">
        <span
          aria-hidden="true"
          className="flex size-9 items-center justify-center rounded-[10px] bg-card text-brand-text"
        >
          <InfoIcon className="size-[18px]" />
        </span>
        <h2 id={`${id}-title`} className="text-[17px] font-bold">
          How Meta charges
        </h2>
      </div>
      <ul className="flex list-disc flex-col gap-2 pl-[18px]">
        <li>
          Meta charges for each <strong>template message</strong> that is delivered. The price
          depends on its category (Marketing, Utility, Authentication) and the customer’s country.
        </li>
        <li>
          When a customer messages you, replies inside the{' '}
          <strong>24-hour customer service window</strong> are free.
        </li>
        <li>
          Messages that <strong>fail</strong> are not charged. If we already reserved money for
          them, it comes back to your wallet as a refund.
        </li>
        <li>
          Your Relay plan is billed separately to your card. Meta charges come out of your wallet.
        </li>
      </ul>
      <Link
        to="/billing"
        className="self-start text-[13px] font-bold text-brand-text underline hover:text-foreground"
      >
        See current rates by country
      </Link>
    </aside>
  )
}
