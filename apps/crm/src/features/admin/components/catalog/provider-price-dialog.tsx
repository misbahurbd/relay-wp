import { useState } from 'react'
import { toast } from 'sonner'
import { useCreateProviderPrice } from '../../api/admin.api'
import type { InstallmentOffer, Price, Provider } from '../../types'
import { CheckField } from '../kit/check-field'
import { FormDialog } from '../kit/form-dialog'
import { SelectField } from '../kit/select-field'
import { TextField } from '../kit/text-field'

type Collection = 'provider_schedule' | 'app_link' | 'provider_financed'

const ALL: Provider[] = ['stripe', 'creem', 'paypal', 'sslcommerz']

/**
 * Links a price, or one of its installment offers, to how a payment provider sells it.
 * A plain price is created on the provider unless staff paste an ID they made there.
 */
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
  const [useExisting, setUseExisting] = useState(false)
  const [collection, setCollection] = useState<Collection>('app_link')

  const createOnProvider = !offer && !useExisting
  const idMissing = !offer && useExisting && !externalId.trim()

  const submit = () =>
    create.mutate(
      {
        plan_price_id: price.id,
        installment_offer_id: offer?.id,
        provider,
        external_price_id: createOnProvider ? undefined : externalId.trim() || undefined,
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
      description={
        offer
          ? "Create the price or product in the provider's dashboard first, then paste its ID here."
          : `Relay creates the product and price on ${provider} for you, or you can paste an ID you made there.`
      }
      submitLabel={createOnProvider ? `Create on ${provider}` : 'Link provider'}
      pending={create.isPending}
      disabled={idMissing}
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
      {!offer && (
        <CheckField
          id="pp-existing"
          label="Use an existing provider ID instead"
          checked={useExisting}
          onChange={setUseExisting}
        />
      )}
      {!createOnProvider && (
        <TextField
          id="pp-external"
          label="Provider price or product ID"
          value={externalId}
          onChange={setExternalId}
          placeholder={provider === 'creem' ? 'prod_…' : 'price_…'}
          hint={
            offer
              ? 'Optional for installment offers on Stripe; Creem needs its product.'
              : 'The price or product you created in the provider.'
          }
        />
      )}
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
