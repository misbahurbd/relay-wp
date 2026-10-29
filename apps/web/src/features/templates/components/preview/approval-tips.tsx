import { InfoIcon } from 'lucide-react'

export function ApprovalTips() {
  return (
    <aside
      aria-labelledby="tips-title"
      className="flex flex-col gap-2.5 rounded-xl bg-info-soft px-[18px] py-4"
    >
      <h3 id="tips-title" className="flex items-center gap-2 text-sm font-bold text-info">
        <InfoIcon className="size-[18px]" aria-hidden="true" />
        Tips for fast approval
      </h3>
      <ul className="flex list-disc flex-col gap-1.5 pl-[18px] text-[13px] text-foreground">
        <li>Don’t start or end the body with a variable.</li>
        <li>Use your full website address, not short links.</li>
        <li>Pick Utility only for order and account updates. Offers are Marketing.</li>
        <li>Meta usually reviews templates within 24 hours.</li>
      </ul>
    </aside>
  )
}
