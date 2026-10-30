import { z } from 'zod'

const schema = z.object({
  VITE_API_URL: z.url().default('http://localhost:8080'),
  /** Show billing sections that run on sample data, with a Preview badge (docs/PHASE-7-BILLING.md). */
  VITE_BILLING_PREVIEW: z.stringbool().default(true),
})

export const env = schema.parse(import.meta.env)
