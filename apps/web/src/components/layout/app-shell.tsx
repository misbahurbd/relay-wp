import { Link, useLocation } from '@tanstack/react-router'
import {
  BarChart3Icon,
  BotIcon,
  CheckIcon,
  ChevronsUpDownIcon,
  ContactIcon,
  CreditCardIcon,
  FileTextIcon,
  InboxIcon,
  LayoutDashboardIcon,
  LogOutIcon,
  MegaphoneIcon,
  PlugIcon,
  SettingsIcon,
  UsersIcon,
  WorkflowIcon,
} from 'lucide-react'
import type { ReactNode } from 'react'
import { Avatar, AvatarFallback, AvatarImage } from '#/components/ui/avatar'
import { Badge } from '#/components/ui/badge'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarHeader,
  SidebarInset,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarProvider,
  SidebarTrigger,
} from '#/components/ui/sidebar'
import { LogoMark } from './logo'
import { ThemeToggle } from './theme-toggle'

type NavItem = {
  id: string
  label: string
  icon: typeof InboxIcon
  /** Built sections link somewhere; the rest show "Soon". */
  to?: '/' | '/inbox' | '/contacts' | '/templates' | '/integrations' | '/team' | '/billing'
}

const NAV: NavItem[] = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboardIcon, to: '/' },
  { id: 'inbox', label: 'Inbox', icon: InboxIcon, to: '/inbox' },
  { id: 'contacts', label: 'Contacts', icon: ContactIcon, to: '/contacts' },
  { id: 'broadcasts', label: 'Broadcasts', icon: MegaphoneIcon },
  { id: 'templates', label: 'Templates', icon: FileTextIcon, to: '/templates' },
  { id: 'chatbots', label: 'Chatbots', icon: BotIcon },
  { id: 'automations', label: 'Automations', icon: WorkflowIcon },
  { id: 'integrations', label: 'Integrations', icon: PlugIcon, to: '/integrations' },
  { id: 'analytics', label: 'Analytics', icon: BarChart3Icon },
  { id: 'team', label: 'Team', icon: UsersIcon, to: '/team' },
  { id: 'settings', label: 'Settings', icon: SettingsIcon },
  { id: 'billing', label: 'Billing', icon: CreditCardIcon, to: '/billing' },
]

type WorkspaceOption = { id: string; name: string; roleName: string }

type Props = {
  workspace: { id: string; name: string; trialDaysLeft: number | null }
  /** Every workspace the user belongs to, for the switcher. */
  workspaces: WorkspaceOption[]
  onSwitchWorkspace: (id: string) => void
  user: { name: string; email: string; avatarUrl: string | null; roleName?: string }
  onLogout: () => void
  children: ReactNode
}

export function AppShell({
  workspace,
  workspaces,
  onSwitchWorkspace,
  user,
  onLogout,
  children,
}: Props) {
  const { pathname } = useLocation()
  // Dashboard is active only on its own page; other sections own everything below them.
  const isActive = (to: string) => (to === '/' ? pathname === '/' : pathname.startsWith(to))
  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader className="p-3">
          <WorkspaceSwitcher
            current={workspace}
            workspaces={workspaces}
            onSwitch={onSwitchWorkspace}
          />
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                {NAV.map((item) => (
                  <SidebarMenuItem key={item.id} data-tour={item.id}>
                    {item.to ? (
                      <SidebarMenuButton
                        asChild
                        isActive={isActive(item.to)}
                        size="lg"
                        className="h-10 font-bold"
                      >
                        <Link to={item.to}>
                          <item.icon aria-hidden="true" />
                          {item.label}
                        </Link>
                      </SidebarMenuButton>
                    ) : (
                      <SidebarMenuButton
                        aria-disabled="true"
                        className="h-10 cursor-default font-bold opacity-70 hover:bg-transparent"
                        tooltip={`${item.label} is coming soon`}
                      >
                        <item.icon aria-hidden="true" />
                        <span className="flex-1">{item.label}</span>
                        <Badge variant="neutral" className="text-[10px]">
                          Soon
                        </Badge>
                      </SidebarMenuButton>
                    )}
                  </SidebarMenuItem>
                ))}
              </SidebarMenu>
            </SidebarGroupContent>
          </SidebarGroup>
        </SidebarContent>
        {workspace.trialDaysLeft !== null && (
          <SidebarFooter className="p-3">
            <div className="flex flex-col gap-1 rounded-lg bg-brand-soft p-3 text-sm">
              <span className="font-extrabold text-brand-text">Growth plan · free trial</span>
              <span className="text-muted-foreground">
                {workspace.trialDaysLeft} {workspace.trialDaysLeft === 1 ? 'day' : 'days'} left
              </span>
            </div>
          </SidebarFooter>
        )}
      </Sidebar>
      <SidebarInset className="min-w-0 bg-background">
        <header className="sticky top-0 z-10 flex h-16 items-center gap-3 border-b bg-card/95 px-4 backdrop-blur sm:px-6">
          <SidebarTrigger className="size-11" aria-label="Toggle navigation" />
          <div className="flex-1" />
          <ThemeToggle />
          <AccountMenu user={user} onLogout={onLogout} />
        </header>
        {children}
      </SidebarInset>
    </SidebarProvider>
  )
}

function WorkspaceSwitcher({
  current,
  workspaces,
  onSwitch,
}: {
  current: Props['workspace']
  workspaces: WorkspaceOption[]
  onSwitch: (id: string) => void
}) {
  const label = (
    <>
      <LogoMark className="size-8 shrink-0" />
      <span className="flex min-w-0 flex-1 flex-col text-left leading-tight">
        <span className="truncate font-extrabold">{current.name}</span>
        <span className="text-xs text-muted-foreground">Relay</span>
      </span>
    </>
  )
  // Nothing to switch to: show the name without a menu.
  if (workspaces.length < 2)
    return <div className="flex items-center gap-2.5 px-1 py-1">{label}</div>
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="flex min-h-11 w-full items-center gap-2.5 rounded-lg px-1 py-1 hover:bg-sidebar-accent"
        aria-label={`Workspace: ${current.name}. Switch workspace`}
      >
        {label}
        <ChevronsUpDownIcon className="size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
      </DropdownMenuTrigger>
      <DropdownMenuContent
        align="start"
        className="w-(--radix-dropdown-menu-trigger-width) min-w-60"
      >
        <DropdownMenuLabel className="text-xs font-normal text-muted-foreground">
          Workspaces
        </DropdownMenuLabel>
        {workspaces.map((w) => (
          <DropdownMenuItem key={w.id} onSelect={() => onSwitch(w.id)} className="gap-2.5">
            <span className="flex min-w-0 flex-1 flex-col leading-tight">
              <span className="truncate font-bold">{w.name}</span>
              <span className="text-xs text-muted-foreground">{w.roleName}</span>
            </span>
            {w.id === current.id && <CheckIcon className="size-4" aria-label="Current workspace" />}
          </DropdownMenuItem>
        ))}
      </DropdownMenuContent>
    </DropdownMenu>
  )
}

function AccountMenu({ user, onLogout }: Pick<Props, 'user' | 'onLogout'>) {
  const initials = user.name
    .split(/\s+/)
    .map((p) => p[0])
    .join('')
    .slice(0, 2)
    .toUpperCase()
  return (
    <DropdownMenu>
      <DropdownMenuTrigger
        className="flex h-11 items-center gap-2.5 rounded-lg px-2 hover:bg-accent"
        aria-label="Account menu"
      >
        <Avatar className="size-8">
          {user.avatarUrl && <AvatarImage src={user.avatarUrl} alt="" />}
          <AvatarFallback className="bg-brand-soft text-xs font-extrabold text-brand-text">
            {initials}
          </AvatarFallback>
        </Avatar>
        <span className="hidden text-left leading-tight sm:flex sm:flex-col">
          <span className="text-sm font-bold">{user.name}</span>
          {user.roleName && <span className="text-xs text-muted-foreground">{user.roleName}</span>}
        </span>
      </DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="w-60">
        <DropdownMenuLabel className="flex flex-col">
          <span>{user.name}</span>
          <span className="truncate text-xs font-normal text-muted-foreground">{user.email}</span>
        </DropdownMenuLabel>
        <DropdownMenuSeparator />
        <DropdownMenuItem onSelect={onLogout}>
          <LogOutIcon aria-hidden="true" />
          Log out
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
