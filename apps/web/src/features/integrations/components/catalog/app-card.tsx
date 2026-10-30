import { AlertTriangleIcon, ShieldCheckIcon } from 'lucide-react'
import { type AppDef, CATEGORY_LABEL } from '../../schemas/integrations.schema'
import type { AppStatus } from '../../types'
import { AppTile } from '../kit/app-tile'
import { Btn } from '../kit/btn'
import { Pill } from '../kit/pill'

/** One app in the catalog: tile, name, description, status and its one action. */
export function AppCard({
  app,
  status,
  canAct,
  onOpen,
}: {
  app: AppDef
  status: AppStatus
  /** False when the member's role can't connect or manage it. */
  canAct: boolean
  onOpen: () => void
}) {
  const titleId = `app-${app.id}`
  return (
    <article
      aria-labelledby={titleId}
      className="flex flex-col gap-3 rounded-xl border bg-card p-5 shadow-card transition-shadow hover:shadow-md"
    >
      <div className="flex items-start gap-3">
        <AppTile mono={app.mono} tone={app.tone} />
        <div className="flex min-w-0 flex-1 flex-col">
          <h2 id={titleId} className="truncate text-base font-extrabold">
            {app.name}
          </h2>
          <span className="text-xs font-semibold text-muted-foreground">
            {CATEGORY_LABEL[app.category]}
          </span>
        </div>
        {status.connected &&
          (status.error ? (
            <Pill tone="amber" dot>
              Needs attention
            </Pill>
          ) : (
            <Pill tone="green" dot>
              Connected
            </Pill>
          ))}
      </div>
      <p className="flex-1 text-[13px] leading-relaxed text-muted-foreground">{app.description}</p>
      {status.connected && status.meta && (
        <span className="truncate rounded-lg bg-muted px-2.5 py-1.5 text-xs font-semibold text-muted-foreground">
          {status.meta}
        </span>
      )}
      {status.error && (
        <span className="flex items-start gap-1.5 text-xs font-semibold text-warning">
          <AlertTriangleIcon className="mt-px size-3.5 flex-none" aria-hidden="true" />
          {status.error}
        </span>
      )}
      <div className="flex flex-wrap items-center justify-between gap-2">
        {!status.available ? (
          <span className="text-xs font-semibold text-muted-foreground">
            Not set up on this server yet
          </span>
        ) : (
          <Btn
            size="sm"
            variant={status.connected ? 'outline' : 'primary'}
            disabled={!canAct}
            title={canAct ? undefined : "Your role can't change integrations"}
            onClick={onOpen}
            aria-describedby={titleId}
          >
            {status.connected ? 'Manage' : 'Connect'}
          </Btn>
        )}
        {status.available && !status.connected && status.oneClick && (
          <span className="flex items-center gap-1 text-xs font-semibold text-muted-foreground">
            <ShieldCheckIcon className="size-3.5 text-brand-text" aria-hidden="true" />
            Sign in with {app.id === 'google_sheets' ? 'Google' : app.name}
          </span>
        )}
      </div>
    </article>
  )
}
