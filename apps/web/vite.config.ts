import tailwindcss from '@tailwindcss/vite'
import { devtools } from '@tanstack/devtools-vite'
import { tanstackStart } from '@tanstack/react-start/plugin/vite'
import viteReact from '@vitejs/plugin-react'
import { nitro } from 'nitro/vite'
import { defineConfig } from 'vite'

export default defineConfig({
  resolve: { tsconfigPaths: true },
  // strictPort: fail instead of silently moving to another port, which would break CORS and cookies.
  server: { port: Number(process.env.WEB_PORT ?? 3000), strictPort: true },
  plugins: [
    devtools(),
    nitro({
      routeRules: {
        // No framing (clickjacking on the login page), no MIME sniffing, and no
        // Referer: emailed links put one-time tokens in page URLs.
        '/**': {
          headers: {
            'X-Frame-Options': 'DENY',
            'Content-Security-Policy': "frame-ancestors 'none'",
            'X-Content-Type-Options': 'nosniff',
            'Referrer-Policy': 'no-referrer',
          },
        },
      },
    }),
    tailwindcss(),
    tanstackStart(),
    viteReact(),
  ],
})
