import { Section } from '../kit/section'
import { NoPayPermission } from '../kit/states'
import { PlanChoices } from './plan-choices'

/** The plan catalog on the billing page, with the current plan marked. */
export function ComparePlans({ workspaceId, canPay }: { workspaceId: string; canPay: boolean }) {
  return (
    <Section id="plans" aria-labelledby="compare-heading" className="scroll-mt-20">
      <PlanChoices
        workspaceId={workspaceId}
        canPay={canPay}
        header={(toggle) => (
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="flex min-w-0 flex-col gap-0.5">
              <h2 id="compare-heading" className="text-[17px] font-bold">
                Compare plans
              </h2>
              <p className="text-[13px] text-muted-foreground">
                All plans use the official WhatsApp Business API. Meta message costs are billed
                separately.
              </p>
              {!canPay && <NoPayPermission />}
            </div>
            {toggle}
          </div>
        )}
      />
    </Section>
  )
}
