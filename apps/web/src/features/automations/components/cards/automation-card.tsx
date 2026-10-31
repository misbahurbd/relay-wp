import { Link } from '@tanstack/react-router'
import { BarChart3Icon, PencilIcon } from 'lucide-react'
import { cn } from '#/lib/utils'
import { description, KINDS, stat } from '../../schemas/automations.schema'
import type { Automation } from '../../types'
import { Toggle } from '../kit/toggle'

/** One automation: toggle, stat, status and Edit settings. */
export function AutomationCard({
  automation: a,
  needsStore,
  canPublish,
  busy,
  onToggle,
  onEdit,
}: {
  automation: Automation
  /** The store this kind needs, when it isn't connected. */
  needsStore?: string
  canPublish: boolean
  busy: boolean
  onToggle: (on: boolean) => void
  onEdit: () => void
}) {
  const kind = KINDS[a.kind]
  const Icon = kind.icon
  const blocked = !canPublish || (needsStore !== undefined && !a.enabled)
  return (
    <div className="flex flex-col gap-3.5 rounded-2xl border bg-card p-5 shadow-card">
      <div className="flex items-start gap-3.5">
        <span
          className={cn(
            'flex size-[46px] flex-none items-center justify-center rounded-[13px]',
            kind.iconClass,
          )}
        >
          <Icon className="size-[22px]" aria-hidden="true" />
        </span>
        <div className="flex min-w-0 flex-1 flex-col gap-1">
          <h2 className="text-base font-bold">{kind.title}</h2>
          <p className="text-[13px] text-muted-foreground">{description(a)}</p>
        </div>
        <span className="mt-2 flex-none">
          <Toggle
            on={a.enabled}
            onChange={onToggle}
            label={`${kind.title} is ${a.enabled ? 'on' : 'off'}`}
            disabled={blocked || busy}
            title={canPublish ? undefined : "Your role can't turn automations on or off"}
          />
        </span>
      </div>
      <div className="flex items-center gap-2 rounded-[10px] bg-muted px-3 py-2.5 text-[13px]">
        <BarChart3Icon className="size-[15px] text-muted-foreground" aria-hidden="true" />
        <span className="flex-1">{stat(a)}</span>
      </div>
      {needsStore && (
        <p className="text-[13px] text-muted-foreground">
          Connect {needsStore} to use this.{' '}
          <Link to="/integrations" className="font-bold text-brand-text">
            Go to Integrations
          </Link>
        </p>
      )}
      <div className="mt-auto flex items-center justify-between gap-2.5">
        <span
          className={cn(
            'inline-flex items-center gap-1.5 text-xs font-bold',
            a.enabled ? 'text-brand-text' : 'text-muted-foreground',
          )}
        >
          <span aria-hidden="true" className="size-[7px] rounded-full bg-current" />
          {a.enabled ? 'Active' : 'Paused'}
        </span>
        <button
          type="button"
          onClick={onEdit}
          className="inline-flex h-10 items-center gap-1.5 rounded-[10px] border bg-card px-3.5 text-[13px] font-bold hover:bg-muted"
        >
          <PencilIcon className="size-[15px]" aria-hidden="true" />
          Edit settings
        </button>
      </div>
    </div>
  )
}
