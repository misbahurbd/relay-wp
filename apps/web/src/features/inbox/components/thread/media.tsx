import { DownloadIcon, Loader2Icon } from 'lucide-react'
import { useEffect, useState } from 'react'
import { Button } from '#/components/ui/button'
import { useMediaUrl } from '../../hooks/use-media-url'
import type { Message } from '../../types'
import { formatSize } from '../kit/format'

type Props = { workspaceId: string; conversationId: string; message: Message }

/** A file still on its way from WhatsApp (AD-11): the worker stores it, then the thread refetches. */
function MediaLoading({ label = 'Media is loading' }: { label?: string }) {
  return (
    <div
      role="status"
      className="flex h-32 w-60 max-w-full items-center justify-center gap-2 rounded-md bg-muted text-[13px] text-muted-foreground"
    >
      <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
      {label}
    </div>
  )
}

function MediaFailed() {
  return (
    <div className="flex h-20 w-60 max-w-full items-center justify-center rounded-md bg-muted text-[13px] text-muted-foreground">
      We couldn’t load this file.
    </div>
  )
}

export function MediaImage({
  workspaceId,
  conversationId,
  message,
  sticker,
}: Props & { sticker?: boolean }) {
  const ready = !!message.content.media?.ready
  const { url, isError } = useMediaUrl(workspaceId, conversationId, message.id, ready)
  if (!ready) return <MediaLoading />
  if (isError) return <MediaFailed />
  if (!url) return <MediaLoading label="Loading photo" />
  const alt = message.body || (sticker ? 'Sticker' : 'Photo')
  return sticker ? (
    <img src={url} alt={alt} className="size-32 object-contain" />
  ) : (
    <a href={url} target="_blank" rel="noreferrer" aria-label="Open photo in a new tab">
      <img src={url} alt={alt} className="max-h-72 w-auto max-w-full rounded-md object-cover" />
    </a>
  )
}

export function MediaPlayer({
  workspaceId,
  conversationId,
  message,
  kind,
}: Props & { kind: 'video' | 'audio' }) {
  const ready = !!message.content.media?.ready
  const { url, isError } = useMediaUrl(workspaceId, conversationId, message.id, ready)
  if (!ready) return <MediaLoading />
  if (isError) return <MediaFailed />
  if (!url) return <MediaLoading label={kind === 'video' ? 'Loading video' : 'Loading audio'} />
  return kind === 'video' ? (
    // biome-ignore lint/a11y/useMediaCaption: customer videos come without captions
    <video src={url} controls preload="metadata" className="max-h-72 max-w-full rounded-md" />
  ) : (
    // biome-ignore lint/a11y/useMediaCaption: voice notes come without captions
    <audio src={url} controls preload="metadata" className="w-64 max-w-full" />
  )
}

function extension(filename: string, mime: string) {
  const ext = filename.includes('.') ? filename.split('.').pop() : mime.split('/').pop()
  return (ext ?? 'file').slice(0, 4).toUpperCase()
}

/** A document card; the file downloads only when asked for (documents can be 100 MB). */
export function MediaDocument({ workspaceId, conversationId, message }: Props) {
  const media = message.content.media
  const ready = !!media?.ready
  const [wanted, setWanted] = useState(false)
  const { url, isLoading, isError } = useMediaUrl(
    workspaceId,
    conversationId,
    message.id,
    ready && wanted,
  )
  const filename = media?.filename || 'Document'

  // Save the file as soon as it arrives after a click.
  useEffect(() => {
    if (!wanted || !url) return
    const a = document.createElement('a')
    a.href = url
    a.download = filename
    a.click()
    setWanted(false)
  }, [wanted, url, filename])

  return (
    <div className="flex w-64 max-w-full items-center gap-2.5 rounded-md bg-card/70 p-2.5">
      <span className="flex h-10 w-9 flex-none items-center justify-center rounded-sm bg-destructive-soft text-[10px] font-extrabold text-destructive">
        {extension(filename, media?.mime ?? '')}
      </span>
      <span className="flex min-w-0 flex-1 flex-col leading-tight">
        <span className="truncate text-[13px] font-bold">{filename}</span>
        <span className="text-xs text-muted-foreground">
          {ready ? formatSize(media?.size ?? 0) : 'Media is loading'}
          {isError && ' · download failed'}
        </span>
      </span>
      <Button
        variant="unstyled"
        type="button"
        disabled={!ready || isLoading}
        onClick={() => setWanted(true)}
        aria-label={`Download ${filename}`}
        className="flex size-9 flex-none items-center justify-center rounded-md text-muted-foreground hover:bg-muted disabled:opacity-50"
      >
        {isLoading ? (
          <Loader2Icon className="size-4 animate-spin" aria-hidden="true" />
        ) : (
          <DownloadIcon className="size-4" aria-hidden="true" />
        )}
      </Button>
    </div>
  )
}
