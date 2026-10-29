import { useState } from 'react'
import { toast } from 'sonner'
import { useCreateProviderPrice } from '../../api/admin.api'
import type { InstallmentOffer, Price, Provider } from '../../types'
import { FormDialog } from '../kit/form-dialog'
import { SelectField } from '../kit/select-field'
import { TextField } from '../kit/text-field'

type Collection = 'provider_schedule' | 'app_link' | 'provider_financed'

const ALL: Provider[] = ['stripe', 'creem', 'paypal', 'sslcommerz']

/** Links a price, or one of its installment offers, to how a payment provider sells it. */
export function ProviderPriceDialog({
  price,
  offer,
  enabled,
  onClose,
}: {
  price: Price
  offer?: InstallmentOffer
  enabled: string[]
  onClose: () => void
}) {
  const create = useCreateProviderPrice()
  const first =
    (enabled.find((p) => ALL.includes(p as Provider)) as Provider | undefined) ?? 'stripe'
  const [provider, setProvider] = useState<Provider>(first)
  const [externalId, setExternalId] = useState('')
  const [collection, setCollection] = useState<Collection>('app_link')

  const submit = () =>
    create.mutate(
      {
        plan_price_id: price.id,
        installment_offer_id: offer?.id,
        provider,
        external_price_id: externalId.trim() || undefined,
        collection: offer ? collection : undefined,
      },
      {
        onSuccess: () => {
          toast.success(`On sale through ${provider}`)
          onClose()
        },
      },
    )

  return (
    <FormDialog
      title={offer ? `Sell the ${offer.count}-payment offer` : 'Sell this price'}
      description="Create the price or product in the provider's dashboard first, then paste its ID here."
      submitLabel="Link provider"
      pending={create.isPending}
      error={create.error}
      onSubmit={submit}
      onClose={onClose}
    >
      <SelectField
        id="pp-provider"
        label="Provider"
        value={provider}
        onChange={setProvider}
        options={ALL.map((p) => ({
          value: p,
          label: enabled.includes(p) ? p : `${p} (not enabled on this server)`,
        }))}
      />
      <TextField
        id="pp-external"
        label="Provider price or product ID"
        value={externalId}
        onChange={setExternalId}
        placeholder={provider === 'creem' ? 'prod_…' : 'price_…'}
        hint={
          price.billing_type === 'recurring'
            ? 'Required for subscriptions: the recurring price the provider renews.'
            : 'Optional for one-time payments on Stripe; Creem needs its product.'
        }
      />
      {offer && (
        <SelectField
          id="pp-collection"
          label="Who collects each payment"
          value={collection}
          onChange={setCollection}
          options={[
            { value: 'app_link', label: 'Relay emails a pay link (app_link)' },
            { value: 'provider_schedule', label: 'The provider charges on a schedule' },
            { value: 'provider_financed', label: 'A bank pays in full (SSLCommerz EMI)' },
          ]}
        />
      )}
    </FormDialog>
  )
}
