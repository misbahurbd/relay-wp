import { client } from '@relay/api-client'
import { env } from './env'

// Cookies carry the session for the web app; the API also accepts Bearer tokens for other clients.
client.setConfig({
  baseUrl: env.VITE_API_URL,
  credentials: 'include',
})

export { client }
