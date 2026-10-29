import { Link } from '@tanstack/react-router'
import { CheckIcon, TriangleAlertIcon } from 'lucide-react'
import { Alert, AlertDescription, AlertTitle } from '#/components/ui/alert'
import { Badge } from '#/components/ui/badge'
import { Button } from '#/components/ui/button'
import { GRANT_SOURCE } from '../../schemas/billing.schema'
import type { Overview } from '../../types'
import { formatDate } from '../kit/format'
import { limitLines } from '../kit/money'
import { Section, SectionHeader } from '../kit/section'

/** The plan in force now. A suspended grant shows free limits until the missed payment is in. */
export function CurrentPlan({ overview, canPay }: { overview: Overview; canPay: boolean }) {
  const { plan } = overview
  const grant = plan.grant
  const suspended = grant?.status === 'suspended'

  return (
    <Section aria-labelledby="plan-heading">
      <SectionHeader
        id="plan-heading"
        title="Current plan"
        action={
          canPay && (
            <Button asChild variant={grant ? 'outline' : 'default'}>
              <Link to="/billing/plans">{grant ? 'Change plan' : 'Upgrade'}</Link>
            </Button>
          )
        }
      />
      <div className="flex flex-col gap-1">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-2xl font-extrabold tracking-tight">{plan.plan_name}</p>
          {grant && <Badge variant="neutral">{GRANT_SOURCE[grant.source]}</Badge>}
          {suspended && <Badge variant="danger">Suspended</Badge>}
        </div>
        <p className="text-[13px] text-muted-foreground">
          {!grant
            ? 'The free plan. Upgrade for more team members and WhatsApp numbers.'
            : suspended
              ? 'Your lifetime deal is paused until the missed payment is in.'
              : grant.ends_at === null
                ? `Yours for life, since ${formatDate(grant.starts_at)}.`
                : grant.source === 'trial'
                  ? `Trial ends ${formatDate(grant.ends_at)}.`
                  : `Current period ends ${formatDate(grant.ends_at)}.`}
        </p>
      </div>

      {suspended && (
        <Alert variant="warning">
          <TriangleAlertIcon aria-hidden="true" />
          <AlertTitle>Free limits apply for now</AlertTitle>
          <AlertDescription>
            <p>
              An installment was missed
              {grant.suspended_at && `, so access was limited on ${formatDate(grant.suspended_at)}`}
              . Pay it below to get your plan back straight away.
            </p>
          </AlertDescription>
        </Alert>
      )}

      <div className="flex flex-col gap-2">
        <h3 className="text-[13px] font-bold">{suspended ? 'What applies now' : 'Included'}</h3>
        <ul className="flex flex-col gap-1.5 text-sm">
          {limitLines(plan.limits).map((line) => (
            <li key={line} className="flex items-center gap-2">
              <CheckIcon className="size-4 flex-none text-brand-text" aria-hidden="true" />
              {line}
            </li>
          ))}
        </ul>
      </div>
    </Section>
  )
}
