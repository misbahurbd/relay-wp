import { useQuery } from '@tanstack/react-query'
import { Link } from '@tanstack/react-router'
import { ArrowUpRightIcon, InfoIcon } from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Skeleton } from '#/components/ui/skeleton'
import { overviewQueryOptions } from '#/features/integrations'
import { errorMessage, isApiError } from '#/lib/api'
import { useSetAutomationEnabled } from '../api/automations.mutations'
import { automationsQueryOptions } from '../api/automations.queries'
import type { Automation, Kind } from '../types'
import { AutomationCard } from './cards/automation-card'
import { AutomationDrawer } from './drawers/automation-drawer'

type Props = {
  workspaceId: string
  /** automations.edit: save settings. */
  canEdit: boolean
  /** automations.publish: turn automations on or off. */
  canPublish: boolean
}

/** The store a kind needs, when the workspace hasn't connected it. */
function missingStore(a: Automation, stores: string[]) {
  const need =
    a.kind === 'cart'
      ? 'shopify'
      : a.kind === 'order'
        ? a.settings.order?.trigger.startsWith('woocommerce')
          ? 'woocommerce'
          : 'shopify'
        : undefined
  if (!need || stores.includes(need)) return undefined
  return need === 'shopify' ? 'Shopify' : 'WooCommerce'
}

export function AutomationsPage({ workspaceId, canEdit, canPublish }: Props) {
  const automations = useQuery(automationsQueryOptions(workspaceId))
  const overview = useQuery(overviewQueryOptions(workspaceId))
  const setEnabled = useSetAutomationEnabled(workspaceId)
  const [editing, setEditing] = useState<Kind>()

  const items = automations.data ?? []
  const stores = (overview.data?.connections ?? [])
    .filter((c) => c.status !== 'disconnected')
    .map((c) => c.provider as string)
  const active = items.filter((a) => a.enabled).length
  const open = items.find((a) => a.kind === editing)

  if (isApiError(automations.error) && automations.error.code === 'plan_required') {
    return (
      <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
        <Heading />
        <div className="flex flex-col items-start gap-3 rounded-2xl border bg-card p-6 shadow-card">
          <h2 className="text-base font-bold">Automations aren’t on your plan</h2>
          <p className="text-[13px] text-muted-foreground">
            Upgrade to turn on welcome messages, keyword replies, cart reminders and more.
          </p>
          <Link
            to="/billing"
            className="inline-flex h-11 items-center rounded-lg bg-primary px-[18px] font-bold text-primary-foreground hover:bg-primary/90"
          >
            See plans
          </Link>
        </div>
      </main>
    )
  }

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-[22px] p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <Heading />
        <div className="flex flex-wrap items-center gap-2.5">
          {automations.data && (
            <span className="inline-flex h-10 items-center gap-2 rounded-full bg-brand-soft px-3.5 font-bold text-brand-text">
              <span aria-hidden="true" className="size-2 rounded-full bg-primary" />
              {active} of {items.length} active
            </span>
          )}
          <span
            title="Chatbots are coming soon"
            className="inline-flex h-11 items-center gap-2 rounded-lg border bg-card px-4 font-bold text-muted-foreground"
          >
            Need more? Build a chatbot
            <ArrowUpRightIcon className="size-4" aria-hidden="true" />
          </span>
        </div>
      </div>

      <div className="flex items-center gap-3 rounded-[14px] bg-info-soft px-4 py-3.5 text-[13px]">
        <InfoIcon className="size-[18px] flex-none text-info" aria-hidden="true" />
        <span>
          Replies inside the 24-hour window are free. Reminders sent later (like abandoned cart) use
          an approved template and are charged by Meta.
        </span>
      </div>

      {automations.isError ? (
        <p role="alert" className="text-sm font-semibold text-destructive">
          {errorMessage(automations.error, 'Could not load automations. Try again.')}
        </p>
      ) : (
        <section
          aria-label="Automations"
          className="grid grid-cols-[repeat(auto-fit,minmax(min(100%,320px),1fr))] gap-4"
        >
          {automations.isPending &&
            ['a', 'b', 'c', 'd', 'e', 'f'].map((k) => (
              <Skeleton key={k} className="h-[210px] rounded-2xl" />
            ))}
          {items.map((a) => (
            <AutomationCard
              key={a.kind}
              automation={a}
              needsStore={missingStore(a, stores)}
              canPublish={canPublish}
              busy={setEnabled.isPending}
              onEdit={() => setEditing(a.kind)}
              onToggle={(enabled) =>
                setEnabled.mutate(
                  { kind: a.kind, enabled },
                  {
                    onError: (e) => {
                      toast.error(errorMessage(e))
                      if (isApiError(e) && e.code === 'automation_not_ready') setEditing(a.kind)
                    },
                  },
                )
              }
            />
          ))}
        </section>
      )}

      {open && (
        <AutomationDrawer
          key={open.kind}
          workspaceId={workspaceId}
          automation={open}
          canEdit={canEdit}
          onClose={() => setEditing(undefined)}
        />
      )}
    </main>
  )
}

function Heading() {
  return (
    <div className="flex flex-col gap-1">
      <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Automations</h1>
      <p className="text-[15px] text-muted-foreground">
        Turn on ready-made replies and reminders. No coding needed.
      </p>
    </div>
  )
}
