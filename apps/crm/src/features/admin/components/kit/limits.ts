import type { Limits, Resource } from '../../types'
import { planLines } from './plan-highlights'

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

/** "5 team seats · 1 WhatsApp number · Automations & drip campaigns", leaving out what isn't included. */
export function limitsSummary(resources: Resource[], limits: Limits) {
  return planLines(resources, limits).join(' · ') || 'Nothing included'
}

/** Every resource set to its widest: unlimited counts, features on. */
export function includeEverything(resources: Resource[]): ResourceEntries {
  return Object.fromEntries(
    resources.map((r) => [r.key, r.kind === 'access' ? { mode: 'on' } : { mode: 'unlimited' }]),
  )
}

/** Whether a row counts as part of the plan. */
export function isIncluded(e: ResourceEntry | undefined) {
  return !!e && e.mode !== 'off' && e.mode !== 'plan'
}

/** A limit someone typed: a whole number of at least 1, or a message saying what to enter. */
export function capError(cap: string) {
  const n = Number(cap)
  return cap.trim() !== '' && Number.isInteger(n) && n >= 1
    ? undefined
    : 'Enter a whole number, 1 or more'
}
