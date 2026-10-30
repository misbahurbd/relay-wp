import { ArrowRightIcon, CheckIcon, KeyRoundIcon, LockIcon } from 'lucide-react'
import { type FormEvent, useState } from 'react'
import { toast } from 'sonner'
import { errorMessage } from '#/lib/api'
import { useAuthorizeStore, useConnectStore } from '../../api/integrations.mutations'
import {
  APP_BY_ID,
  STORE_ACCESS,
  shopifySchema,
  storeAddressSchema,
  wooSchema,
} from '../../schemas/integrations.schema'
import type { StoreCredentials, StoreProvider } from '../../types'
import { AppTile } from '../kit/app-tile'
import { Btn } from '../kit/btn'
import { ErrorText, TextField } from '../kit/field'
import { Modal } from '../kit/modal'

const HELP: Record<StoreProvider, string[]> = {
  shopify: [
    'In Shopify admin, open Settings → Apps and sales channels → Develop apps.',
    'Create an app, and under Admin API scopes allow read_customers and read_orders.',
    'Install the app, then copy the Admin API access token (it starts with shpat_).',
  ],
  woocommerce: [
    'In WordPress, open WooCommerce → Settings → Advanced → REST API.',
    'Add a key with Read permission for a store admin.',
    'Copy the consumer key (ck_…) and consumer secret (cs_…).',
  ],
}

const ADDRESS: Record<StoreProvider, { label: string; placeholder: string; field: Field }> = {
  shopify: {
    label: 'Store address',
    placeholder: 'your-store.myshopify.com',
    field: 'shop_domain',
  },
  woocommerce: { label: 'Store URL', placeholder: 'https://your-store.com', field: 'store_url' },
}

type Field = keyof StoreCredentials

/**
 * Connect a Shopify or WooCommerce store. With `oauth`, the owner approves Relay on the store's
 * own page (one click); credentials pasted by hand stay available as the fallback.
 */
export function StoreDialog({
  workspaceId,
  provider,
  oauth,
  onClose,
}: {
  workspaceId: string
  provider: StoreProvider
  /** The server can connect this store in one click. */
  oauth: boolean
  onClose: () => void
}) {
  const app = APP_BY_ID[provider]
  const [manual, setManual] = useState(!oauth)
  // The address carries over when switching between the two ways.
  const [values, setValues] = useState<StoreCredentials>({})

  return (
    <Modal
      onClose={onClose}
      title={`Connect ${app.name}`}
      description={app.description}
      icon={<AppTile mono={app.mono} tone={app.tone} />}
    >
      {manual ? (
        <ManualForm
          workspaceId={workspaceId}
          provider={provider}
          values={values}
          setValues={setValues}
          onBack={oauth ? () => setManual(false) : undefined}
          onClose={onClose}
        />
      ) : (
        <OneClickForm
          workspaceId={workspaceId}
          provider={provider}
          values={values}
          setValues={setValues}
          onManual={() => setManual(true)}
          onClose={onClose}
        />
      )}
    </Modal>
  )
}

type FormProps = {
  workspaceId: string
  provider: StoreProvider
  values: StoreCredentials
  setValues: (fn: (v: StoreCredentials) => StoreCredentials) => void
  onClose: () => void
}

function OneClickForm({
  workspaceId,
  provider,
  values,
  setValues,
  onManual,
  onClose,
}: FormProps & { onManual: () => void }) {
  const authorize = useAuthorizeStore(workspaceId)
  const app = APP_BY_ID[provider]
  const address = ADDRESS[provider]
  const [error, setError] = useState<string>()
  const value = values[address.field] ?? ''

  function submit(e: FormEvent) {
    e.preventDefault()
    const parsed = storeAddressSchema[provider].safeParse(value)
    if (!parsed.success) {
      setError(parsed.error.issues[0]?.message)
      return
    }
    setError(undefined)
    authorize.mutate(
      { provider, address: parsed.data },
      {
        onSuccess: (url) => window.location.assign(url),
        onError: (err) => setError(errorMessage(err)),
      },
    )
  }

  // Leaving for the store's page: keep the button busy until the browser navigates.
  const leaving = authorize.isPending || authorize.isSuccess

  return (
    <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
      <TextField
        id="store-address"
        label={address.label}
        type={provider === 'woocommerce' ? 'url' : 'text'}
        inputMode="url"
        placeholder={address.placeholder}
        value={value}
        onChange={(e) => setValues((v) => ({ ...v, [address.field]: e.target.value }))}
        error={error}
        hint={`You'll approve Relay on your ${app.name} admin, then come straight back here.`}
        autoComplete="off"
        spellCheck={false}
        autoFocus
      />

      <div className="flex flex-col gap-2 rounded-[10px] bg-muted p-3 text-[13px]">
        <span className="flex items-center gap-1.5 font-bold">
          <LockIcon className="size-3.5" aria-hidden="true" />
          Relay will be able to
        </span>
        <ul className="flex flex-col gap-1 text-muted-foreground">
          {STORE_ACCESS[provider].map((a) => (
            <li key={a} className="flex items-start gap-1.5">
              <CheckIcon className="mt-0.5 size-3.5 flex-none text-brand-text" aria-hidden="true" />
              {a}
            </li>
          ))}
        </ul>
      </div>

      <div className="flex flex-wrap items-center justify-between gap-2">
        <Btn variant="ghost" size="sm" onClick={onManual} className="px-2">
          <KeyRoundIcon className="size-4" aria-hidden="true" />
          Use {provider === 'shopify' ? 'an access token' : 'REST API keys'} instead
        </Btn>
        <div className="flex gap-2">
          <Btn variant="outline" onClick={onClose}>
            Cancel
          </Btn>
          <Btn type="submit" disabled={leaving}>
            {leaving ? `Opening ${app.name}…` : `Continue to ${app.name}`}
            {!leaving && <ArrowRightIcon className="size-4" aria-hidden="true" />}
          </Btn>
        </div>
      </div>
    </form>
  )
}

function ManualForm({
  workspaceId,
  provider,
  values,
  setValues,
  onBack,
  onClose,
}: FormProps & { onBack?: () => void }) {
  const connect = useConnectStore(workspaceId)
  const app = APP_BY_ID[provider]
  const [errors, setErrors] = useState<Partial<Record<Field, string>>>({})
  const [error, setError] = useState<string>()

  const set = (k: Field) => (e: { target: { value: string } }) =>
    setValues((v) => ({ ...v, [k]: e.target.value }))

  async function submit(e: FormEvent) {
    e.preventDefault()
    const parsed = (provider === 'shopify' ? shopifySchema : wooSchema).safeParse(values)
    if (!parsed.success) {
      setErrors(
        Object.fromEntries(
          Object.entries(parsed.error.flatten().fieldErrors).map(([k, v]) => [k, v?.[0]]),
        ),
      )
      return
    }
    setErrors({})
    setError(undefined)
    try {
      const c = await connect.mutateAsync({ provider, body: parsed.data })
      toast.success(`${app.name} connected: ${c.account_label}`)
      onClose()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  return (
    <>
      <ol className="list-decimal space-y-1 rounded-[10px] bg-muted p-3 pl-8 text-[13px] text-muted-foreground">
        {HELP[provider].map((step) => (
          <li key={step}>{step}</li>
        ))}
      </ol>
      <form onSubmit={submit} className="flex flex-col gap-4" noValidate>
        {provider === 'shopify' ? (
          <>
            <TextField
              id="shop-domain"
              label="Store address"
              placeholder="your-store.myshopify.com"
              value={values.shop_domain ?? ''}
              onChange={set('shop_domain')}
              error={errors.shop_domain}
              autoComplete="off"
              spellCheck={false}
              autoFocus={!values.shop_domain}
            />
            <TextField
              id="shop-token"
              label="Admin API access token"
              type="password"
              placeholder="shpat_…"
              value={values.access_token ?? ''}
              onChange={set('access_token')}
              error={errors.access_token}
              autoComplete="off"
              autoFocus={!!values.shop_domain}
            />
          </>
        ) : (
          <>
            <TextField
              id="woo-url"
              label="Store URL"
              type="url"
              inputMode="url"
              placeholder="https://your-store.com"
              value={values.store_url ?? ''}
              onChange={set('store_url')}
              error={errors.store_url}
              spellCheck={false}
              autoFocus={!values.store_url}
            />
            <TextField
              id="woo-key"
              label="Consumer key"
              placeholder="ck_…"
              value={values.consumer_key ?? ''}
              onChange={set('consumer_key')}
              error={errors.consumer_key}
              autoComplete="off"
              autoFocus={!!values.store_url}
            />
            <TextField
              id="woo-secret"
              label="Consumer secret"
              type="password"
              placeholder="cs_…"
              value={values.consumer_secret ?? ''}
              onChange={set('consumer_secret')}
              error={errors.consumer_secret}
              autoComplete="off"
            />
          </>
        )}
        <p className="flex items-center gap-1.5 text-xs text-muted-foreground">
          <LockIcon className="size-3.5 flex-none" aria-hidden="true" />
          Credentials are encrypted and only used to talk to your store.
        </p>
        <ErrorText>{error}</ErrorText>
        <div className="flex flex-wrap items-center justify-between gap-2">
          {onBack ? (
            <Btn variant="ghost" size="sm" onClick={onBack} className="px-2">
              Connect in one click instead
            </Btn>
          ) : (
            <span />
          )}
          <div className="flex gap-2">
            <Btn variant="outline" onClick={onClose}>
              Cancel
            </Btn>
            <Btn type="submit" disabled={connect.isPending}>
              {connect.isPending ? 'Checking…' : `Connect ${app.name}`}
            </Btn>
          </div>
        </div>
      </form>
    </>
  )
}
