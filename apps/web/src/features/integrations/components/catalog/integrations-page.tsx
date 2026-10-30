import { useQuery } from '@tanstack/react-query'
import { SearchIcon, XIcon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { toast } from 'sonner'
import { Skeleton } from '#/components/ui/skeleton'
import { errorMessage } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useAuthorizeGoogle } from '../../api/integrations.mutations'
import { overviewQueryOptions } from '../../api/integrations.queries'
import { APPS, CALLBACK_ERRORS, CATEGORIES } from '../../schemas/integrations.schema'
import type { AppId, Category, KeyApp, StoreProvider } from '../../types'
import { ConnectionDialog } from '../connect/connection-dialog'
import { KeyAppDialog } from '../connect/key-app-dialog'
import { StoreDialog } from '../connect/store-dialog'
import { ApiKeysPanel } from '../keys/api-keys-panel'
import { Pill } from '../kit/pill'
import { WebhooksPanel } from '../webhooks/webhooks-panel'
import { AppCard } from './app-card'
import { appStatus } from './app-status'
import { DevSetupNotice } from './dev-setup-notice'

type Props = {
  workspaceId: string
  /** integrations.manage: stores, Sheets, webhooks. */
  canManage: boolean
  /** integrations.api_keys: keys, and the no-code apps that use them. */
  canKeys: boolean
  /**
   * From a provider's redirect: ?connected=<provider> or ?integration_error=<code> (Google,
   * Shopify), or WooCommerce's ?provider=woocommerce&success=1|0.
   */
  callback?: { connected?: string; error?: string; provider?: string; success?: string }
  onCallbackSeen: () => void
}

type Open =
  | { kind: 'store'; provider: StoreProvider }
  | { kind: 'manage'; provider: StoreProvider | 'google_sheets' }
  | { kind: 'key-app'; app: KeyApp }
  | null

const KEY_APPS: AppId[] = ['zapier', 'make', 'n8n']

export function IntegrationsPage({
  workspaceId,
  canManage,
  canKeys,
  callback,
  onCallbackSeen,
}: Props) {
  const overview = useQuery(overviewQueryOptions(workspaceId))
  const google = useAuthorizeGoogle(workspaceId)
  const [category, setCategory] = useState<Category | 'all'>('all')
  const [q, setQ] = useState('')
  const [open, setOpen] = useState<Open>(null)

  // biome-ignore lint/correctness/useExhaustiveDependencies: only a new callback should toast
  useEffect(() => {
    if (!callback?.connected && !callback?.error && !callback?.provider) return
    const connected =
      callback.connected ?? (callback.success === '1' ? callback.provider : undefined)
    const app = APPS.find((a) => a.id === (connected ?? callback.provider))
    if (connected && app) toast.success(`${app.name} connected`)
    else if (callback.error)
      toast.error(CALLBACK_ERRORS[callback.error] ?? "Couldn't connect. Try again.")
    else if (callback.success === '0' && app)
      toast.info(`${app.name} wasn't connected`, {
        description: 'You can approve Relay any time from this page.',
      })
    onCallbackSeen()
  }, [callback?.connected, callback?.error, callback?.provider, callback?.success])

  const term = q.trim().toLowerCase()
  const apps = APPS.filter(
    (a) =>
      (category === 'all' || a.category === category) &&
      (!term || a.name.toLowerCase().includes(term) || a.description.toLowerCase().includes(term)),
  )
  const statuses = overview.data
    ? Object.fromEntries(APPS.map((a) => [a.id, appStatus(a.id, overview.data)]))
    : undefined
  const connectedCount = statuses ? Object.values(statuses).filter((s) => s.connected).length : 0

  function scrollTo(id: string) {
    document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' })
  }

  function openApp(id: AppId) {
    const connected = statuses?.[id]?.connected
    switch (id) {
      case 'shopify':
      case 'woocommerce':
        setOpen(connected ? { kind: 'manage', provider: id } : { kind: 'store', provider: id })
        return
      case 'google_sheets':
        if (connected) setOpen({ kind: 'manage', provider: id })
        else
          google.mutate(undefined, {
            onSuccess: (url) => window.location.assign(url),
            onError: (err) => toast.error(errorMessage(err)),
          })
        return
      case 'zapier':
      case 'make':
      case 'n8n':
        setOpen({ kind: 'key-app', app: id })
        return
      case 'webhooks':
        scrollTo('developer-webhooks')
        return
      case 'api_keys':
        scrollTo('developer-api-keys')
        return
    }
  }

  const canAct = (id: AppId) => (KEY_APPS.includes(id) || id === 'api_keys' ? canKeys : canManage)

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex flex-col gap-1">
          <h1 className="text-[28px] font-extrabold tracking-[-0.02em]">Integrations</h1>
          <p className="text-[15px] text-muted-foreground">
            Connect your store and tools so orders, contacts and messages sync automatically.
          </p>
        </div>
        {statuses && (
          <Pill tone="green" dot className="px-3 py-1 text-[13px]">
            {connectedCount} connected
          </Pill>
        )}
      </div>

      {canManage && overview.data && <DevSetupNotice overview={overview.data} />}

      <div className="flex flex-wrap items-center justify-between gap-3">
        <div role="tablist" aria-label="Categories" className="flex flex-wrap gap-2">
          {CATEGORIES.map((c) => {
            const on = c.id === category
            const count = APPS.filter((a) => c.id === 'all' || a.category === c.id).length
            return (
              <button
                key={c.id}
                type="button"
                role="tab"
                aria-selected={on}
                onClick={() => setCategory(c.id)}
                className={cn(
                  'inline-flex h-9 items-center gap-1.5 rounded-full border px-4 text-[13px] font-bold transition-colors',
                  on
                    ? 'border-primary bg-primary text-primary-foreground'
                    : 'bg-card text-foreground hover:bg-muted',
                )}
              >
                {c.label}
                <span
                  className={cn(
                    'rounded-full px-1.5 text-[11px] tabular-nums',
                    on ? 'bg-primary-foreground/20' : 'bg-muted text-muted-foreground',
                  )}
                >
                  {count}
                </span>
              </button>
            )
          })}
        </div>
        <label className="flex h-10 w-full max-w-[300px] items-center gap-2 rounded-[10px] border border-input bg-card px-3 text-muted-foreground focus-within:outline-2 focus-within:outline-offset-2 focus-within:outline-ring sm:w-auto sm:flex-[0_1_300px]">
          <SearchIcon className="size-4 flex-none" aria-hidden="true" />
          <span className="sr-only">Search apps</span>
          <input
            type="search"
            value={q}
            onChange={(e) => setQ(e.target.value)}
            placeholder="Search apps"
            className="min-w-0 flex-1 bg-transparent text-base text-foreground outline-none placeholder:text-muted-foreground md:text-[13px] [&::-webkit-search-cancel-button]:hidden"
          />
          {q && (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => setQ('')}
              className="-mr-1 flex size-7 flex-none items-center justify-center rounded-md hover:bg-muted hover:text-foreground"
            >
              <XIcon className="size-3.5" aria-hidden="true" />
            </button>
          )}
        </label>
      </div>

      {overview.isError ? (
        <p role="alert" className="rounded-xl border bg-card p-5 text-sm text-destructive">
          {errorMessage(overview.error, "Couldn't load integrations. Refresh to try again.")}
        </p>
      ) : (
        <section
          aria-label="Apps"
          className="grid grid-cols-[repeat(auto-fill,minmax(260px,1fr))] gap-4"
        >
          {statuses
            ? apps.map((a) => (
                <AppCard
                  key={a.id}
                  app={a}
                  status={statuses[a.id]}
                  canAct={canAct(a.id)}
                  onOpen={() => openApp(a.id)}
                />
              ))
            : APPS.map((a) => <Skeleton key={a.id} className="h-[214px] rounded-xl" />)}
        </section>
      )}
      {statuses && apps.length === 0 && (
        <div className="flex flex-col items-center gap-2 rounded-xl border border-dashed py-10 text-center">
          <p className="text-sm font-bold">No apps match “{q.trim()}”</p>
          <button
            type="button"
            onClick={() => {
              setQ('')
              setCategory('all')
            }}
            className="text-[13px] font-bold text-brand-text hover:underline"
          >
            Show all apps
          </button>
        </div>
      )}

      <section aria-label="Developer settings" className="grid gap-4 pt-2 lg:grid-cols-2">
        {canKeys && <ApiKeysPanel id="developer-api-keys" workspaceId={workspaceId} />}
        {canManage && <WebhooksPanel id="developer-webhooks" workspaceId={workspaceId} />}
      </section>

      {open?.kind === 'store' && (
        <StoreDialog
          workspaceId={workspaceId}
          provider={open.provider}
          oauth={
            open.provider === 'shopify'
              ? !!overview.data?.shopify_oauth
              : !!overview.data?.woocommerce_oauth
          }
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === 'manage' && overview.data && (
        <ConnectionDialog
          workspaceId={workspaceId}
          connection={overview.data.connections.find((c) => c.provider === open.provider)}
          provider={open.provider}
          onReconnect={() =>
            open.provider === 'google_sheets'
              ? google.mutate(undefined, {
                  onSuccess: (url) => window.location.assign(url),
                  onError: (err) => toast.error(errorMessage(err)),
                })
              : setOpen({ kind: 'store', provider: open.provider })
          }
          onClose={() => setOpen(null)}
        />
      )}
      {open?.kind === 'key-app' && (
        <KeyAppDialog workspaceId={workspaceId} app={open.app} onClose={() => setOpen(null)} />
      )}
    </main>
  )
}
