import type {
  CreatePlanInputBody,
  CreatePriceInputBody,
  CreateProviderPriceInputBody,
} from '@relay/api-client'

export type Cadence = 'month' | 'year' | 'lifetime'

export type PlanPublishApi = {
  createPlan: (body: CreatePlanInputBody) => Promise<string>
  createPrice: (body: CreatePriceInputBody) => Promise<string>
  /** Resolves with an ID too when the provider already sells the price (a retry after a lost reply). */
  createProviderPrice: (body: CreateProviderPriceInputBody) => Promise<string>
}

/** One API call of a plan going on sale. Later steps need the IDs earlier ones created. */
export type PublishStep =
  | { id: 'plan'; kind: 'plan'; body: CreatePlanInputBody }
  | {
      id: `price:${Cadence}`
      kind: 'price'
      cadence: Cadence
      body: Omit<CreatePriceInputBody, 'plan_id'>
    }
  | {
      id: `sale:${Cadence}:${string}`
      kind: 'sale'
      cadence: Cadence
      provider: CreateProviderPriceInputBody['provider']
    }

export type StepStatus = 'pending' | 'running' | 'done' | 'failed'

export type StepState = { status: StepStatus; error?: unknown }

/** What each finished step created, by step ID. A retry skips every step listed here. */
export type CreatedIds = Record<string, string>

export type PublishResult =
  | { ok: true; ids: CreatedIds }
  | { ok: false; ids: CreatedIds; failed: PublishStep['id']; error: unknown }

/**
 * Runs the steps in order and stops at the first failure. Steps whose ID is already in `done` are
 * skipped, so calling it again with the returned `ids` finishes the rest without creating anything twice.
 */
export async function runPublish(
  steps: PublishStep[],
  done: CreatedIds,
  api: PlanPublishApi,
  onState: (id: PublishStep['id'], state: StepState) => void,
): Promise<PublishResult> {
  const ids = { ...done }
  for (const step of steps) {
    if (ids[step.id]) {
      onState(step.id, { status: 'done' })
      continue
    }
    onState(step.id, { status: 'running' })
    try {
      ids[step.id] = await create(step, ids, api)
      onState(step.id, { status: 'done' })
    } catch (error) {
      onState(step.id, { status: 'failed', error })
      return { ok: false, ids, failed: step.id, error }
    }
  }
  return { ok: true, ids }
}

function create(step: PublishStep, ids: CreatedIds, api: PlanPublishApi) {
  switch (step.kind) {
    case 'plan':
      return api.createPlan(step.body)
    case 'price':
      return api.createPrice({ ...step.body, plan_id: ids.plan })
    case 'sale':
      return api.createProviderPrice({
        plan_price_id: ids[`price:${step.cadence}`],
        provider: step.provider,
      })
  }
}
