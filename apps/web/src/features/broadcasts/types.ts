import type {
  BroadcastDto,
  BroadcastSummaryDto,
  FailureDto,
  RecipientDto,
  VariableDto,
} from '@relay/api-client'

export type BroadcastStatus = BroadcastDto['status']
export type VariableSource = VariableDto['source']

export type Variable = { source: VariableSource; value: string; fallback: string }

/** The broadcast with the API's nullable lists turned into empty ones. */
export type Audience = {
  all: boolean
  tag_ids: string[]
  segment_ids: string[]
  skip_recent_marketing: boolean
}

export type Broadcast = Omit<BroadcastDto, 'audience' | 'variables' | 'failures'> & {
  audience: Audience
  variables: { header?: Variable; body: Variable[] }
  failures: FailureDto[]
}

export type BroadcastSummary = BroadcastSummaryDto
export type Recipient = RecipientDto
export type RecipientFilter = 'failed' | 'delivered' | 'read'

export type BroadcastFilter = { status?: BroadcastStatus; q?: string }

export type AudienceCount = {
  included: number
  left_out: number
  recent: number
  tier_limit: number
}

/** What the editor holds before it is saved. */
export type BroadcastForm = {
  name: string
  number_id?: string
  template_id?: string
  audience: Audience
  header?: Variable
  body: Variable[]
}

/** When it goes: now, or a local date and time in a time zone. */
export type Schedule = { when: 'now' | 'later'; date: string; time: string; zone: string }
