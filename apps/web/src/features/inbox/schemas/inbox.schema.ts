import { z } from 'zod'
import type { Channel, ChannelType, InboxTemplate, MediaKind } from '../types'

export const INBOX_TABS = ['open', 'pending', 'closed', 'mine'] as const
export const INBOX_SORTS = ['newest', 'waiting'] as const

/** Bad values in a shared link fall back to the defaults instead of an error page. */
export const inboxSearchSchema = z.object({
  c: z.uuid().optional().catch(undefined),
  tab: z.enum(INBOX_TABS).optional().catch(undefined),
  q: z.string().trim().max(200).optional().catch(undefined),
  sort: z.enum(INBOX_SORTS).optional().catch(undefined),
})

export type InboxSearch = z.infer<typeof inboxSearchSchema>

export const TAB_LABEL = {
  open: 'Open',
  pending: 'Pending',
  closed: 'Closed',
  mine: 'Assigned to me',
} as const

export const STATUS_LABEL = { open: 'Open', pending: 'Pending', closed: 'Closed' } as const

/**
 * Meta's per-template charge, as a hint only (AD-6): we never bill it. Lowest list prices;
 * update when Meta's rate card changes.
 */
export const TEMPLATE_PRICE: Record<InboxTemplate['category'], string> = {
  utility: '$0.004',
  authentication: '$0.004',
  marketing: '$0.025',
}

export const CATEGORY_LABEL: Record<InboxTemplate['category'], string> = {
  utility: 'Utility',
  authentication: 'Authentication',
  marketing: 'Marketing',
}

/** Category label for a stored template message, whose category is a plain string. */
export function categoryLabel(category: string) {
  return CATEGORY_LABEL[category as InboxTemplate['category']] ?? category
}

export function templatePrice(category: string) {
  return TEMPLATE_PRICE[category as InboxTemplate['category']] ?? TEMPLATE_PRICE.utility
}

export const REACTIONS = [
  { emoji: '👍', label: 'thumbs up' },
  { emoji: '❤️', label: 'heart' },
  { emoji: '😂', label: 'laughing' },
  { emoji: '😮', label: 'surprised' },
  { emoji: '🙏', label: 'thank you' },
] as const

export const EMOJI = [
  '😀',
  '😊',
  '😂',
  '😍',
  '🙏',
  '👍',
  '👌',
  '🎉',
  '❤️',
  '🔥',
  '😢',
  '😮',
  '🤔',
  '👋',
  '✅',
  '📦',
  '🚚',
  '💳',
  '🛍️',
  '⭐',
  '🎁',
  '📍',
  '⏰',
  '💬',
] as const

/**
 * What differs by channel beyond its capabilities. Meta's per-template pricing hints are
 * WhatsApp's rate card, so they show only where metaPricing is on.
 */
export const CHANNEL_COPY: Record<ChannelType, { label: string; metaPricing: boolean }> = {
  whatsapp: { label: 'WhatsApp', metaPricing: true },
  messenger: { label: 'Messenger', metaPricing: false },
  instagram: { label: 'Instagram', metaPricing: false },
}

export const DISCONNECTED_CHANNEL = 'Disconnected channel'

/** The channel's handle (or name), or "Disconnected channel" once its account is gone. */
export function channelHandle(channel: Channel) {
  return channel.handle || channel.name || DISCONNECTED_CHANNEL
}

/** The channel's name and handle together, or "Disconnected channel". */
export function channelAccount(channel: Channel) {
  return [channel.name, channel.handle].filter(Boolean).join(' · ') || DISCONNECTED_CHANNEL
}

/** One entry of the attach menu: the media kinds its file picker offers. */
export type AttachOption = { label: string; kinds: MediaKind[]; accept: string | undefined }

const KIND_ANY: Record<MediaKind['kind'], string> = {
  image: 'image/*',
  video: 'video/*',
  audio: 'audio/*',
  // The inbox stores these document types; the channel narrows them further if it lists any.
  document: '.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt',
}

const KIND_NOUN: Record<MediaKind['kind'], string> = {
  image: 'Photos',
  video: 'Videos',
  audio: 'Audio files',
  document: 'Documents',
}

/** The attach menu for a channel: only the kinds it can send (photos and videos share a picker). */
export function attachOptions(media: MediaKind[]): AttachOption[] {
  const pick = (...kinds: MediaKind['kind'][]) => media.filter((m) => kinds.includes(m.kind))
  const accept = (kinds: MediaKind[]) =>
    kinds
      .map((k) => (k.mime_types.length > 0 ? k.mime_types.join(',') : KIND_ANY[k.kind]))
      .join(',')
  const visual = pick('image', 'video')
  const options = [
    {
      label:
        visual.length === 2 ? 'Photo or video' : visual[0]?.kind === 'video' ? 'Video' : 'Photo',
      kinds: visual,
    },
    { label: 'Audio', kinds: pick('audio') },
    { label: 'Document', kinds: pick('document') },
  ]
  return options
    .filter((o) => o.kinds.length > 0)
    .map((o) => ({ ...o, accept: accept(o.kinds) || undefined }))
}

function kindOf(file: File, kinds: MediaKind[]) {
  const fits = (k: MediaKind) =>
    k.mime_types.length > 0
      ? k.mime_types.includes(file.type)
      : k.kind === 'document' || file.type.startsWith(`${k.kind}/`)
  return kinds.find(fits)
}

/** "JPEG or PNG" from image/jpeg and image/png. */
function typeList(types: string[]) {
  const names = types.map((t) => (t.split('/')[1] ?? t).replace(/^x-/, '').toUpperCase())
  return names.length > 1 ? `${names.slice(0, -1).join(', ')} or ${names.at(-1)}` : names[0]
}

/** Why a file can't be sent through this attach option, or null when it can (AD-11). */
export function uploadProblem(file: File, option: AttachOption): string | null {
  const kind = kindOf(file, option.kinds)
  if (!kind) {
    const wanted = option.kinds
      .filter((k) => k.mime_types.length > 0)
      .map((k) => `${KIND_NOUN[k.kind]} must be ${typeList(k.mime_types)}.`)
    return wanted.length > 0 ? wanted.join(' ') : 'This type of file can’t be sent on this channel.'
  }
  return file.size > kind.max_bytes
    ? `${KIND_NOUN[kind.kind]} can be up to ${formatLimit(kind.max_bytes)}.`
    : null
}

function formatLimit(bytes: number) {
  return bytes >= 1 << 20 ? `${Math.round(bytes / (1 << 20))} MB` : `${Math.round(bytes / 1024)} KB`
}

export const quickReplySchema = z.object({
  shortcut: z
    .string()
    .trim()
    .transform((s) => s.replace(/^\//, '').toLowerCase())
    .pipe(z.string().regex(/^[a-z0-9_-]{1,32}$/, 'Use 1 to 32 lowercase letters, numbers, - or _')),
  title: z.string().trim().min(1, 'Give it a short title').max(80, 'Use at most 80 characters'),
  body: z.string().trim().min(1, 'Write the reply').max(1024, 'Use at most 1,024 characters'),
})

export type QuickReplyValues = z.input<typeof quickReplySchema>
