import { LightbulbIcon } from 'lucide-react'

export function ApprovalTips() {
  return (
    <aside
      aria-labelledby="tips-title"
      className="flex flex-col gap-2 rounded-xl border bg-card p-4 shadow-card"
    >
      <h3 id="tips-title" className="flex items-center gap-2 text-sm font-extrabold">
        <LightbulbIcon className="size-4 text-warning" aria-hidden="true" />
        Tips for fast approval
      </h3>
      <ul className="flex list-disc flex-col gap-1 pl-5 text-[13px] text-muted-foreground">
        <li>Don’t start or end the body with a variable.</li>
        <li>Use your full website address, not short links.</li>
        <li>Pick Utility only for order and account updates. Offers are Marketing.</li>
        <li>Meta usually reviews templates within 24 hours.</li>
      </ul>
    </aside>
  )
}
