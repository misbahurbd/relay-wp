import {
  ArrowLeftIcon,
  BellOffIcon,
  CheckCircle2Icon,
  ChevronDownIcon,
  PanelRightIcon,
  RotateCcwIcon,
} from 'lucide-react'
import { Button } from '#/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import { useAccess } from '#/features/team'
import { useConversationActions } from '../../hooks/use-conversation-actions'
import { CHANNEL_COPY, STATUS_LABEL } from '../../schemas/inbox.schema'
import type { ConversationDetail, Member } from '../../types'
import { AssignMenu } from '../kit/assign-menu'
import { Avatar } from '../kit/avatar'
import { ChannelBadge } from '../kit/channel-badge'
import { assigneeShort, formatWhen, snoozeOptions } from '../kit/format'

/** Name, channel, phone and status, plus assign, snooze and close / reopen (F7). */
export function ThreadHeader({
  workspaceId,
  conversation: c,
  me,
  members,
  now,
  onBack,
  onShowDetails,
}: {
  workspaceId: string
  conversation: ConversationDetail
  me: { id: string; name: string }
  members: Member[]
  now: number
  onBack: () => void
  onShowDetails: () => void
}) {
  const { can } = useAccess(workspaceId)
  const actions = useConversationActions(workspaceId, c, me)
  const canReassign = can('inbox.close_reassign')
  const mine = c.assignee?.id === me.id
  const name = c.contact.name || c.contact.phone
  const snoozed = c.status === 'pending' && c.snoozed_until

  return (
    <div className="flex flex-wrap items-center gap-2.5 border-b bg-card px-3 py-3 sm:px-[18px]">
      <Button
        variant="ghost"
        size="icon-sm"
        className="md:hidden"
        aria-label="Back to chats"
        onClick={onBack}
      >
        <ArrowLeftIcon aria-hidden="true" />
      </Button>
      <Avatar name={name} seed={c.contact.id} size="sm" />
      <div className="flex min-w-0 flex-[1_1_150px] flex-col leading-tight">
        <span className="flex min-w-0 items-center gap-2">
          <h2 className="truncate text-[15px] font-bold">{name}</h2>
          <ChannelBadge channel={c.channel} className="hidden sm:inline-flex" />
        </span>
        <span className="truncate text-xs text-muted-foreground">
          <span className="sm:hidden">{CHANNEL_COPY[c.channel.type].label} · </span>
          {c.contact.phone} · {STATUS_LABEL[c.status]}
          {snoozed && ` · Snoozed until ${formatWhen(snoozed, now)}`}
        </span>
      </div>

      <div className="flex flex-wrap items-center gap-2">
        {canReassign ? (
          <AssignMenu members={members} me={me} assignee={c.assignee} onAssign={actions.assign}>
            <Button
              variant="outline"
              size="sm"
              className="h-10 gap-2"
              aria-label={`Assigned to ${c.assignee?.name ?? 'nobody'}. Change`}
            >
              <AssigneeDot assignee={c.assignee} />
              {c.assignee ? assigneeShort(c.assignee, me.id) : 'Assign'}
              <ChevronDownIcon className="size-3.5" aria-hidden="true" />
            </Button>
          </AssignMenu>
        ) : !c.assignee && can('inbox.reply') ? (
          // Agents may always take an unassigned chat themselves (AD-10).
          <Button
            variant="outline"
            size="sm"
            className="h-10"
            disabled={actions.pending}
            onClick={() => actions.assign({ id: me.id, name: me.name, role: '' })}
          >
            Assign to me
          </Button>
        ) : (
          <span className="inline-flex h-10 items-center gap-2 rounded-lg border px-3 text-[13px] font-semibold">
            <AssigneeDot assignee={c.assignee} />
            {assigneeShort(c.assignee, me.id)}
          </span>
        )}

        {canReassign && c.status !== 'closed' && (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button
                variant="outline"
                size="sm"
                className="h-10 gap-1.5"
                disabled={actions.pending}
              >
                <BellOffIcon className="size-4" aria-hidden="true" />
                Snooze
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end" aria-label="Snooze chat">
              {snoozeOptions(new Date(now)).map((o) => (
                <DropdownMenuItem
                  key={o.label}
                  className="min-h-10"
                  onSelect={() => actions.snooze(o)}
                >
                  {o.label}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}

        {c.status === 'closed'
          ? (canReassign || mine) && (
              <Button
                size="sm"
                className="h-10 bg-info-soft text-info hover:bg-info-soft/80"
                disabled={actions.pending}
                onClick={actions.reopen}
              >
                <RotateCcwIcon className="size-4" aria-hidden="true" />
                Reopen chat
              </Button>
            )
          : (canReassign || mine) && (
              <Button
                variant="soft"
                size="sm"
                className="h-10"
                disabled={actions.pending}
                onClick={actions.close}
              >
                <CheckCircle2Icon className="size-4" aria-hidden="true" />
                Close
              </Button>
            )}

        <Button
          variant="ghost"
          size="icon-sm"
          className="xl:hidden"
          aria-label="Customer details"
          onClick={onShowDetails}
        >
          <PanelRightIcon aria-hidden="true" />
        </Button>
      </div>
    </div>
  )
}

function AssigneeDot({ assignee }: { assignee: ConversationDetail['assignee'] }) {
  if (!assignee)
    return (
      <span
        aria-hidden="true"
        className="flex size-[22px] items-center justify-center rounded-full bg-muted text-[10px] font-extrabold"
      >
        ?
      </span>
    )
  return <Avatar name={assignee.name} seed={assignee.id} size="xs" className="size-[22px]" />
}
