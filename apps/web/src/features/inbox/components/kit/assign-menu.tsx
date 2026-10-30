import type { ReactNode } from 'react'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import type { Member, Person } from '../../types'
import { Avatar } from './avatar'

const NONE = 'unassigned'

/** "Assign to": Unassigned, you, then every member who can work chats. */
export function AssignMenu({
  members,
  me,
  assignee,
  onAssign,
  children,
}: {
  members: Member[]
  me: { id: string; name: string }
  assignee: Person | null
  onAssign: (member: Member | null) => void
  children: ReactNode
}) {
  const others = members.filter((m) => m.id !== me.id && m.role !== 'viewer')
  const options: { value: string; label: string; member: Member | null }[] = [
    { value: NONE, label: 'Unassigned', member: null },
    { value: me.id, label: `${me.name} (you)`, member: { id: me.id, name: me.name, role: '' } },
    ...others.map((m) => ({ value: m.id, label: m.name, member: m })),
  ]
  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>{children}</DropdownMenuTrigger>
      <DropdownMenuContent align="end" className="max-h-80 w-60 overflow-y-auto">
        <DropdownMenuLabel className="text-[11px] font-extrabold tracking-wider text-muted-foreground uppercase">
          Assign to
        </DropdownMenuLabel>
        <DropdownMenuRadioGroup
          value={assignee?.id ?? NONE}
          onValueChange={(v) => onAssign(options.find((o) => o.value === v)?.member ?? null)}
        >
          {options.map((o) => (
            <DropdownMenuRadioItem key={o.value} value={o.value} className="min-h-10 gap-2">
              {o.member ? (
                <Avatar name={o.member.name} seed={o.member.id} size="xs" />
              ) : (
                <span
                  aria-hidden="true"
                  className="flex size-6 items-center justify-center rounded-full bg-muted text-[10px] font-extrabold"
                >
                  ?
                </span>
              )}
              <span className="truncate">{o.label}</span>
            </DropdownMenuRadioItem>
          ))}
        </DropdownMenuRadioGroup>
      </DropdownMenuContent>
    </DropdownMenu>
  )
}
