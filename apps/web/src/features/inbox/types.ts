import type {
  InboxCapabilitiesDto,
  InboxChannelDto,
  InboxContentDto,
  InboxConversationDetailDto,
  InboxConversationDto,
  InboxCountsDto,
  InboxEventDto,
  InboxMediaKindDto,
  InboxMentionDto,
  InboxMessageDto,
  InboxPersonDto,
  InboxQuickReplyDto,
  InboxReplyRefDto,
  InboxTemplateButtonDto,
  InboxTemplateDto,
  InboxUploadOutputBody,
} from '@relay/api-client'

// The API sends null for these pointers even where the generated types say otherwise; the
// queries normalise each DTO into these shapes so components never trip on a missing field.

export type Person = InboxPersonDto
export type Counts = InboxCountsDto
export type QuickReply = InboxQuickReplyDto
export type Upload = InboxUploadOutputBody
export type TemplateButton = InboxTemplateButtonDto

export type InboxTab = 'open' | 'pending' | 'closed' | 'mine'
export type InboxSort = 'newest' | 'waiting'
export type ConversationStatus = InboxConversationDto['status']

export type Conversation = Omit<InboxConversationDto, 'assignee' | 'team_id'> & {
  assignee: Person | null
  team_id: string | null
}

/** The channel account a chat is on; name and handle are "" once the account is gone. */
export type Channel = InboxChannelDto
export type ChannelType = Channel['type']

export type MediaKind = Omit<InboxMediaKindDto, 'mime_types'> & {
  /** Empty accepts any type the inbox can send. */
  mime_types: string[]
}

/** What a chat's channel allows (ADR 0010); every channel rule in the UI reads these. */
export type Capabilities = Omit<InboxCapabilitiesDto, 'media'> & { media: MediaKind[] }

export type ConversationDetail = Omit<
  InboxConversationDetailDto,
  'assignee' | 'team_id' | 'capabilities'
> & {
  assignee: Person | null
  team_id: string | null
  capabilities: Capabilities
}

export type ConversationPage = {
  items: Conversation[]
  next: string | null
  counts: Counts
}

export type SystemEvent = Omit<InboxEventDto, 'actor_id' | 'target_id'> & {
  actor_id: string | null
  target_id: string | null
}

export type MessageContent = Omit<InboxContentDto, 'event'> & { event?: SystemEvent }

export type Message = Omit<
  InboxMessageDto,
  'author' | 'reply_to' | 'mentions' | 'content' | 'reactions'
> & {
  author: Person | null
  reply_to: InboxReplyRefDto | null
  mentions: string[]
  content: MessageContent
  reactions: { in: string; out: string }
}

export type MessagePage = { items: Message[]; before: string | null }

export type InboxTemplate = Omit<InboxTemplateDto, 'buttons' | 'url_buttons'> & {
  buttons: TemplateButton[]
  url_buttons: number[]
}

export type Mention = Omit<InboxMentionDto, 'author'> & { author: Person | null }

/** An active member who can be assigned a chat or mentioned in a note. */
export type Member = { id: string; name: string; role: string }

/** A live-update signal from GET /inbox/events (AD-8): never content, only what to refetch. */
export type LiveEvent = {
  ws: string
  kind: 'message' | 'status' | 'conversation' | 'note' | 'mention'
  conversation_id: string
  team_id: string | null
  assignee_id: string | null
  channel_type: ChannelType
  channel_id: string
  user_id?: string | null
}
