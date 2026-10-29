import tailwindcss from '@tailwindcss/vite'
import viteReact from '@vitejs/plugin-react'
import { defineConfig } from 'vite'

// The staff CRM: a static single-page app on its own origin. The API admits it through
// CORS_ALLOWED_ORIGINS and lets only ADMIN_EMAILS use /v1/admin.
export default defineConfig({
  resolve: { tsconfigPaths: true },
  // strictPort: a silently moved port would break CORS and cookies.
  server: { port: Number(process.env.CRM_PORT ?? 3020), strictPort: true },
  preview: {
    headers: {
      'X-Frame-Options': 'DENY',
      'Content-Security-Policy': "frame-ancestors 'none'",
      'X-Content-Type-Options': 'nosniff',
      'Referrer-Policy': 'no-referrer',
    },
  },
  plugins: [tailwindcss(), viteReact()],
})
