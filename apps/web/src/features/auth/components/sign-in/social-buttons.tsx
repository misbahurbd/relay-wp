import { useQuery } from '@tanstack/react-query'
import { Button } from '#/components/ui/button'
import { FieldSeparator } from '#/components/ui/field'
import { oauthStartUrl, providersQueryOptions, sortProviders } from '../../api/auth.api'
import { PROVIDER_META } from './provider-icons'

type Props = {
  /** "sign up" or "log in": used in the divider text. */
  action: 'sign up' | 'log in'
  /** Where to go after signing in; carried through the provider round trip. */
  redirect?: string
  /** Replaces the "or … with email" divider text. */
  divider?: string
}

/** One button per provider the API has configured; nothing at all when there are none. */
export function SocialButtons({ action, redirect, divider }: Props) {
  const returnPath = redirect ? `/login?redirect=${encodeURIComponent(redirect)}` : '/login'
  const { data } = useQuery(providersQueryOptions())
  const providers = sortProviders(data?.providers)
  if (providers.length === 0) return null

  return (
    <>
      <div className="flex flex-col gap-2.5">
        {providers.map((p) => {
          const { label, Icon } = PROVIDER_META[p]
          return (
            <Button
              key={p}
              variant={p === 'facebook' ? 'meta' : 'outline'}
              className="w-full [&_svg]:size-5"
              asChild
            >
              <a href={oauthStartUrl(p, returnPath)}>
                <Icon />
                Continue with {label}
              </a>
            </Button>
          )
        })}
      </div>
      <FieldSeparator className="*:data-[slot=field-separator-content]:bg-card">
        {divider ?? `or ${action} with email`}
      </FieldSeparator>
    </>
  )
}
