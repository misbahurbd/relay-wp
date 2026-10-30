import {
  authorizeGoogleSheets,
  authorizeShopify,
  authorizeWoocommerce,
  connectIntegration,
  createApiKey,
  createWebhook,
  deleteWebhook,
  disconnectIntegration,
  getWebhookSecret,
  redeliverWebhook,
  revokeApiKey,
  rollApiKey,
  rotateWebhookSecret,
  sendWebhookTest,
  testIntegration,
  updateWebhook,
} from '@relay/api-client'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { inWorkspace } from '#/lib/api'
import type {
  EndpointInput,
  KeyApp,
  NewApiKey,
  Provider,
  StoreCredentials,
  StoreProvider,
} from '../types'
import { invalidateIntegrations } from './integrations.keys'

/** Every change can move a card's status, so it refreshes all the workspace's integrations data. */
function useIntegrationsMutation<TInput, TOutput>(
  workspaceId: string,
  fn: (input: TInput) => Promise<TOutput>,
) {
  const queryClient = useQueryClient()
  return useMutation({
    mutationFn: fn,
    onSettled: () => invalidateIntegrations(queryClient, workspaceId),
  })
}

// ---------- API keys ----------

export function useCreateApiKey(workspaceId: string) {
  return useIntegrationsMutation(
    workspaceId,
    async (body: { name: string; app?: KeyApp }): Promise<NewApiKey> => {
      const { data } = await createApiKey({
        headers: inWorkspace(workspaceId),
        body,
        throwOnError: true,
      })
      return { apiKey: data.api_key, key: data.key }
    },
  )
}

export function useRollApiKey(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (keyId: string): Promise<NewApiKey> => {
    const { data } = await rollApiKey({
      headers: inWorkspace(workspaceId),
      path: { keyId },
      throwOnError: true,
    })
    return { apiKey: data.api_key, key: data.key }
  })
}

export function useRevokeApiKey(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (keyId: string) => {
    await revokeApiKey({ headers: inWorkspace(workspaceId), path: { keyId }, throwOnError: true })
  })
}

// ---------- webhooks ----------

export function useCreateWebhook(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (body: EndpointInput) => {
    const { data } = await createWebhook({
      headers: inWorkspace(workspaceId),
      body,
      throwOnError: true,
    })
    return data
  })
}

export function useUpdateWebhook(workspaceId: string) {
  return useIntegrationsMutation(
    workspaceId,
    async ({ endpointId, body }: { endpointId: string; body: EndpointInput }) => {
      const { data } = await updateWebhook({
        headers: inWorkspace(workspaceId),
        path: { endpointId },
        body,
        throwOnError: true,
      })
      return data
    },
  )
}

export function useDeleteWebhook(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (endpointId: string) => {
    await deleteWebhook({
      headers: inWorkspace(workspaceId),
      path: { endpointId },
      throwOnError: true,
    })
  })
}

/** Reads the signing secret on request; it's never cached. */
export function useRevealSecret(workspaceId: string) {
  return useMutation({
    mutationFn: async (endpointId: string) => {
      const { data } = await getWebhookSecret({
        headers: inWorkspace(workspaceId),
        path: { endpointId },
        throwOnError: true,
      })
      return data.secret
    },
  })
}

export function useRotateSecret(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (endpointId: string) => {
    const { data } = await rotateWebhookSecret({
      headers: inWorkspace(workspaceId),
      path: { endpointId },
      throwOnError: true,
    })
    return data.secret
  })
}

export function useSendTest(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (endpointId: string) => {
    const { data } = await sendWebhookTest({
      headers: inWorkspace(workspaceId),
      path: { endpointId },
      throwOnError: true,
    })
    return data
  })
}

export function useRedeliver(workspaceId: string) {
  return useIntegrationsMutation(
    workspaceId,
    async ({ endpointId, deliveryId }: { endpointId: string; deliveryId: string }) => {
      await redeliverWebhook({
        headers: inWorkspace(workspaceId),
        path: { endpointId, deliveryId },
        throwOnError: true,
      })
    },
  )
}

// ---------- connections ----------

export function useConnectStore(workspaceId: string) {
  return useIntegrationsMutation(
    workspaceId,
    async ({ provider, body }: { provider: StoreProvider; body: StoreCredentials }) => {
      const { data } = await connectIntegration({
        headers: inWorkspace(workspaceId),
        path: { provider },
        body,
        throwOnError: true,
      })
      return data
    },
  )
}

export function useTestConnection(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (provider: Provider) => {
    const { data } = await testIntegration({
      headers: inWorkspace(workspaceId),
      path: { provider },
      throwOnError: true,
    })
    return data
  })
}

export function useDisconnect(workspaceId: string) {
  return useIntegrationsMutation(workspaceId, async (provider: Provider) => {
    await disconnectIntegration({
      headers: inWorkspace(workspaceId),
      path: { provider },
      throwOnError: true,
    })
  })
}

/** Starts Google sign-in; the caller sends the browser to the returned URL. */
export function useAuthorizeGoogle(workspaceId: string) {
  return useMutation({
    mutationFn: async () => {
      const { data } = await authorizeGoogleSheets({
        headers: inWorkspace(workspaceId),
        throwOnError: true,
      })
      return data.url
    },
  })
}

/** Starts a one-click store connection: returns the store's page where the owner approves Relay. */
export function useAuthorizeStore(workspaceId: string) {
  return useMutation({
    mutationFn: async ({ provider, address }: { provider: StoreProvider; address: string }) => {
      const headers = inWorkspace(workspaceId)
      if (provider === 'shopify') {
        const { data } = await authorizeShopify({
          headers,
          body: { shop_domain: address },
          throwOnError: true,
        })
        return data.url
      }
      const { data } = await authorizeWoocommerce({
        headers,
        body: { store_url: address },
        throwOnError: true,
      })
      return data.url
    },
  })
}
