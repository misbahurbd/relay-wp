import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Toaster } from 'sonner'
import { Button } from '#/components/ui/button'
import { Skeleton } from '#/components/ui/skeleton'
import { AdminPage, type AdminTab } from '#/features/admin'
import { meQueryOptions, SignIn, useSignOut } from '#/features/session'

const TABS: AdminTab[] = ['plans', 'discounts', 'workspaces']

function tabFromURL(): AdminTab {
  const t = new URLSearchParams(window.location.search).get('tab')
  return TABS.find((x) => x === t) ?? 'plans'
}

/** The open tab lives in ?tab=, so a reload or a shared link opens the same one. */
function useTab() {
  const [tab, setTab] = useState(tabFromURL)
  useEffect(() => {
    const onPop = () => setTab(tabFromURL())
    window.addEventListener('popstate', onPop)
    return () => window.removeEventListener('popstate', onPop)
  }, [])
  const select = (t: AdminTab) => {
    const url = new URL(window.location.href)
    if (t === 'plans') url.searchParams.delete('tab')
    else url.searchParams.set('tab', t)
    window.history.pushState(null, '', url)
    setTab(t)
  }
  return [tab, select] as const
}

export function App() {
  const me = useQuery(meQueryOptions())
  const signOut = useSignOut()
  const [tab, setTab] = useTab()

  let body: React.ReactNode
  if (me.isPending) {
    body = (
      <main className="mx-auto w-full max-w-[1080px] p-4 sm:p-7">
        <Skeleton className="h-[240px] rounded-xl" />
      </main>
    )
  } else if (!me.data) {
    body = <SignIn />
  } else {
    body = (
      <>
        <header className="flex items-center justify-between gap-3 border-b bg-card px-4 py-2.5 sm:px-7">
          <span className="font-extrabold">Relay CRM</span>
          <div className="flex items-center gap-3 text-sm">
            <span className="truncate text-muted-foreground">{me.data.user.email}</span>
            <Button
              size="sm"
              variant="ghost"
              disabled={signOut.isPending}
              onClick={() => signOut.mutate()}
            >
              Sign out
            </Button>
          </div>
        </header>
        <AdminPage tab={tab} onTab={setTab} />
      </>
    )
  }
  return (
    <>
      {body}
      <Toaster theme="system" position="bottom-right" />
    </>
  )
}
