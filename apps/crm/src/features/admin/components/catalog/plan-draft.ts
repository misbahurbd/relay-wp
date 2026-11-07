import type { CreatePlanInputBody } from '@relay/api-client'
import type { Cadence, PublishStep } from '../../api/plan-publish'
import type { Limits, Plan, Provider, ProviderCaps, Resource } from '../../types'
import {
  capError,
  FREE_PLAN,
  isIncluded,
  limitsOf,
  planEntries,
  type ResourceEntries,
} from '../kit/limits'
import { toMinor } from '../kit/money'
import { extrasOf, planLines } from '../kit/plan-highlights'

/** Add a plan, publish the next version of one, or edit one version in place. */
export type PlanEditorMode =
  | { kind: 'create' }
  | { kind: 'version'; from: Plan }
  | { kind: 'edit'; plan: Plan }

export type StepId = 'basics' | 'included' | 'pricing' | 'trial' | 'review'

export const CURRENCY = 'USD'
export const MAX_TRIAL_DAYS = 90
const MAX_FEATURES = 20
const MAX_FEATURE_LENGTH = 80

export const KNOWN_PROVIDERS: Provider[] = ['stripe', 'creem', 'paypal', 'sslcommerz']

export const CADENCES: { id: Cadence; label: string }[] = [
  { id: 'month', label: 'Monthly' },
  { id: 'year', label: 'Yearly' },
  { id: 'lifetime', label: 'Lifetime' },
]

export type PlanDraft = {
  name: string
  code: string
  /** Once staff edit the code it stops following the name. */
  codeEdited: boolean
  description: string
  recommended: boolean
  sortOrder: string
  entries: ResourceEntries
  /** Extra highlights, one per line: what limits can't say. The limits list themselves. */
  features: string
  /** Dollars as typed, by cadence; empty means no such price. */
  prices: Record<Cadence, string>
  sell: boolean
  trialDays: string
  requiresCard: boolean
  reason: string
  /** Sent with the new plan, so a retried publish finds the plan it already created. */
  publishKey: string
}

export type Errors = Record<string, string>

export function sourceOf(mode: PlanEditorMode) {
  return mode.kind === 'create' ? undefined : mode.kind === 'version' ? mode.from : mode.plan
}

/** The steps of a mode: an edit has no prices to enter, and the free plan has no card or trial. */
export function stepsOf(mode: PlanEditorMode): StepId[] {
  const free = sourceOf(mode)?.code === FREE_PLAN
  if (free) return ['basics', 'included', 'review']
  return mode.kind === 'edit'
    ? ['basics', 'included', 'trial', 'review']
    : ['basics', 'included', 'pricing', 'review']
}

export function slugify(text: string) {
  return text
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '')
}

export function initialDraft(
  mode: PlanEditorMode,
  resources: Resource[],
  sellable: boolean,
): PlanDraft {
  const source = sourceOf(mode)
  return {
    name: source?.name ?? '',
    code: source?.code ?? '',
    codeEdited: false,
    description: source?.description ?? '',
    recommended: source?.recommended ?? false,
    sortOrder: String(source?.sort_order ?? 0),
    entries: planEntries(resources, source?.limits),
    features: (source?.features ?? []).join('\n'),
    prices: { month: '', year: '', lifetime: '' },
    sell: sellable,
    trialDays: String(source?.trial_days ?? 0),
    requiresCard: source?.trial_requires_card ?? false,
    reason: '',
    publishKey: crypto.randomUUID(),
  }
}

export function draftLimits(draft: PlanDraft): Limits {
  return limitsOf(draft.entries, false)
}

export function includedCount(resources: Resource[], entries: ResourceEntries) {
  return resources.filter((r) => isIncluded(entries[r.key])).length
}

/** The extra highlights staff typed, one per non-empty line. */
export function featureLines(draft: PlanDraft) {
  return draft.features
    .split('\n')
    .map((f) => f.trim())
    .filter(Boolean)
}

/** Extra highlights that only say again what a limit already says. The pricing card leaves them out. */
export function repeatedLines(draft: PlanDraft, resources: Resource[]) {
  const keep = extrasOf(featureLines(draft), planLines(resources, draftLimits(draft)))
  return featureLines(draft).length - keep.length
}

/** The extra highlights as text, without the lines that repeat a limit. */
export function withoutRepeats(draft: PlanDraft, resources: Resource[]) {
  return extrasOf(featureLines(draft), planLines(resources, draftLimits(draft))).join('\n')
}

/** Dollars typed into a price field as minor units, or undefined when the field is empty or not a price. */
export function minorOf(dollars: string) {
  const n = toMinor(dollars)
  return Number.isFinite(n) && n > 0 ? n : undefined
}

export function enteredCadences(draft: PlanDraft) {
  return CADENCES.filter((c) => draft.prices[c.id].trim() !== '')
}

export function nextVersionOf(code: string, plans: Plan[]) {
  const same = plans.filter((p) => p.code === code)
  return same.length ? Math.max(...same.map((p) => p.version)) + 1 : undefined
}

/** Whether a card-required trial can be offered, and which enabled providers can't run one. */
export function cardTrialSupport(enabled: string[], caps: ProviderCaps[] | undefined) {
  const without = enabled.filter((name) => caps?.some((c) => c.name === name && !c.checkoutTrials))
  return { allowed: enabled.length === 0 || without.length < enabled.length, without }
}

export function sellProviders(enabled: string[]) {
  return KNOWN_PROVIDERS.filter((p) => enabled.includes(p))
}

export function limitsChanged(mode: PlanEditorMode, resources: Resource[], draft: PlanDraft) {
  if (mode.kind !== 'edit') return false
  const now = draftLimits(draft)
  return resources.some((r) => (mode.plan.limits[r.key] ?? 0) !== (now[r.key] ?? 0))
}

export type Context = { mode: PlanEditorMode; resources: Resource[] }

function trialErrors(d: PlanDraft): Errors {
  const days = Number(d.trialDays)
  return !Number.isInteger(days) || days < 0 || days > MAX_TRIAL_DAYS
    ? { trialDays: `Use a whole number from 0 to ${MAX_TRIAL_DAYS}` }
    : {}
}

const ERRORS: Record<StepId, (d: PlanDraft, ctx: Context) => Errors> = {
  basics(d, { mode }) {
    const e: Errors = {}
    if (!d.name.trim()) e.name = 'Give the plan a name'
    if (mode.kind === 'create' && !/^[a-z0-9_]+$/.test(d.code)) {
      e.code = d.code ? 'Use lowercase letters, digits and _' : 'Add a code'
    }
    const sort = Number(d.sortOrder)
    if (!Number.isInteger(sort) || sort < 0) e.sortOrder = 'Use a whole number, 0 or more'
    return e
  },
  included(d, { resources }) {
    const e: Errors = {}
    for (const r of resources) {
      const entry = d.entries[r.key]
      if (entry?.mode !== 'cap') continue
      const msg = capError(entry.cap)
      if (msg) e[`cap:${r.key}`] = msg
    }
    return e
  },
  pricing(d) {
    const e: Errors = {}
    for (const c of enteredCadences(d)) {
      if (!minorOf(d.prices[c.id])) e[c.id] = 'Enter an amount above 0, like 29 or 29.99'
    }
    return { ...e, ...trialErrors(d) }
  },
  trial: trialErrors,
  review(d, { mode, resources }) {
    const e: Errors = {}
    const lines = featureLines(d)
    if (lines.length > MAX_FEATURES) e.features = `Keep it to ${MAX_FEATURES} lines`
    else if (lines.some((l) => l.length > MAX_FEATURE_LENGTH)) {
      e.features = `Keep each line to ${MAX_FEATURE_LENGTH} characters`
    }
    if (mode.kind === 'edit' && limitsChanged(mode, resources, d) && !d.reason.trim()) {
      e.reason = 'Say why the limits change'
    }
    return e
  },
}

export function stepErrors(step: StepId, draft: PlanDraft, ctx: Context): Errors {
  return ERRORS[step](draft, ctx)
}

export function planBody(draft: PlanDraft): CreatePlanInputBody {
  return {
    code: draft.code.trim(),
    name: draft.name.trim(),
    description: draft.description.trim(),
    features: featureLines(draft),
    recommended: draft.recommended,
    trial_days: Number(draft.trialDays) || 0,
    trial_requires_card: draft.requiresCard,
    limits: draftLimits(draft),
    sort_order: Number(draft.sortOrder) || 0,
  }
}

/** The calls a create or a new version makes, in order: the plan, each price, then each price on each provider. */
export function publishSteps(draft: PlanDraft, providers: Provider[]): PublishStep[] {
  const prices = enteredCadences(draft)
  return [
    { id: 'plan', kind: 'plan', body: { ...planBody(draft), publish_key: draft.publishKey } },
    ...prices.map((c): PublishStep => {
      const lifetime = c.id === 'lifetime'
      return {
        id: `price:${c.id}`,
        kind: 'price',
        cadence: c.id,
        body: {
          billing_type: lifetime ? 'one_time' : 'recurring',
          interval: lifetime ? undefined : c.id === 'year' ? 'year' : 'month',
          currency: CURRENCY,
          amount_minor: minorOf(draft.prices[c.id]) ?? 0,
        },
      }
    }),
    ...(draft.sell
      ? prices.flatMap((c) =>
          providers.map(
            (provider): PublishStep => ({
              id: `sale:${c.id}:${provider}`,
              kind: 'sale',
              cadence: c.id,
              provider,
            }),
          ),
        )
      : []),
  ]
}

const CADENCE_WORD: Record<Cadence, string> = {
  month: 'monthly',
  year: 'yearly',
  lifetime: 'lifetime',
}

/** What a publish step does, in words: "Add the monthly price ($29)". */
export function stepLabel(step: PublishStep, draft: PlanDraft, format: (minor: number) => string) {
  switch (step.kind) {
    case 'plan':
      return `Create the plan ${draft.code.trim()}`
    case 'price':
      return `Add the ${CADENCE_WORD[step.cadence]} price (${format(step.body.amount_minor)})`
    case 'sale':
      return `Put the ${CADENCE_WORD[step.cadence]} price on sale with ${step.provider}`
  }
}
