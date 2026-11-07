import type { Plan, Resource } from '../../types'
import { FREE_PLAN } from '../kit/limits'
import { type PlanModel, planModel } from '../kit/plan-highlights'
import {
  draftLimits,
  featureLines,
  type PlanDraft,
  type PlanEditorMode,
  sourceOf,
} from './plan-draft'

type Listed = {
  code: string
  sortOrder: number
  name: string
  limits: Plan['limits']
  features: string[]
}

/** Whether customers can buy it today: an active price linked to an active provider. */
function onSale(plan: Plan) {
  return (
    plan.is_active &&
    plan.prices.some((p) => p.is_active && p.provider_prices.some((pp) => pp.is_active))
  )
}

function byDisplayOrder(a: Listed, b: Listed) {
  return a.sortOrder - b.sortOrder || a.code.localeCompare(b.code)
}

/**
 * The plans customers compare as they stand in the catalog now, with this draft in its place.
 * The plan being edited, or the one getting a new version, is swapped for the draft; the free plan
 * and anything not on sale never reach the pricing page.
 */
export function previewCatalog(
  draft: PlanDraft,
  mode: PlanEditorMode,
  plans: Plan[],
  resources: Resource[],
): { model: PlanModel; index: number } {
  const replaced = sourceOf(mode)?.code ?? draft.code.trim()
  const seen = new Set<string>()
  const others = plans.flatMap((p): Listed[] => {
    if (p.code === FREE_PLAN || p.code === replaced || !onSale(p) || seen.has(p.code)) return []
    seen.add(p.code)
    return [
      {
        code: p.code,
        sortOrder: p.sort_order,
        name: p.name,
        limits: p.limits,
        features: p.features ?? [],
      },
    ]
  })
  const mine: Listed = {
    code: draft.code.trim() || replaced,
    sortOrder: Number(draft.sortOrder) || 0,
    name: draft.name.trim() || 'This plan',
    limits: draftLimits(draft),
    features: featureLines(draft),
  }
  const ordered = [...others, mine].sort(byDisplayOrder)
  return { model: planModel(ordered, resources), index: ordered.indexOf(mine) }
}
