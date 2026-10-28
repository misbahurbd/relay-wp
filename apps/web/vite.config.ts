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
  plugins: [devtools(), nitro(), tailwindcss(), tanstackStart(), viteReact()],
})
