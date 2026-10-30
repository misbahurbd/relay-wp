import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { mediaQueryOptions } from '../api/inbox.queries'

/**
 * An object URL for a message's file, fetched through the API with the workspace header.
 * `enabled` false waits (a file still loading from WhatsApp, or a document until asked for).
 */
export function useMediaUrl(
  workspaceId: string,
  conversationId: string,
  messageId: string,
  enabled: boolean,
) {
  const query = useQuery({ ...mediaQueryOptions(workspaceId, conversationId, messageId), enabled })
  const [url, setUrl] = useState<string | null>(null)
  useEffect(() => {
    if (!query.data) return
    const next = URL.createObjectURL(query.data)
    setUrl(next)
    return () => {
      URL.revokeObjectURL(next)
      setUrl(null)
    }
  }, [query.data])
  return { url, blob: query.data, isLoading: enabled && query.isPending, isError: query.isError }
}
