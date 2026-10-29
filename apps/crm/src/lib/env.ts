import { z } from 'zod'

const schema = z.object({
  VITE_API_URL: z.url().default('http://localhost:8080'),
})

export const env = schema.parse(import.meta.env)
