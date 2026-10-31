import { Button } from '#/components/ui/button'
import { ContactDetails } from '#/features/contacts'
import { useAccess } from '#/features/team'
import { cn } from '#/lib/utils'
import { useConversationActions } from '../../hooks/use-conversation-actions'
import { CHANNEL_COPY, channelAccount } from '../../schemas/inbox.schema'
import type { ConversationDetail, Member } from '../../types'
import { AssignMenu } from '../kit/assign-menu'
import { Avatar } from '../kit/avatar'
import { ChannelIcon } from '../kit/channel-badge'

const sectionTitle = 'text-xs font-extrabold tracking-[0.05em] text-muted-foreground uppercase'

const OPT_IN = {
  in: { label: 'Opted in to marketing', tone: 'bg-brand-soft text-brand-text' },
  out: { label: 'Opted out of marketing', tone: 'bg-destructive-soft text-destructive' },
  pending: { label: 'Marketing opt-in pending', tone: 'bg-warning-soft text-warning' },
}

const SOURCE = { organic: 'Messaged first', ctwa_ad: 'Click-to-WhatsApp ad', template: 'Template' }

/**
 * "Customer details" (F11): who the customer is, who has the chat, and the contact's tags and
 * fields from the Contacts feature. Past orders wait for Commerce.
 */
export function CustomerPanel({
  workspaceId,
  conversation: c,
  me,
  members,
  className,
}: {
  workspaceId: string
  conversation: ConversationDetail
  me: { id: string; name: string }
  members: Member[]
  className?: string
}) {
  const { can } = useAccess(workspaceId)
  const actions = useConversationActions(workspaceId, c, me)
  const name = c.contact.name || c.contact.phone
  const optIn = OPT_IN[c.contact_opt_in]
  const assignee = c.assignee
    ? c.assignee.id === me.id
      ? `${c.assignee.name} (you)`
      : c.assignee.name
    : 'Unassigned'

  const facts = [
    {
      k: 'Conversation via',
      v: (
        <span className="inline-flex items-center gap-1">
          <ChannelIcon type={c.channel.type} className="size-3.5" />
          {CHANNEL_COPY[c.channel.type].label} · {channelAccount(c.channel)}
        </span>
      ),
    },
    { k: 'Team', v: c.team_name ?? '—' },
    { k: 'Came from', v: SOURCE[c.source] },
  ]

  return (
    <aside
      aria-label="Customer details"
      className={cn(
        'flex min-h-0 flex-col gap-5 overflow-y-auto bg-card px-[18px] py-5',
        className,
      )}
    >
      <div className="flex flex-col items-center gap-2 text-center">
        <Avatar name={name} seed={c.contact.id} size="lg" />
        <div className="flex flex-col leading-tight">
          <span className="text-[17px] font-extrabold break-words">{name}</span>
          <span className="text-muted-foreground tabular-nums">{c.contact.phone}</span>
        </div>
        <span
          className={cn(
            'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-bold',
            optIn.tone,
          )}
        >
          <span aria-hidden="true" className="size-1.5 rounded-full bg-current" />
          {optIn.label}
        </span>
      </div>

      <section aria-labelledby="panel-agent" className="flex flex-col gap-2">
        <h2 id="panel-agent" className={sectionTitle}>
          Assigned agent
        </h2>
        {can('inbox.close_reassign') ? (
          <AssignMenu members={members} me={me} assignee={c.assignee} onAssign={actions.assign}>
            <Button
              variant="unstyled"
              type="button"
              className="flex min-h-11 items-center gap-2.5 rounded-lg border px-3 text-left hover:bg-muted"
            >
              <AssigneeAvatar assignee={c.assignee} />
              <span className="min-w-0 flex-1 truncate text-sm font-semibold">{assignee}</span>
              <span className="text-xs font-bold text-brand-text">Change</span>
            </Button>
          </AssignMenu>
        ) : (
          <div className="flex min-h-11 items-center gap-2.5 rounded-lg border px-3">
            <AssigneeAvatar assignee={c.assignee} />
            <span className="text-sm font-semibold">{assignee}</span>
          </div>
        )}
      </section>

      <dl className="flex flex-col">
        {facts.map((f) => (
          <div
            key={f.k}
            className="flex justify-between gap-3 border-b py-[7px] text-[13px] last:border-b-0"
          >
            <dt className="text-muted-foreground">{f.k}</dt>
            <dd className="min-w-0 text-right font-semibold break-words">{f.v || '—'}</dd>
          </div>
        ))}
      </dl>

      {can('contacts.view') && (
        <ContactDetails
          workspaceId={workspaceId}
          contactId={c.contact.id}
          canEdit={can('contacts.edit')}
          summary={false}
        />
      )}
    </aside>
  )
}

function AssigneeAvatar({ assignee }: { assignee: ConversationDetail['assignee'] }) {
  if (!assignee)
    return (
      <span
        aria-hidden="true"
        className="flex size-8 items-center justify-center rounded-full bg-muted text-xs font-extrabold"
      >
        ?
      </span>
    )
  return <Avatar name={assignee.name} seed={assignee.id} size="xs" className="size-8 text-xs" />
}
