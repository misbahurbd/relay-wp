import { InfoIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { StepHeading } from './audience-step'

export type ReviewRow = { label: string; value: string; step: number }

/** Step 4: everything once more, each row a way back to its step, and what Meta charges. */
export function ReviewStep({
  rows,
  marketing,
  onEdit,
}: {
  rows: ReviewRow[]
  marketing: boolean
  onEdit: (step: number) => void
}) {
  return (
    <>
      <StepHeading title="Review and send">
        Check everything once more. You can’t edit a broadcast after it starts sending.
      </StepHeading>
      <dl className="flex flex-col rounded-[14px] border">
        {rows.map((r) => (
          <div
            key={r.label}
            className="flex flex-wrap items-center gap-x-4 gap-y-1 border-b px-4 py-3.5 last:border-b-0"
          >
            <dt className="w-[120px] flex-none font-semibold text-muted-foreground">{r.label}</dt>
            <dd className="min-w-[200px] flex-1 font-bold">{r.value}</dd>
            <Button
              variant="ghost"
              size="sm"
              className="text-brand-text"
              onClick={() => onEdit(r.step)}
            >
              Edit<span className="sr-only"> {r.label.toLowerCase()}</span>
            </Button>
          </div>
        ))}
      </dl>
      <div className="flex flex-col gap-2 rounded-[14px] bg-muted p-4">
        <h3 className="flex items-center gap-2 text-[15px] font-extrabold">
          <InfoIcon className="size-[18px] text-info" aria-hidden="true" />
          What Meta charges
        </h3>
        <p className="text-[13px] text-muted-foreground">
          {marketing
            ? 'Meta charges for each delivered marketing message. '
            : 'Meta charges for each delivered utility message, unless the customer wrote to you in the last 24 hours. '}
          Prices depend on each contact’s country and are billed by Meta to your WhatsApp Business
          account.{' '}
          <a
            href="https://developers.facebook.com/docs/whatsapp/pricing"
            target="_blank"
            rel="noreferrer"
            className="font-bold text-brand-text"
          >
            See Meta’s prices
          </a>
        </p>
      </div>
    </>
  )
}
