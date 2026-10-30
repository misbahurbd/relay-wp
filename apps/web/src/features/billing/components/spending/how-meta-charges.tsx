import { InfoIcon } from 'lucide-react'
import { useId } from 'react'
import { ComingSoonButton } from './coming-soon-button'

/** Plain-language rules for Meta's per-message charges. */
export function HowMetaCharges() {
  const id = useId()
  return (
    <aside
      aria-labelledby={`${id}-title`}
      className="flex flex-col gap-3 rounded-xl border bg-brand-soft p-5 sm:p-6"
    >
      <div className="flex items-center gap-2">
        <InfoIcon aria-hidden="true" className="size-5 text-brand-text" />
        <h2 id={`${id}-title`} className="text-base font-extrabold tracking-tight">
          How Meta charges
        </h2>
      </div>
      <ul className="flex list-disc flex-col gap-2 pl-5 text-[13px] leading-relaxed">
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
      <div>
        <ComingSoonButton variant="link">See current rates by country</ComingSoonButton>
      </div>
    </aside>
  )
}
