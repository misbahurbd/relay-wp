import { z } from 'zod'
import type { SegmentCondition, SegmentField, SegmentOp, SegmentRules, Tag } from '../types'

/** ?new opens the page on a blank segment (the broadcast wizard's "New segment"). */
export const segmentsSearchSchema = z.object({
  new: z.boolean().optional().catch(undefined),
})

type ValueKind = 'tag' | 'opt_in' | 'days' | 'source' | 'text' | 'code' | 'attribute'

/** What a segment can look at (ADR 0014), with each field's conditions. */
export const SEGMENT_FIELDS: {
  value: SegmentField
  label: string
  kind: ValueKind
  ops: { value: SegmentOp; label: string }[]
}[] = [
  {
    value: 'tag',
    label: 'Tag',
    kind: 'tag',
    ops: [
      { value: 'is', label: 'is' },
      { value: 'is_not', label: 'is not' },
    ],
  },
  {
    value: 'opt_in',
    label: 'Opt-in status',
    kind: 'opt_in',
    ops: [
      { value: 'is', label: 'is' },
      { value: 'is_not', label: 'is not' },
    ],
  },
  {
    value: 'last_message',
    label: 'Last message',
    kind: 'days',
    ops: [
      { value: 'within', label: 'within last' },
      { value: 'over', label: 'more than' },
      { value: 'never', label: 'never' },
    ],
  },
  {
    value: 'added',
    label: 'Added to contacts',
    kind: 'days',
    ops: [
      { value: 'within', label: 'within last' },
      { value: 'over', label: 'more than' },
    ],
  },
  {
    value: 'source',
    label: 'Source',
    kind: 'source',
    ops: [
      { value: 'is', label: 'is' },
      { value: 'is_not', label: 'is not' },
    ],
  },
  {
    value: 'language',
    label: 'Language',
    kind: 'text',
    ops: [
      { value: 'is', label: 'is' },
      { value: 'is_not', label: 'is not' },
    ],
  },
  {
    value: 'country_code',
    label: 'Phone country code',
    kind: 'code',
    ops: [{ value: 'starts_with', label: 'is' }],
  },
  {
    value: 'attribute',
    label: 'Custom field',
    kind: 'attribute',
    ops: [
      { value: 'is', label: 'is' },
      { value: 'contains', label: 'contains' },
    ],
  },
]

export const OPT_IN_VALUES = [
  { value: 'in', label: 'Opted in' },
  { value: 'pending', label: 'Not opted in yet' },
  { value: 'out', label: 'Opted out' },
]

export const SOURCE_VALUES = [
  { value: 'manual', label: 'Added by hand' },
  { value: 'import', label: 'Imported' },
  { value: 'inbox', label: 'Messaged us' },
  { value: 'api', label: 'API' },
]

export const fieldOf = (f: SegmentField) =>
  SEGMENT_FIELDS.find((x) => x.value === f) ?? SEGMENT_FIELDS[0]

/** A fresh condition on a field, with a sensible first value. */
export function blankCondition(field: SegmentField, tags: Tag[] = []): SegmentCondition {
  const f = fieldOf(field)
  const value =
    f.kind === 'tag'
      ? (tags[0]?.id ?? '')
      : f.kind === 'opt_in'
        ? 'in'
        : f.kind === 'days'
          ? '30'
          : f.kind === 'source'
            ? 'manual'
            : f.kind === 'code'
              ? '+'
              : ''
  return { field, op: f.ops[0].value, value, key: '' }
}

/** Starting points the builder offers, built only from fields Relay has. */
export function presets(tags: Tag[]): { name: string; description: string; rules: SegmentRules }[] {
  const out: { name: string; description: string; rules: SegmentRules }[] = [
    {
      name: 'Active this month',
      description: 'Opted in and wrote to us in the last 30 days',
      rules: {
        groups: [
          {
            match: 'all',
            conditions: [
              { field: 'opt_in', op: 'is', value: 'in' },
              { field: 'last_message', op: 'within', value: '30' },
            ],
          },
        ],
      },
    },
    {
      name: 'Gone quiet',
      description: 'No message for more than 60 days',
      rules: {
        groups: [
          { match: 'all', conditions: [{ field: 'last_message', op: 'over', value: '60' }] },
        ],
      },
    },
    {
      name: 'New this week',
      description: 'Added in the last 7 days',
      rules: {
        groups: [{ match: 'all', conditions: [{ field: 'added', op: 'within', value: '7' }] }],
      },
    },
    {
      name: 'Never messaged',
      description: 'Opted in but never wrote to us',
      rules: {
        groups: [
          {
            match: 'all',
            conditions: [
              { field: 'opt_in', op: 'is', value: 'in' },
              { field: 'last_message', op: 'never', value: '' },
            ],
          },
        ],
      },
    },
  ]
  if (tags[0]) {
    out.unshift({
      name: `${tags[0].name} customers`,
      description: `Tagged ${tags[0].name} and opted in`,
      rules: {
        groups: [
          {
            match: 'all',
            conditions: [
              { field: 'tag', op: 'is', value: tags[0].id },
              { field: 'opt_in', op: 'is', value: 'in' },
            ],
          },
        ],
      },
    })
  }
  return out
}

/** Whether a condition has what it needs; the server checks the same. */
export function conditionComplete(c: SegmentCondition) {
  const kind = fieldOf(c.field).kind
  if (c.op === 'never') return true
  if (kind === 'days') return /^\d+$/.test(c.value) && Number(c.value) >= 1
  if (kind === 'code') return /^\+\d{1,4}$/.test(c.value)
  if (kind === 'attribute') return !!c.key?.trim() && !!c.value.trim()
  return !!c.value.trim()
}

export const rulesComplete = (r: SegmentRules) =>
  r.groups.length > 0 &&
  r.groups.every((g) => g.conditions.length > 0 && g.conditions.every(conditionComplete))

function describe(c: SegmentCondition, tags: Tag[]) {
  const f = fieldOf(c.field)
  const op = f.ops.find((o) => o.value === c.op)?.label ?? c.op
  switch (f.kind) {
    case 'tag':
      return `${c.op === 'is' ? 'are' : 'aren’t'} tagged ${tags.find((t) => t.id === c.value)?.name ?? 'a deleted tag'}`
    case 'opt_in':
      return `${c.op === 'is' ? 'are' : 'aren’t'} ${OPT_IN_VALUES.find((o) => o.value === c.value)?.label.toLowerCase() ?? c.value}`
    case 'days':
      if (c.op === 'never') return 'have never messaged us'
      return c.field === 'added'
        ? `were added ${op} ${c.value} days${c.op === 'over' ? ' ago' : ''}`
        : `last messaged ${op} ${c.value} days${c.op === 'over' ? ' ago' : ''}`
    case 'source':
      return `${c.op === 'is' ? 'came from' : 'didn’t come from'} ${SOURCE_VALUES.find((s) => s.value === c.value)?.label.toLowerCase() ?? c.value}`
    case 'code':
      return `have a ${c.value} number`
    case 'attribute':
      return `have ${c.key || 'a field'} ${op} “${c.value}”`
    default:
      return `speak ${c.op === 'is' ? '' : 'something other than '}${c.value}`
  }
}

/** "contacts who are tagged VIP and are opted in, or have a +1 number" */
export function plainWords(r: SegmentRules, tags: Tag[]) {
  return r.groups
    .map((g) =>
      g.conditions.map((c) => describe(c, tags)).join(g.match === 'all' ? ' and ' : ' or '),
    )
    .filter(Boolean)
    .join(', or ')
}
