import type { CreateInputBody1 } from '@relay/api-client'
import { z } from 'zod'
import type {
  ButtonType,
  HeaderSample,
  HeaderType,
  StatusFilter,
  Template,
  TemplateCategory,
  TemplateFormValues,
  TemplateLanguage,
  TemplateStatus,
} from '../types'

// ---------- options shared by the list and the form ----------

/** The languages the API accepts (AD-6). Synced templates may use others; show their code. */
export const LANGUAGES: { value: TemplateLanguage; label: string }[] = [
  { value: 'en_US', label: 'English (US)' },
  { value: 'en_GB', label: 'English (UK)' },
  { value: 'es', label: 'Spanish' },
  { value: 'pt_BR', label: 'Portuguese (BR)' },
  { value: 'hi', label: 'Hindi' },
  { value: 'bn', label: 'Bengali' },
  { value: 'ar', label: 'Arabic' },
]

export function languageLabel(code: string) {
  return LANGUAGES.find((l) => l.value === code)?.label ?? code
}

export const CATEGORIES: { value: TemplateCategory; label: string; hint: string }[] = [
  { value: 'marketing', label: 'Marketing', hint: 'Offers, sales, announcements' },
  { value: 'utility', label: 'Utility', hint: 'Order and account updates' },
  { value: 'authentication', label: 'Authentication', hint: 'One-time passcodes' },
]

export function categoryLabel(category: TemplateCategory) {
  return CATEGORIES.find((c) => c.value === category)?.label ?? category
}

export const STATUS_LABEL: Record<TemplateStatus, string> = {
  draft: 'Draft',
  pending: 'Pending',
  approved: 'Approved',
  rejected: 'Rejected',
  paused: 'Paused',
  disabled: 'Disabled',
}

export const STATUS_TABS: { value: StatusFilter | undefined; label: string }[] = [
  { value: undefined, label: 'All' },
  { value: 'approved', label: 'Approved' },
  { value: 'pending', label: 'Pending' },
  { value: 'rejected', label: 'Rejected' },
]

/** Meta is reviewing it, Meta stopped it, or it's an authentication template (AD-11): look, don't touch. */
export function isFrozen(t: Pick<Template, 'status' | 'category'>) {
  return (
    t.status === 'pending' ||
    t.status === 'paused' ||
    t.status === 'disabled' ||
    t.category === 'authentication'
  )
}

export const HEADER_TYPES: { value: HeaderType; label: string }[] = [
  { value: 'none', label: 'None' },
  { value: 'text', label: 'Text' },
  { value: 'image', label: 'Image' },
  { value: 'video', label: 'Video' },
  { value: 'document', label: 'Document' },
]

export type MediaHeaderType = Exclude<HeaderType, 'none' | 'text'>

/** What the header sample upload accepts per media type (AD-8). */
export const MEDIA: Record<
  MediaHeaderType,
  { accept: string; types: HeaderSample['content_type'][]; maxBytes: number; hint: string }
> = {
  image: {
    accept: 'image/jpeg,image/png',
    types: ['image/jpeg', 'image/png'],
    maxBytes: 5 * 1024 * 1024,
    hint: 'JPG or PNG, up to 5 MB',
  },
  video: {
    accept: 'video/mp4',
    types: ['video/mp4'],
    maxBytes: 16 * 1024 * 1024,
    hint: 'MP4, up to 16 MB',
  },
  document: {
    accept: 'application/pdf',
    types: ['application/pdf'],
    maxBytes: 100 * 1024 * 1024,
    hint: 'PDF, up to 100 MB',
  },
}

export function isMediaHeader(type: HeaderType): type is MediaHeaderType {
  return type === 'image' || type === 'video' || type === 'document'
}

/** Which header type an uploaded sample fits, from its content type. */
export function sampleKind(contentType: string | undefined): MediaHeaderType | null {
  for (const [kind, m] of Object.entries(MEDIA) as [MediaHeaderType, (typeof MEDIA)['image']][]) {
    if (m.types.some((t) => t === contentType)) return kind
  }
  return null
}

/** Why a picked file can't be the sample for this header type, or undefined when it can. */
export function sampleFileProblem(type: MediaHeaderType, file: File) {
  const m = MEDIA[type]
  if (!m.types.some((t) => t === file.type)) return `Choose a file of this type: ${m.hint}`
  if (file.size > m.maxBytes) return `This file is too large. ${m.hint}`
  return undefined
}

export const BUTTON_TYPES: Record<
  ButtonType,
  { label: string; add: string; valueLabel: string; placeholder: string }
> = {
  quick_reply: { label: 'Quick reply', add: 'Quick reply', valueLabel: '', placeholder: '' },
  url: {
    label: 'Visit website',
    add: 'Visit website',
    valueLabel: 'Website URL',
    placeholder: 'https://example.com/track',
  },
  phone: {
    label: 'Call phone',
    add: 'Call phone number',
    valueLabel: 'Phone number with country code',
    placeholder: '+14155550142',
  },
}

// ---------- AD-7 rules (mirrors apps/api/internal/features/templates/service/validation.go) ----------

export const LIMITS = {
  name: 512,
  headerText: 60,
  body: 1024,
  footer: 60,
  buttons: 10,
  urlButtons: 2,
  phoneButtons: 1,
  buttonText: 25,
} as const

const NAME_RE = /^[a-z0-9_]{1,512}$/
const VARIABLE_RE = /\{\{([^{}]*)\}\}/g
const INDEX_RE = /^[1-9][0-9]*$/
const E164_RE = /^\+[1-9][0-9]{6,14}$/
const SHORT_LINK_HOSTS = ['bit.ly', 'tinyurl.com', 't.co', 'goo.gl']

/** Characters as people count them (code points, like the server), not UTF-16 units. */
export function charCount(s: string) {
  return [...s].length
}

/** Turns typed text into a valid name: lowercase, spaces to underscores, nothing else. */
export function toTemplateName(s: string) {
  return s
    .toLowerCase()
    .replace(/[\s-]+/g, '_')
    .replace(/[^a-z0-9_]/g, '')
}

/** Drops the spaces, dashes and brackets people type in phone numbers. */
export function compactPhone(s: string) {
  return s.replace(/[\s\-().]/g, '')
}

function variables(text: string) {
  return [...text.matchAll(VARIABLE_RE)]
}

/** The distinct, well-formed variable numbers in the text, ascending. */
export function variableNumbers(text: string) {
  const nums = new Set<number>()
  for (const m of variables(text)) if (INDEX_RE.test(m[1])) nums.add(Number(m[1]))
  return [...nums].sort((a, b) => a - b)
}

/** How many sample values the text needs: one per number up to the highest variable. */
export function exampleSlots(text: string) {
  const nums = variableNumbers(text)
  return nums.length ? nums[nums.length - 1] : 0
}

/** Keeps the samples already typed and grows or shrinks the list to fit the text. */
export function resizeExamples(examples: string[], text: string) {
  return Array.from({ length: exampleSlots(text) }, (_, i) => examples[i] ?? '')
}

/** The body's variable count N, or why they aren't {{1}}..{{N}}. */
export function bodyVariables(text: string): { count: number; error?: string } {
  const seen = new Set<number>()
  for (const m of variables(text)) {
    if (!INDEX_RE.test(m[1]))
      return { count: 0, error: 'Variables look like {{1}}, {{2}} and so on' }
    seen.add(Number(m[1]))
  }
  for (let i = 1; i <= seen.size; i++) {
    if (!seen.has(i)) return { count: 0, error: 'Number the variables from {{1}} with no gaps' }
  }
  return { count: seen.size }
}

function isHttps(s: string) {
  if (/\s/.test(s)) return false
  try {
    const u = new URL(s)
    return u.protocol === 'https:' && u.host !== ''
  } catch {
    return false
  }
}

export type TemplateIssue = { field: string; message: string }

export type CheckOptions = {
  /** Submit adds the completeness rules; a draft is only checked for format. */
  complete?: boolean
  /** The uploaded header sample, needed for a media header on submit. */
  sample?: HeaderSample | null
}

/** Every AD-7 problem with the form, located like the server's details ("buttons[1].value"). */
export function templateIssues(v: TemplateFormValues, opts: CheckOptions = {}): TemplateIssue[] {
  const out: TemplateIssue[] = []
  const add = (field: string, message: string) => out.push({ field, message })
  const complete = !!opts.complete

  if (!NAME_RE.test(v.name.trim())) {
    add('name', `Use 1 to ${LIMITS.name} lowercase letters, numbers and underscores`)
  }
  if (!LANGUAGES.some((l) => l.value === v.language)) {
    add('language', 'Choose one of the supported languages')
  }
  if (v.category === 'authentication') add('category', 'Authentication templates are coming soon')

  // Header
  const h = v.header
  if (h.type === 'text') {
    const text = h.text.trim()
    if (charCount(text) > LIMITS.headerText) {
      add('header.text', `Keep the header to ${LIMITS.headerText} characters`)
    }
    const vars = variables(text)
    if (vars.length > 1) add('header.text', 'The header can have at most one variable')
    else if (vars.length === 1 && vars[0][1] !== '1') {
      add('header.text', 'The header’s variable must be {{1}}')
    }
    if (complete && text === '') add('header.text', 'Enter the header text')
    if (complete && vars.length === 1 && h.example.trim() === '') {
      add('header.example', 'Give an example for the header’s variable')
    }
  } else if (
    isMediaHeader(h.type) &&
    complete &&
    sampleKind(opts.sample?.content_type) !== h.type
  ) {
    add('header', `Upload a sample ${h.type} for the header`)
  }

  // Body
  const body = v.body.text.trim()
  if (charCount(body) > LIMITS.body) add('body.text', `Keep the body to ${LIMITS.body} characters`)
  if (complete && body === '') add('body.text', 'Enter the message text')
  const { count, error } = bodyVariables(body)
  if (error) add('body.text', error)
  else if (count > 0) {
    const vars = variables(body)
    const last = vars[vars.length - 1]
    if (vars[0].index === 0 || (last.index ?? 0) + last[0].length === body.length) {
      add('body.text', 'The body can’t start or end with a variable')
    }
  }
  if (complete && !error) {
    const examples = v.body.examples.slice(0, count)
    if (examples.length !== count) {
      add('body.examples', `Give one example for each of the ${count} variables`)
    } else {
      examples.forEach((e, i) => {
        if (e.trim() === '') add(`body.examples[${i}]`, 'Enter an example')
      })
    }
  }

  // Footer
  const footer = v.footer.text.trim()
  if (charCount(footer) > LIMITS.footer) {
    add('footer.text', `Keep the footer to ${LIMITS.footer} characters`)
  }
  if (footer.includes('{{')) add('footer.text', 'The footer can’t have variables')

  // Buttons
  if (v.buttons.length > LIMITS.buttons) add('buttons', `Add at most ${LIMITS.buttons} buttons`)
  let urls = 0
  let phones = 0
  let lastQuick = -1
  let grouped = true
  v.buttons.forEach((b, i) => {
    const at = `buttons[${i}]`
    const text = b.text.trim()
    const value = b.type === 'phone' ? compactPhone(b.value.trim()) : b.value.trim()
    if (b.type === 'quick_reply') {
      if (lastQuick >= 0 && lastQuick !== i - 1) grouped = false
      lastQuick = i
    } else if (b.type === 'url') {
      urls++
      if (value !== '' && !isHttps(value))
        add(`${at}.value`, 'Use a link that starts with https://')
    } else {
      phones++
      if (value !== '' && !E164_RE.test(value)) {
        add(`${at}.value`, 'Use the full number with its country code, like +14155550142')
      }
    }
    if (b.type !== 'quick_reply' && complete && value === '') {
      add(`${at}.value`, 'Enter the button’s link or number')
    }
    if (complete && text === '') add(`${at}.text`, 'Enter the button text')
    if (charCount(text) > LIMITS.buttonText) {
      add(`${at}.text`, `Keep button text to ${LIMITS.buttonText} characters`)
    }
  })
  if (urls > LIMITS.urlButtons) add('buttons', `Add at most ${LIMITS.urlButtons} link buttons`)
  if (phones > LIMITS.phoneButtons) add('buttons', 'Add at most one phone button')
  if (!grouped) add('buttons', 'Keep the quick replies together')

  return out
}

/** "buttons[1].value" → ["buttons", 1, "value"]. */
export function fieldPath(field: string): (string | number)[] {
  return field
    .split(/\.|\[(\d+)\]/)
    .filter((s) => s !== undefined && s !== '')
    .map((s) => (/^\d+$/.test(s) ? Number(s) : s))
}

const formShape = z.object({
  name: z.string(),
  language: z.string(),
  category: z.enum(['marketing', 'utility', 'authentication']),
  header: z.object({
    type: z.enum(['none', 'text', 'image', 'video', 'document']),
    text: z.string(),
    example: z.string(),
  }),
  body: z.object({ text: z.string(), examples: z.array(z.string()) }),
  footer: z.object({ text: z.string() }),
  buttons: z.array(
    z.object({
      type: z.enum(['quick_reply', 'url', 'phone']),
      text: z.string(),
      value: z.string(),
    }),
  ),
})

/** The AD-7 rules as a zod schema. `complete` adds what only a submit needs. */
export function templateSchema(opts: CheckOptions = {}) {
  return formShape.superRefine((v, ctx) => {
    for (const issue of templateIssues(v, opts)) {
      ctx.addIssue({ code: 'custom', path: fieldPath(issue.field), message: issue.message })
    }
  })
}

/** What a draft must pass to be saved: format only. */
export const templateDraftSchema = templateSchema()

// ---------- warnings (don't block saving) ----------

function shortLinkIn(words: string[]) {
  for (const word of words) {
    const w = word
      .toLowerCase()
      .replace(/^[.,;:!?()[\]"'<>]+|[.,;:!?()[\]"'<>]+$/g, '')
      .replace(/^https?:\/\//, '')
    const host = w.split('/')[0].replace(/^www\./, '')
    if (SHORT_LINK_HOSTS.includes(host)) return host
  }
  return undefined
}

function shortLinkMessage(host: string) {
  return `Meta often rejects templates with ${host} links. Use the full link instead.`
}

/** Short links Meta tends to reject, keyed by field. A warning, never an error (AD-7). */
export function shortLinkWarnings(v: TemplateFormValues): Record<string, string> {
  const out: Record<string, string> = {}
  const inBody = shortLinkIn(v.body.text.split(/\s+/).filter(Boolean))
  if (inBody) out['body.text'] = shortLinkMessage(inBody)
  v.buttons.forEach((b, i) => {
    if (b.type !== 'url') return
    const host = shortLinkIn([b.value.trim()])
    if (host) out[`buttons[${i}].value`] = shortLinkMessage(host)
  })
  return out
}

// ---------- converting between the form and the API ----------

export function emptyTemplateForm(): TemplateFormValues {
  return {
    name: '',
    language: 'en_US',
    category: 'marketing',
    header: { type: 'none', text: '', example: '' },
    body: { text: '', examples: [] },
    footer: { text: '' },
    buttons: [],
  }
}

export function templateToForm(t: Template): TemplateFormValues {
  const c = t.components
  return {
    name: t.name,
    language: t.language,
    category: t.category,
    header: { type: c.header.type, text: c.header.text, example: c.header.example },
    body: { text: c.body.text, examples: resizeExamples(c.body.examples, c.body.text) },
    footer: { text: c.footer.text },
    buttons: c.buttons.map((b) => ({ type: b.type, text: b.text, value: b.value })),
  }
}

/** The create/update body: our components shape (AD-6), trimmed, with unused parts emptied. */
export function formToInput(v: TemplateFormValues): CreateInputBody1 {
  const h = v.header
  const headerText = h.text.trim()
  const body = v.body.text.trim()
  return {
    name: v.name.trim(),
    language: v.language,
    category: v.category,
    components: {
      header:
        h.type === 'text'
          ? {
              type: 'text',
              text: headerText,
              example: variables(headerText).length ? h.example.trim() : '',
            }
          : { type: h.type, text: '', example: '' },
      body: {
        text: body,
        examples: v.body.examples.slice(0, bodyVariables(body).count).map((e) => e.trim()),
      },
      footer: { text: v.footer.text.trim() },
      buttons: v.buttons.map((b) => ({
        type: b.type,
        text: b.text.trim(),
        value:
          b.type === 'quick_reply'
            ? ''
            : b.type === 'phone'
              ? compactPhone(b.value.trim())
              : b.value.trim(),
      })),
    },
  }
}

// ---------- list search params ----------

export const templateSearchSchema = z.object({
  status: z.enum(['approved', 'pending', 'rejected']).optional().catch(undefined),
  category: z.enum(['marketing', 'utility', 'authentication']).optional().catch(undefined),
  q: z.string().trim().max(200).optional().catch(undefined),
})
