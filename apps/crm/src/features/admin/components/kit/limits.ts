import type { Limits, Resource } from '../../types'

/** The plan a workspace with no grant uses. It can be edited but not sold. */
export const FREE_PLAN = 'free'

const UNLIMITED = -1
const NOT_INCLUDED = 0
const INCLUDED = 1

/**
 * One resource row as staff edit it. `plan` only appears in a workspace's
 * overrides, where it means "whatever the plan includes".
 */
export type ResourceEntry =
  | { mode: 'plan' }
  | { mode: 'off' }
  | { mode: 'on' }
  | { mode: 'unlimited' }
  | { mode: 'cap'; cap: string }

export type ResourceEntries = Record<string, ResourceEntry>

function entryOf(r: Resource, value: number): ResourceEntry {
  if (value === NOT_INCLUDED) return { mode: 'off' }
  if (r.kind === 'access') return { mode: 'on' }
  return value === UNLIMITED ? { mode: 'unlimited' } : { mode: 'cap', cap: String(value) }
}

/** Rows for a plan's limits: a resource the plan leaves out is not included. */
export function planEntries(resources: Resource[], limits?: Limits): ResourceEntries {
  return Object.fromEntries(
    resources.map((r) => {
      if (!limits) return [r.key, r.required ? { mode: 'cap', cap: '1' } : { mode: 'off' }]
      return [r.key, entryOf(r, limits[r.key] ?? NOT_INCLUDED)]
    }),
  )
}

/** Rows for a workspace's overrides: a key it doesn't override follows the plan. */
export function overrideEntries(resources: Resource[], overrides: Limits): ResourceEntries {
  return Object.fromEntries(
    resources.map((r) => {
      const v = overrides[r.key]
      return [r.key, v === undefined ? { mode: 'plan' } : entryOf(r, v)]
    }),
  )
}

function limitOf(e: ResourceEntry): number | undefined {
  switch (e.mode) {
    case 'plan':
      return undefined
    case 'off':
      return NOT_INCLUDED
    case 'on':
      return INCLUDED
    case 'unlimited':
      return UNLIMITED
    case 'cap':
      return Number(e.cap)
  }
}

/**
 * The API's limits from rows. A plan leaves out what it doesn't include;
 * overrides keep an explicit 0, since leaving it out would mean "the plan's".
 */
export function limitsOf(entries: ResourceEntries, keepOff: boolean): Limits {
  const out: Limits = {}
  for (const [key, e] of Object.entries(entries)) {
    const v = limitOf(e)
    if (v === undefined || (v === NOT_INCLUDED && !keepOff)) continue
    out[key] = v
  }
  return out
}

/** How a count or monthly resource reads in a sentence: "5,000 contacts", "30 broadcasts / month". */
function amount(r: Resource, value: number) {
  const n = value === UNLIMITED ? 'Unlimited' : value.toLocaleString()
  const unit = value === 1 ? r.unit : `${r.unit}s`
  return `${n} ${unit}${r.kind === 'monthly' ? ' / month' : ''}`
}

/** "5 team seats · 1 WhatsApp number · Automations & drip campaigns", leaving out what isn't included. */
export function limitsSummary(resources: Resource[], limits: Limits) {
  return pricingLines(resources, limits).join(' · ') || 'Nothing included'
}

/** Pricing table bullets for what a plan includes, in catalog order. */
export function pricingLines(resources: Resource[], limits: Limits) {
  return resources.flatMap((r) => {
    const v = limits[r.key] ?? NOT_INCLUDED
    if (v === NOT_INCLUDED) return []
    return [r.kind === 'access' ? r.label : amount(r, v)]
  })
}
