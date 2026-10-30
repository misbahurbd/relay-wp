import { type QueryClient, type QueryKey, useQueryClient } from '@tanstack/react-query'
import { useEffect, useRef } from 'react'
import { toast } from 'sonner'
import { inWorkspace, refreshSession } from '#/lib/api'
import { env } from '#/lib/env'
import { inboxKeys } from '../api/inbox.keys'
import { mentionsQueryOptions } from '../api/inbox.queries'
import type { LiveEvent } from '../types'
import { setLiveConnected } from './use-live-status'

const DEBOUNCE_MS = 150
const MAX_BACKOFF_MS = 30_000
const KINDS = new Set<LiveEvent['kind']>(['message', 'status', 'conversation', 'note', 'mention'])

type Frame = { event: string; data: string }

/**
 * Reads a text/event-stream body and calls `onFrame` per event. Comments (": ping") are
 * skipped, `retry:` is reported so reconnects honour the server's delay.
 */
async function readStream(
  body: ReadableStream<Uint8Array>,
  onFrame: (frame: Frame) => void,
  onRetry: (ms: number) => void,
) {
  const reader = body.getReader()
  const decoder = new TextDecoder()
  let buffer = ''
  let event = ''
  let data: string[] = []
  try {
    while (true) {
      const { done, value } = await reader.read()
      if (done) return
      buffer += decoder.decode(value, { stream: true })
      const lines = buffer.split(/\r\n|\r|\n/)
      buffer = lines.pop() ?? ''
      for (const line of lines) {
        if (line === '') {
          if (data.length > 0) onFrame({ event: event || 'message', data: data.join('\n') })
          event = ''
          data = []
          continue
        }
        if (line.startsWith(':')) continue
        const colon = line.indexOf(':')
        const field = colon === -1 ? line : line.slice(0, colon)
        let value = colon === -1 ? '' : line.slice(colon + 1)
        if (value.startsWith(' ')) value = value.slice(1)
        if (field === 'event') event = value
        else if (field === 'data') data.push(value)
        else if (field === 'retry' && /^\d+$/.test(value)) onRetry(Number(value))
      }
    }
  } finally {
    reader.releaseLock()
  }
}

function sleep(ms: number, signal: AbortSignal) {
  return new Promise<void>((resolve) => {
    const id = setTimeout(resolve, ms)
    signal.addEventListener(
      'abort',
      () => {
        clearTimeout(id)
        resolve()
      },
      { once: true },
    )
  })
}

/** Collects query keys and invalidates each once, ~150 ms after a burst of events settles. */
function batcher(queryClient: QueryClient) {
  const pending = new Map<string, QueryKey>()
  let timer: ReturnType<typeof setTimeout> | undefined
  return {
    add(...keys: QueryKey[]) {
      for (const k of keys) pending.set(JSON.stringify(k), k)
      clearTimeout(timer)
      timer = setTimeout(() => {
        const keys = [...pending.values()]
        pending.clear()
        for (const queryKey of keys) void queryClient.invalidateQueries({ queryKey })
      }, DEBOUNCE_MS)
    },
    cancel() {
      clearTimeout(timer)
      pending.clear()
    },
  }
}

/**
 * Live inbox updates (AD-8): one event stream per tab, each event a signal to refetch.
 *
 * Why fetch and not EventSource or the generated client: the stream needs the X-Workspace-ID
 * header, which EventSource can't send, and on a 401 it must refresh the session the way the
 * API client does and then reconnect, which needs the response status that the generated SSE
 * helper hides. This is the one hand-written request in the web app; every other call goes
 * through @relay/api-client.
 *
 * While disconnected, `useLiveConnected()` is false and lists and threads poll every 15 s.
 */
export function useInboxEvents(
  workspaceId: string,
  userId: string,
  openChat: (conversationId: string) => void,
) {
  const queryClient = useQueryClient()
  const openChatRef = useRef(openChat)
  openChatRef.current = openChat

  useEffect(() => {
    const controller = new AbortController()
    const { signal } = controller
    const batch = batcher(queryClient)

    const announceMention = async (event: LiveEvent) => {
      const mentions = await queryClient
        .fetchQuery({ ...mentionsQueryOptions(workspaceId), staleTime: 0 })
        .catch(() => null)
      const latest = mentions?.items.find(
        (m) => m.conversation_id === event.conversation_id && !m.seen_at,
      )
      const author = latest?.author?.name ?? 'A teammate'
      toast(`${author} mentioned you`, {
        description: latest ? latest.contact_name || latest.contact_phone : undefined,
        action: {
          label: 'Open chat',
          onClick: () => openChatRef.current(event.conversation_id),
        },
      })
    }

    const onFrame = ({ event, data }: Frame) => {
      let payload: LiveEvent
      try {
        payload = JSON.parse(data) as LiveEvent
      } catch {
        return
      }
      const kind = (
        KINDS.has(event as LiveEvent['kind']) ? event : payload.kind
      ) as LiveEvent['kind']
      batch.add(inboxKeys.lists(workspaceId))
      if (kind === 'mention') {
        if (payload.user_id && payload.user_id !== userId) return
        void announceMention(payload)
        return
      }
      if (payload.conversation_id) {
        // Only queries on screen refetch; others are just marked stale.
        batch.add(
          inboxKeys.detail(workspaceId, payload.conversation_id),
          inboxKeys.messages(workspaceId, payload.conversation_id),
        )
      }
    }

    const run = async () => {
      let retryMs = 3000
      let failures = 0
      let refreshed = false
      while (!signal.aborted) {
        try {
          const response = await fetch(`${env.VITE_API_URL}/v1/workspace/inbox/events`, {
            credentials: 'include',
            cache: 'no-store',
            headers: { ...inWorkspace(workspaceId), Accept: 'text/event-stream' },
            signal,
          })
          if (response.status === 401 && !refreshed) {
            // The access cookie expired: refresh once (shared with the API client), then retry now.
            refreshed = true
            void response.body?.cancel()
            if (await refreshSession()) continue
          }
          if (!response.ok || !response.body) throw new Error(`stream ${response.status}`)
          refreshed = false
          failures = 0
          setLiveConnected(true)
          // Anything may have changed while we were away: refetch what's on screen.
          // Stored files never change, so media blobs are left alone.
          void queryClient.invalidateQueries({
            queryKey: inboxKeys.workspace(workspaceId),
            predicate: (q) => q.queryKey[2] !== 'media',
          })
          await readStream(response.body, onFrame, (ms) => {
            retryMs = ms
          })
        } catch {
          if (signal.aborted) return
          failures += 1
        }
        setLiveConnected(false)
        if (signal.aborted) return
        // The server ends each stream after 15 minutes: reconnect after its retry delay. After
        // failures, back off exponentially with jitter, up to 30 s.
        const backoff = Math.min(retryMs * 2 ** Math.max(0, failures - 1), MAX_BACKOFF_MS)
        await sleep(failures === 0 ? retryMs : backoff * (0.8 + Math.random() * 0.4), signal)
      }
    }

    void run()
    return () => {
      controller.abort()
      batch.cancel()
      setLiveConnected(false)
    }
  }, [queryClient, workspaceId, userId])
}
