import { z } from 'zod'
import type { ContactSource, ImportField, LastMessage, OptIn } from '../types'

// ---------- list filters (URL search params, AD-10) ----------

export const OPT_INS = ['in', 'out', 'pending'] as const satisfies readonly OptIn[]
export const LAST_MESSAGES = [
  '24h',
  '7d',
  '30d',
  'over_90d',
] as const satisfies readonly LastMessage[]

/** Bad values in a shared link fall back to "no filter" instead of an error page. */
export const contactsSearchSchema = z.object({
  q: z.string().trim().max(200).optional().catch(undefined),
  opt_in: z.enum(OPT_INS).optional().catch(undefined),
  tag_id: z.uuid().optional().catch(undefined),
  last_message: z.enum(LAST_MESSAGES).optional().catch(undefined),
  cursor: z.string().max(500).optional().catch(undefined),
})

export type ContactsSearch = z.infer<typeof contactsSearchSchema>

export const OPT_IN_FILTERS: { value: OptIn | undefined; label: string }[] = [
  { value: undefined, label: 'All' },
  { value: 'in', label: 'Opted in' },
  { value: 'out', label: 'Opted out' },
  { value: 'pending', label: 'Pending' },
]

export const OPT_IN_LABEL: Record<OptIn, string> = {
  in: 'Opted in',
  out: 'Opted out',
  pending: 'Pending',
}

export const LAST_MESSAGE_LABEL: Record<LastMessage, string> = {
  '24h': 'Last 24 hours',
  '7d': 'Last 7 days',
  '30d': 'Last 30 days',
  over_90d: 'Over 90 days',
}

export const SOURCE_LABEL: Record<ContactSource, string> = {
  manual: 'Manual',
  import: 'CSV import',
  inbox: 'Inbox',
  api: 'API',
}

// ---------- add / edit contact ----------

export const LANGUAGES = [
  'English',
  'Spanish',
  'Portuguese',
  'French',
  'German',
  'Italian',
  'Arabic',
  'Hindi',
  'Bengali',
  'Japanese',
] as const

export const contactFormSchema = z.object({
  name: z.string().trim().min(1, 'Enter a name').max(200, 'Use at most 200 characters'),
  // The server parses and stores E.164 (AD-2); here we only check the country code is there.
  phone: z
    .string()
    .trim()
    .min(1, 'Enter a phone number')
    .regex(/^\+[\d\s().-]{6,}$/, 'Start with + and the country code, for example +44 7700 900412'),
  email: z.union([z.literal(''), z.email('Enter an email like name@example.com')]),
  opt_in: z.enum(OPT_INS),
  language: z.string(),
  tag_ids: z.array(z.string()),
})

export type ContactFormValues = z.infer<typeof contactFormSchema>

// ---------- import (AD-5) ----------

export const IMPORT_MAX_BYTES = 10 * 1024 * 1024
export const IMPORT_EXTENSIONS = ['.csv', '.xlsx'] as const

/** Client-side check before uploading; the server checks again. */
export function importFileError(file: File): string | undefined {
  const name = file.name.toLowerCase()
  if (name.endsWith('.xls')) return 'Save the file as .xlsx or .csv, then upload it again.'
  if (!IMPORT_EXTENSIONS.some((ext) => name.endsWith(ext))) return 'Upload a .csv or .xlsx file.'
  if (file.size > IMPORT_MAX_BYTES)
    return 'The file is larger than 10 MB. Split it and import each part.'
  if (file.size === 0) return 'The file is empty.'
  return undefined
}

export const IMPORT_FIELDS: { value: ImportField; label: string }[] = [
  { value: 'name', label: 'Name' },
  { value: 'phone', label: 'Phone' },
  { value: 'email', label: 'Email' },
  { value: 'tag', label: 'Tag' },
  { value: 'attribute', label: 'Custom field' },
  { value: 'skip', label: 'Don’t import' },
]

export const mappingSchema = z
  .object({
    columns: z.array(
      z.object({
        name: z.string(),
        field: z.enum(['name', 'phone', 'email', 'tag', 'attribute', 'skip']),
        attribute_key: z.string().trim().max(64, 'Use at most 64 characters').optional(),
      }),
    ),
    tag_name: z.string().trim().max(50, 'Use at most 50 characters'),
    opt_in_confirmed: z.literal(true, {
      error: 'Confirm that these contacts agreed to get WhatsApp messages from you',
    }),
  })
  .superRefine((v, ctx) => {
    const count = (f: ImportField) => v.columns.filter((c) => c.field === f).length
    if (count('phone') !== 1) {
      ctx.addIssue({
        code: 'custom',
        path: ['columns'],
        message: 'Choose exactly one column as Phone',
      })
    } else if (count('name') > 1 || count('email') > 1) {
      ctx.addIssue({
        code: 'custom',
        path: ['columns'],
        message: 'Choose at most one Name column and one Email column',
      })
    }
  })

/** The step's form state; the schema then insists the opt-in box is ticked. */
export type MappingValues = Omit<z.input<typeof mappingSchema>, 'opt_in_confirmed'> & {
  opt_in_confirmed: boolean
}

export const tagNameSchema = z
  .string()
  .trim()
  .min(1, 'Enter a tag name')
  .max(50, 'Use at most 50 characters')
