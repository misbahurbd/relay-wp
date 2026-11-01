import type { BroadcastBody } from '@relay/api-client'
import { z } from 'zod'
import type { Template } from '#/features/templates'
import type { Broadcast, BroadcastForm, BroadcastStatus, Variable, VariableSource } from '../types'

export const broadcastSearchSchema = z.object({
  status: z
    .enum(['draft', 'scheduled', 'sending', 'completed', 'cancelled', 'failed'])
    .optional()
    .catch(undefined),
  q: z.string().trim().max(100).optional().catch(undefined),
})

/** Contacts passes a tag or a segment; Templates' "Use in broadcast" passes a template. */
export const newBroadcastSearchSchema = z.object({
  tag: z.uuid().optional().catch(undefined),
  segment: z.uuid().optional().catch(undefined),
  template: z.uuid().optional().catch(undefined),
})

export const STATUS_LABEL: Record<BroadcastStatus, string> = {
  draft: 'Draft',
  scheduled: 'Scheduled',
  sending: 'Sending',
  completed: 'Sent',
  cancelled: 'Cancelled',
  failed: 'Failed',
}

export const STATUS_TABS: { value?: BroadcastStatus; label: string }[] = [
  { label: 'All' },
  { value: 'draft', label: 'Drafts' },
  { value: 'scheduled', label: 'Scheduled' },
  { value: 'sending', label: 'Sending' },
  { value: 'completed', label: 'Sent' },
]

export const SOURCES: { value: VariableSource; label: string }[] = [
  { value: 'text', label: 'Text you type' },
  { value: 'name', label: 'Contact’s name' },
  { value: 'phone', label: 'Contact’s phone' },
  { value: 'attribute', label: 'Contact field' },
]

/** What a source shows in the preview, so the phone reads like a real message. */
export function sampleValue(v: Variable | undefined) {
  if (!v) return ''
  switch (v.source) {
    case 'text':
      return v.value
    case 'name':
      return 'Ayesha'
    case 'phone':
      return '+880 1711 000000'
    case 'attribute':
      return v.value ? `[${v.value}]` : ''
  }
}

/** Why recipients missed a broadcast, in words. Meta's own reasons pass through. */
export const FAILURE_LABEL: Record<string, string> = {
  opted_out: 'Opted out before it was sent',
  cancelled: 'Broadcast cancelled',
  template_unavailable: 'Template no longer approved',
  missing_variable: 'A variable had no value',
  no_channel: 'WhatsApp number couldn’t send',
  stalled: 'Stopped after a day without progress',
  segment_deleted: 'A segment in its audience was deleted',
}

const PLACEHOLDER = /\{\{(\d+)\}\}/g

/** How many {{n}} a template's body has, and whether its text header has {{1}}. */
export function templateVariables(t: Template | undefined) {
  if (!t) return { header: false, body: 0 }
  let body = 0
  for (const m of t.components.body.text.matchAll(PLACEHOLDER)) body = Math.max(body, Number(m[1]))
  const header = t.components.header.type === 'text' && /\{\{1\}\}/.test(t.components.header.text)
  return { header, body }
}

/** Replaces {{n}} with values[n-1], leaving unfilled ones visible. */
export function fill(text: string, values: string[]) {
  return text.replace(PLACEHOLDER, (all, n) => values[Number(n) - 1] || all)
}

const blank = (): Variable => ({ source: 'text', value: '', fallback: '' })

/** Keeps the variable rows in step with the chosen template. */
export function fitVariables(form: BroadcastForm, t: Template | undefined): BroadcastForm {
  const v = templateVariables(t)
  return {
    ...form,
    header: v.header ? (form.header ?? blank()) : undefined,
    body: Array.from({ length: v.body }, (_, i) => form.body[i] ?? blank()),
  }
}

export function emptyForm(tagIds: string[] = [], segmentIds: string[] = []): BroadcastForm {
  return {
    name: '',
    audience: {
      all: false,
      tag_ids: tagIds,
      segment_ids: segmentIds,
      skip_recent_marketing: true,
    },
    body: [],
  }
}

export function formFrom(b: Broadcast): BroadcastForm {
  return {
    name: b.name,
    number_id: b.number_id ?? undefined,
    template_id: b.template_id ?? undefined,
    audience: b.audience,
    header: b.variables.header,
    body: b.variables.body,
  }
}

export function toBody(f: BroadcastForm): BroadcastBody {
  return {
    name: f.name,
    number_id: f.number_id,
    template_id: f.template_id,
    audience: f.audience,
    variables: { header: f.header, body: f.body },
  }
}

/** Nobody is picked: no "everyone", tag, list or segment. */
export const audienceEmpty = (a: BroadcastForm['audience']) =>
  !a.all && a.tag_ids.length === 0 && a.segment_ids.length === 0

/** Percent of whole, rounded; 0 when there is nothing to divide. */
export function percent(part: number, whole: number) {
  return whole > 0 ? Math.round((part / whole) * 100) : 0
}
