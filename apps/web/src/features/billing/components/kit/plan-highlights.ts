// Duplicated in apps/crm (features/admin/components/kit/plan-highlights.ts): the apps never import
// each other, and the CRM preview must read exactly like this card. Change both together.

const UNLIMITED = -1
const NOT_INCLUDED = 0

/** What the catalog says about one resource (GET /v1/billing/resources). */
export type PlanResource = {
  key: string
  kind: 'count' | 'monthly' | 'access'
  unit: string
  label: string
}

/** The parts of a plan the model reads. */
export type PlanInput = {
  name: string
  limits: Record<string, number>
  /** Staff-written lines that limits can't say. */
  features: string[]
}

export type PlanSummary = {
  /** Everything the plan includes, in catalog order. */
  lines: string[]
  /** "Everything in Starter, plus:". Only on a plan that has a cheaper one before it. */
  heading?: string
  /** What the plan adds over the previous one; all of `lines` on the first plan. */
  highlights: string[]
  /** The plan's own marketing lines, shown after the highlights. A line that repeats one of `lines` is left out. */
  extras: string[]
}

export type MatrixCell =
  | { kind: 'text'; text: string }
  | { kind: 'included' }
  | { kind: 'not_included' }

export type MatrixRow = {
  key: string
  label: string
  /** One cell per plan, in the order the plans were given. */
  cells: MatrixCell[]
}

export type MatrixGroup = { title: 'Limits' | 'Features'; rows: MatrixRow[] }

export type PlanModel = {
  /** One summary per plan, in the order the plans were given. */
  plans: PlanSummary[]
  matrix: MatrixGroup[]
}

function capOf(plan: PlanInput, r: PlanResource) {
  return plan.limits[r.key] ?? NOT_INCLUDED
}

/** "reply" → "replies", "contact" → "contacts". Catalog units are English nouns. */
export function plural(unit: string) {
  if (unit.endsWith('s')) return unit
  return /[^aeiou]y$/.test(unit) ? `${unit.slice(0, -1)}ies` : `${unit}s`
}

/** "5,000 contacts", "30 broadcasts / month", "Unlimited broadcasts", or a feature's label. */
export function lineOf(r: PlanResource, value: number) {
  if (r.kind === 'access') return r.label
  const n = value === UNLIMITED ? 'Unlimited' : value.toLocaleString()
  const unit = value === 1 ? r.unit : plural(r.unit)
  return `${n} ${unit}${r.kind === 'monthly' && value !== UNLIMITED ? ' / month' : ''}`
}

/** What a plan includes, in catalog order. */
export function planLines(resources: PlanResource[], limits: Record<string, number>) {
  return resources.flatMap((r) => {
    const v = limits[r.key] ?? NOT_INCLUDED
    return v === NOT_INCLUDED ? [] : [lineOf(r, v)]
  })
}

function same(a: string, b: string) {
  return a.trim().toLowerCase() === b.trim().toLowerCase()
}

/** Staff's own lines, minus any that only say again what the limits already say. */
export function extrasOf(features: string[], lines: string[]) {
  return features.map((f) => f.trim()).filter((f) => f !== '' && !lines.some((l) => same(f, l)))
}

/** Whether `value` is more than `before` of a resource: a higher cap, unlimited, or newly included. */
function improves(r: PlanResource, value: number, before: number) {
  if (value === NOT_INCLUDED || value === before) return false
  if (r.kind === 'access' || before === NOT_INCLUDED) return true
  return value === UNLIMITED || (before !== UNLIMITED && value > before)
}

function summaryOf(
  plan: PlanInput,
  previous: PlanInput | undefined,
  resources: PlanResource[],
): PlanSummary {
  const lines = planLines(resources, plan.limits)
  const extras = extrasOf(plan.features, lines)
  if (!previous) return { lines, highlights: lines, extras }
  const highlights = resources.flatMap((r) => {
    const v = capOf(plan, r)
    return improves(r, v, capOf(previous, r)) ? [lineOf(r, v)] : []
  })
  const heading = `Everything in ${previous.name}${highlights.length + extras.length > 0 ? ', plus:' : ''}`
  return { lines, heading, highlights, extras }
}

function cellOf(r: PlanResource, value: number): MatrixCell {
  if (value === NOT_INCLUDED) return { kind: 'not_included' }
  if (r.kind === 'access') return { kind: 'included' }
  if (value === UNLIMITED) return { kind: 'text', text: 'Unlimited' }
  const n = value.toLocaleString()
  return { kind: 'text', text: r.kind === 'monthly' ? `${n} / month` : n }
}

function matrixOf(plans: PlanInput[], resources: PlanResource[]): MatrixGroup[] {
  const groups: MatrixGroup[] = [
    { title: 'Limits', rows: [] },
    { title: 'Features', rows: [] },
  ]
  for (const r of resources) {
    if (plans.every((p) => capOf(p, r) === NOT_INCLUDED)) continue
    groups[r.kind === 'access' ? 1 : 0].rows.push({
      key: r.key,
      label: r.label,
      cells: plans.map((p) => cellOf(r, capOf(p, r))),
    })
  }
  return groups.filter((g) => g.rows.length > 0)
}

/**
 * What each paid plan shows on its card and in the comparison table, from its limits and the
 * resource catalog alone. `plans` are in display order, cheapest first: each plan is compared with
 * the one before it, and a resource that got worse is never listed.
 */
export function planModel(plans: PlanInput[], resources: PlanResource[]): PlanModel {
  return {
    plans: plans.map((plan, i) => summaryOf(plan, plans[i - 1], resources)),
    matrix: matrixOf(plans, resources),
  }
}
