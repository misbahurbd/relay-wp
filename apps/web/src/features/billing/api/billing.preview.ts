import { queryOptions } from '@tanstack/react-query'
import type {
  CampaignSpendList,
  MetaCostSummary,
  Spending,
  SpendingBudget,
  SpendingPeriod,
  SpendPoint,
  UsageRow,
  Wallet,
  WalletEntry,
  WalletOutlook,
} from '../types'
import { billingKeys } from './billing.keys'

// Sample data for the billing sections whose backend doesn't exist yet (docs/PHASE-7-BILLING.md).
// Every preview reads through query options shaped like the real endpoint will be, so going live
// means swapping a queryFn for an @relay/api-client call. Nothing here calls the API. Sample
// values live in this file only, never in components.

/** Resolves on the next tick, so previews go through the same loading states as real data. */
const sample =
  <T>(value: T) =>
  () =>
    Promise.resolve(value)

/** Usage meters for limits Relay doesn't count yet. */
export const usagePreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'usage'),
    queryFn: sample<UsageRow[]>([
      { key: 'contacts', label: 'Contacts', used: 8640, limit: 10000, preview: true },
      { key: 'chatbot_flows', label: 'Chatbot flows', used: 4, limit: 10, preview: true },
      {
        key: 'api_calls_per_month',
        label: 'API calls this month',
        used: 41200,
        limit: 50000,
        preview: true,
      },
    ]),
    staleTime: Infinity,
  })

/** This month's Meta charges by category, as Meta's pricing analytics will report them. */
export const metaCostsPreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'meta-costs'),
    queryFn: sample<MetaCostSummary>({
      currency: 'USD',
      rows: [
        { category: 'marketing', messages: 18240, avg_micros: 22600, cost_minor: 41280 },
        { category: 'utility', messages: 9860, avg_micros: 3900, cost_minor: 3846 },
        { category: 'authentication', messages: 2310, avg_micros: 3900, cost_minor: 902 },
        { category: 'service', messages: 6914, avg_micros: 0, cost_minor: 0 },
      ],
      messages: 37324,
      total_minor: 46028,
    }),
    staleTime: Infinity,
  })

/** The message wallet. */
export const walletPreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'wallet'),
    queryFn: sample<Wallet>({
      currency: 'USD',
      balance_minor: 13972,
      auto_recharge: { enabled: true, below_minor: 5000, add_minor: 20000 },
    }),
    staleTime: Infinity,
  })

// ---------- Spending page ----------

/** Splits total into integer parts in proportion to weights (largest remainder), so they add up exactly. */
function spread(weights: number[], total: number) {
  const sum = weights.reduce((a, b) => a + b, 0)
  const exact = weights.map((w) => (w / sum) * total)
  const parts = exact.map(Math.floor)
  let left = total - parts.reduce((a, b) => a + b, 0)
  const order = exact.map((v, i) => [v - Math.floor(v), i] as const).sort((a, b) => b[0] - a[0])
  for (const [, i] of order) {
    if (left <= 0) break
    parts[i] += 1
    left -= 1
  }
  return parts
}

function addDays(iso: string, days: number) {
  const d = new Date(`${iso}T00:00:00Z`)
  d.setUTCDate(d.getUTCDate() + days)
  return d.toISOString().slice(0, 10)
}

/** A believable daily (or weekly) series: steady utility and login codes, marketing peaks on broadcast days. */
function series(
  from: string,
  unit: 'day' | 'week',
  count: number,
  spikes: number[],
  totals: { marketing: number; utility: number; authentication: number },
): SpendPoint[] {
  const weekly = unit === 'week'
  const idx = Array.from({ length: count }, (_, i) => i)
  const marketing = spread(
    idx.map(
      (i) =>
        0.45 +
        0.2 * Math.sin(i * 1.1) +
        (spikes.includes(i) ? (weekly ? 0.8 : 4.5) : 0) +
        (weekly ? 0.6 : 0),
    ),
    totals.marketing,
  )
  const utility = spread(
    idx.map((i) => 1 + 0.25 * Math.sin(i * 0.7 + 1)),
    totals.utility,
  )
  const authentication = spread(
    idx.map((i) => 1 + 0.3 * Math.cos(i * 0.9)),
    totals.authentication,
  )
  return idx.map((i) => ({
    date: addDays(from, weekly ? i * 7 : i),
    marketing_minor: marketing[i],
    utility_minor: utility[i],
    authentication_minor: authentication[i],
  }))
}

/** October's Meta total; the number and country splits below are its shares, scaled to each period. */
const OCTOBER_META_MINOR = 46028

function spendingSample(
  period: SpendingPeriod,
  p: {
    from: string
    to: string
    plan: number
    marketing: [number, number]
    utility: [number, number]
    authentication: [number, number]
    service: number
    outlook: Spending['outlook']
    change: Spending['change']
    unit: 'day' | 'week'
    points: number
    spikes: number[]
  },
): Spending {
  const meta = p.marketing[0] + p.utility[0] + p.authentication[0]
  const paid = p.marketing[1] + p.utility[1] + p.authentication[1]
  const scale = (minor: number) => Math.round((minor * meta) / OCTOBER_META_MINOR)
  // cost in minor units to micros per message (USD has two minor digits).
  const avg = ([cost, count]: [number, number]) => Math.round((cost * 10_000) / count)
  return {
    period,
    currency: 'USD',
    from: p.from,
    to: p.to,
    plan_minor: p.plan,
    categories: [
      {
        category: 'marketing',
        messages: p.marketing[1],
        avg_micros: avg(p.marketing),
        cost_minor: p.marketing[0],
      },
      {
        category: 'utility',
        messages: p.utility[1],
        avg_micros: avg(p.utility),
        cost_minor: p.utility[0],
      },
      {
        category: 'authentication',
        messages: p.authentication[1],
        avg_micros: avg(p.authentication),
        cost_minor: p.authentication[0],
      },
      { category: 'service', messages: p.service, avg_micros: 0, cost_minor: 0 },
    ],
    meta_minor: meta,
    paid_messages: paid,
    free_messages: p.service,
    avg_micros: avg([meta, paid]),
    outlook: p.outlook,
    change: p.change,
    series: {
      unit: p.unit,
      points: series(p.from, p.unit, p.points, p.spikes, {
        marketing: p.marketing[0],
        utility: p.utility[0],
        authentication: p.authentication[0],
      }),
    },
    numbers: [
      { name: 'Main line', phone: '+1 415 555 0142', cost_minor: scale(43106) },
      { name: 'Returns line', phone: '+1 415 555 0178', cost_minor: scale(2922) },
    ],
    countries: [
      { code: 'US', name: 'United States', cost_minor: scale(19840) },
      { code: 'GB', name: 'United Kingdom', cost_minor: scale(9610) },
      { code: 'AE', name: 'United Arab Emirates', cost_minor: scale(6390) },
      { code: 'IN', name: 'India', cost_minor: scale(4120) },
      { code: 'PT', name: 'Portugal', cost_minor: scale(2240) },
      { code: null, name: 'Other (14 countries)', cost_minor: scale(3828) },
    ],
  }
}

const SPENDING: Record<SpendingPeriod, Spending> = {
  this_month: spendingSample('this_month', {
    from: '2026-10-01',
    to: '2026-10-28',
    plan: 7900,
    marketing: [41280, 18240],
    utility: [3846, 9860],
    authentication: [902, 2310],
    service: 6914,
    outlook: { kind: 'projected', amount_minor: 58900, note: 'Meta ~$510 + plan $79' },
    change: { against: 'September (same days)', permille: 92, note: 'More marketing broadcasts' },
    unit: 'day',
    points: 28,
    spikes: [6, 13, 21, 24, 27],
  }),
  last_month: spendingSample('last_month', {
    from: '2026-09-01',
    to: '2026-09-30',
    plan: 7900,
    marketing: [37120, 16400],
    utility: [3510, 9000],
    authentication: [840, 2150],
    service: 6320,
    outlook: {
      kind: 'final',
      amount_minor: 49370,
      note: 'Month closed · invoice INV-2026-0914',
    },
    change: { against: 'August', permille: 184, note: 'Back-to-school campaign' },
    unit: 'day',
    points: 30,
    spikes: [4, 11, 18, 26],
  }),
  last_3_months: spendingSample('last_3_months', {
    from: '2026-08-04',
    to: '2026-10-28',
    plan: 23700,
    marketing: [108250, 47830],
    utility: [10536, 27010],
    authentication: [2502, 6400],
    service: 19114,
    outlook: {
      kind: 'monthly_average',
      amount_minor: 48329,
      note: 'Plan $79 + Meta ~$404 a month',
    },
    change: { against: 'previous 3 months', permille: 226, note: 'Contacts grew 31%' },
    unit: 'week',
    points: 13,
    spikes: [3, 6, 9, 12],
  }),
}

/** Plan and Meta charges for one period, with the breakdowns the Spending page charts. */
export const spendingPreviewQueryOptions = (workspaceId: string, period: SpendingPeriod) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, `spending-${period}`),
    queryFn: sample<Spending>(SPENDING[period]),
    staleTime: Infinity,
  })

/** This month's budget and its alerts. */
export const spendingBudgetPreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'spending-budget'),
    queryFn: sample<SpendingBudget>({
      currency: 'USD',
      month: '2026-10-01',
      budget_minor: 60000,
      spent_minor: 53928,
      projected_minor: 58900,
      projected_to: '2026-10-31',
      alert_at_80: true,
      pause_marketing_at_100: false,
      alert_email: 'billing@example.com',
    }),
    staleTime: Infinity,
  })

/** What each broadcast, automation and API flow cost this month. */
export const campaignSpendPreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'spending-campaigns'),
    queryFn: sample<CampaignSpendList>({
      currency: 'USD',
      month: '2026-10-01',
      rows: [
        {
          id: 'winter',
          name: 'Winter collection is here',
          source: 'broadcast',
          detail: 'winter_arrivals_v2',
          date: '2026-10-25',
          recipients: 12105,
          category: 'marketing',
          cost_minor: 26810,
          in_progress: false,
          cost_per_reply_minor: 24,
        },
        {
          id: 'halloween',
          name: 'Halloween Flash Sale – 30% off',
          source: 'broadcast',
          detail: 'sending 64%',
          date: '2026-10-28',
          recipients: 8420,
          category: 'marketing',
          cost_minor: 12140,
          in_progress: true,
          cost_per_reply_minor: 29,
        },
        {
          id: 'order-confirmations',
          name: 'Order confirmations',
          source: 'automation',
          detail: 'Shopify',
          date: null,
          recipients: 7110,
          category: 'utility',
          cost_minor: 2772,
          in_progress: false,
          cost_per_reply_minor: null,
        },
        {
          id: 'abandoned-cart',
          name: 'Abandoned cart reminders',
          source: 'automation',
          detail: null,
          date: null,
          recipients: 1030,
          category: 'marketing',
          cost_minor: 2330,
          in_progress: false,
          cost_per_reply_minor: 24,
        },
        {
          id: 'csat',
          name: 'How did we do? October orders',
          source: 'broadcast',
          detail: 'csat_survey_oct',
          date: '2026-10-22',
          recipients: 2436,
          category: 'utility',
          cost_minor: 950,
          in_progress: false,
          cost_per_reply_minor: 2,
        },
        {
          id: 'login-codes',
          name: 'Login codes',
          source: 'api',
          detail: 'one-time passwords',
          date: null,
          recipients: 2310,
          category: 'authentication',
          cost_minor: 902,
          in_progress: false,
          cost_per_reply_minor: null,
        },
        {
          id: 'payment-due',
          name: 'Payment due reminder',
          source: 'broadcast',
          detail: '12 failed, refunded',
          date: '2026-10-21',
          recipients: 318,
          category: 'utility',
          cost_minor: 124,
          in_progress: false,
          cost_per_reply_minor: 3,
        },
      ],
    }),
    staleTime: Infinity,
  })

/** Spend rate, card and upcoming sends around the wallet balance (walletPreviewQueryOptions). */
export const walletOutlookPreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'wallet-outlook'),
    queryFn: sample<WalletOutlook>({
      currency: 'USD',
      avg_daily_spend_minor: 1640,
      card: { brand: 'Visa', last4: '4242', exp_month: 8, exp_year: 2028 },
      shortfall: { broadcast: 'Weekend offer', needed_minor: 15320 },
      top_up_options_minor: [5000, 10000, 25000],
      min_top_up_minor: 1000,
    }),
    staleTime: Infinity,
  })

/** Recent wallet movements, newest first. */
export const walletLedgerPreviewQueryOptions = (workspaceId: string) =>
  queryOptions({
    queryKey: billingKeys.preview(workspaceId, 'wallet-ledger'),
    queryFn: sample<WalletEntry[]>([
      {
        id: 'w7',
        date: '2026-10-28',
        description: 'Broadcast “Halloween Flash Sale”',
        detail: '5,372 delivered so far · Marketing',
        amount_minor: -12140,
        balance_after_minor: 13972,
      },
      {
        id: 'w6',
        date: '2026-10-27',
        description: 'Daily message charges',
        detail: 'Automations, API and order updates',
        amount_minor: -2369,
        balance_after_minor: 26112,
      },
      {
        id: 'w5',
        date: '2026-10-26',
        description: 'Top up',
        detail: 'Card ending 4242 · Shakil',
        amount_minor: 15000,
        balance_after_minor: 28481,
      },
      {
        id: 'w4',
        date: '2026-10-25',
        description: 'Broadcast “Winter collection is here”',
        detail: '11,862 delivered · Marketing',
        amount_minor: -26810,
        balance_after_minor: 13481,
      },
      {
        id: 'w3',
        date: '2026-10-23',
        description: 'Refund for failed messages',
        detail: '12 failed · “Payment due reminder”',
        amount_minor: 5,
        balance_after_minor: 40291,
      },
      {
        id: 'w2',
        date: '2026-10-22',
        description: 'Broadcast “How did we do? October orders”',
        detail: '2,402 delivered · Utility',
        amount_minor: -950,
        balance_after_minor: 40286,
      },
      {
        id: 'w1',
        date: '2026-10-20',
        description: 'Top up',
        detail: 'Card ending 4242 · Nadia Akter',
        amount_minor: 30000,
        balance_after_minor: 41236,
      },
    ]),
    staleTime: Infinity,
  })
