import { useQuery } from '@tanstack/react-query'
import { createFileRoute, useLocation, useNavigate } from '@tanstack/react-router'
import { Logo } from '#/components/layout/logo'
import { ThemeToggle } from '#/components/layout/theme-toggle'
import { fetchMe, meQueryOptions, SocialButtons, useLogout } from '#/features/auth'
import { InviteAccept, inviteTokenFromLink } from '#/features/team'
import { selectWorkspace } from '#/features/workspaces'

// The link from an invite email (/invite#t=…). Works signed out; joining needs sign-in.
export const Route = createFileRoute('/invite')({
  ssr: false,
  head: () => ({ meta: [{ title: "You're invited · Relay" }] }),
  beforeLoad: async ({ context }) => {
    await fetchMe(context.queryClient)
  },
  component: InvitePage,
})

function InvitePage() {
  const { hash } = useLocation()
  const navigate = useNavigate()
  const logout = useLogout()
  const me = useQuery({ ...meQueryOptions(), enabled: false })
  const token = inviteTokenFromLink(hash)
  const signedInAs = me.data?.user.email_verified ? me.data.user.email : null
  return (
    <div className="flex min-h-dvh flex-col bg-background">
      <header className="flex items-center justify-between gap-3 border-b bg-card px-[clamp(16px,4vw,40px)] py-[18px]">
        <Logo to="/login" />
        <ThemeToggle />
      </header>
      <main className="mx-auto flex w-full max-w-[560px] flex-1 flex-col px-4 py-8">
        <InviteAccept
          token={token}
          signedInAs={signedInAs}
          social={<SocialButtons action="log in" redirect="/invite" divider="or set a password" />}
          onOpenWorkspace={(id) => {
            selectWorkspace(id)
            navigate({ to: '/' })
          }}
          onSwitchAccount={() =>
            logout.mutate(
              {},
              { onSettled: () => navigate({ to: '/login', search: { redirect: '/invite' } }) },
            )
          }
        />
      </main>
    </div>
  )
}
