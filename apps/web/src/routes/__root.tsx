import { TanStackDevtools } from '@tanstack/react-devtools'
import type { QueryClient } from '@tanstack/react-query'
import { ReactQueryDevtoolsPanel } from '@tanstack/react-query-devtools'
import { createRootRouteWithContext, HeadContent, Scripts } from '@tanstack/react-router'
import { TanStackRouterDevtoolsPanel } from '@tanstack/react-router-devtools'
import type { ReactNode } from 'react'
import appCss from '../styles/app.css?url'

interface RouterContext {
  queryClient: QueryClient
}

export const Route = createRootRouteWithContext<RouterContext>()({
  head: () => ({
    meta: [
      { charSet: 'utf-8' },
      { name: 'viewport', content: 'width=device-width, initial-scale=1' },
      { title: 'Relay' },
    ],
    links: [{ rel: 'stylesheet', href: appCss }],
  }),
  shellComponent: RootDocument,
})

function RootDocument({ children }: { children: ReactNode }) {
  return (
    <html lang="en">
      <head>
        <HeadContent />
      </head>
      <body>
        {children}
        {import.meta.env.DEV && (
          <TanStackDevtools
            config={{ position: 'bottom-right' }}
            plugins={[
              { name: 'TanStack Router', render: <TanStackRouterDevtoolsPanel /> },
              { name: 'TanStack Query', render: <ReactQueryDevtoolsPanel /> },
            ]}
          />
        )}
        <Scripts />
      </body>
    </html>
  )
}
