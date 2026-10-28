import { Link } from '@tanstack/react-router'
import {
  BarChart3Icon,
  BotIcon,
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

type NavItem = { id: string; label: string; icon: typeof InboxIcon; ready?: boolean }

const NAV: NavItem[] = [
  { id: 'dashboard', label: 'Dashboard', icon: LayoutDashboardIcon, ready: true },
  { id: 'inbox', label: 'Inbox', icon: InboxIcon },
  { id: 'contacts', label: 'Contacts', icon: ContactIcon },
  { id: 'broadcasts', label: 'Broadcasts', icon: MegaphoneIcon },
  { id: 'templates', label: 'Templates', icon: FileTextIcon },
  { id: 'chatbots', label: 'Chatbots', icon: BotIcon },
  { id: 'automations', label: 'Automations', icon: WorkflowIcon },
  { id: 'integrations', label: 'Integrations', icon: PlugIcon },
  { id: 'analytics', label: 'Analytics', icon: BarChart3Icon },
  { id: 'team', label: 'Team', icon: UsersIcon },
  { id: 'settings', label: 'Settings', icon: SettingsIcon },
  { id: 'billing', label: 'Billing', icon: CreditCardIcon },
]

type Props = {
  workspace: { id: string; name: string; trialDaysLeft: number | null }
  user: { name: string; email: string; avatarUrl: string | null }
  onLogout: () => void
  children: ReactNode
}

export function AppShell({ workspace, user, onLogout, children }: Props) {
  return (
    <SidebarProvider>
      <Sidebar>
        <SidebarHeader className="p-3">
          <div className="flex items-center gap-2.5 px-1 py-1">
            <LogoMark className="size-8" />
            <span className="flex min-w-0 flex-col leading-tight">
              <span className="truncate font-extrabold">{workspace.name}</span>
              <span className="text-xs text-muted-foreground">Relay</span>
            </span>
          </div>
        </SidebarHeader>
        <SidebarContent>
          <SidebarGroup>
            <SidebarGroupContent>
              <SidebarMenu>
                {NAV.map((item) => (
                  <SidebarMenuItem key={item.id} data-tour={item.id}>
                    {item.ready ? (
                      <SidebarMenuButton asChild isActive size="lg" className="h-10 font-bold">
                        <Link to="/w/$workspaceId" params={{ workspaceId: workspace.id }}>
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
          <span className="text-xs text-muted-foreground">Owner</span>
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
