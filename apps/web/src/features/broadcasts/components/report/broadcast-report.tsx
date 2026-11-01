import { Link, useNavigate } from '@tanstack/react-router'
import {
  ArrowLeftIcon,
  CheckCheckIcon,
  CopyIcon,
  EyeIcon,
  MessageCircleReplyIcon,
  SendIcon,
  UsersIcon,
  XCircleIcon,
} from 'lucide-react'
import { useState } from 'react'
import { toast } from 'sonner'
import { Alert, AlertDescription } from '#/components/ui/alert'
import { Button } from '#/components/ui/button'
import { Progress } from '#/components/ui/progress'
import { useAccess } from '#/features/team'
import { useCancelBroadcast, useDuplicateBroadcast } from '../../api/broadcasts.mutations'
import { FAILURE_LABEL, percent } from '../../schemas/broadcasts.schema'
import type { Broadcast, RecipientFilter } from '../../types'
import { ActionError } from '../kit/action-error'
import { formatCount, formatWhen } from '../kit/format'
import { StatCard } from '../kit/stat-card'
import { StatusBadge } from '../kit/status-badge'
import { RecipientsTable } from './recipients-table'

/** A broadcast that started: progress while it sends, then how it did and who got it. */
export function BroadcastReport({
  workspaceId,
  broadcast: b,
}: {
  workspaceId: string
  broadcast: Broadcast
}) {
  const navigate = useNavigate()
  const { can } = useAccess(workspaceId)
  const cancel = useCancelBroadcast(workspaceId)
  const duplicate = useDuplicateBroadcast(workspaceId)
  const [filter, setFilter] = useState<RecipientFilter>()
  const s = b.stats
  const total = b.recipients_total
  const handled = total - s.pending - s.queued
  const sending = b.status === 'sending'

  return (
    <main className="mx-auto flex w-full max-w-[1320px] flex-col gap-5 p-4 sm:p-7">
      <div className="flex flex-wrap items-end justify-between gap-4">
        <div className="flex min-w-0 flex-col gap-1">
          <Link
            to="/broadcasts"
            className="inline-flex items-center gap-1.5 text-[13px] font-bold text-muted-foreground no-underline hover:text-foreground"
          >
            <ArrowLeftIcon className="size-4" aria-hidden="true" />
            Broadcasts
          </Link>
          <div className="flex flex-wrap items-center gap-3">
            <h1 className="text-[28px] font-extrabold tracking-[-0.02em] break-words">{b.name}</h1>
            <StatusBadge status={b.status} />
          </div>
          <p className="text-[15px] text-muted-foreground">
            {b.template?.name ?? 'Template deleted'} · started {formatWhen(b.started_at)}
            {b.completed_at && ` · finished ${formatWhen(b.completed_at)}`}
          </p>
        </div>
        <div className="flex flex-wrap gap-2.5">
          {sending && can('broadcasts.send') && (
            <Button
              variant="outline"
              className="text-destructive"
              disabled={cancel.isPending}
              onClick={() =>
                cancel.mutate(b.id, { onSuccess: () => toast.success('Broadcast cancelled') })
              }
            >
              <XCircleIcon aria-hidden="true" />
              Cancel
            </Button>
          )}
          {can('broadcasts.create') && (
            <Button
              variant="outline"
              disabled={duplicate.isPending}
              onClick={() =>
                duplicate.mutate(b.id, {
                  onSuccess: (copy) =>
                    navigate({ to: '/broadcasts/$broadcastId', params: { broadcastId: copy.id } }),
                })
              }
            >
              <CopyIcon aria-hidden="true" />
              Duplicate
            </Button>
          )}
        </div>
      </div>

      {(cancel.error || duplicate.error) && <ActionError error={cancel.error ?? duplicate.error} />}
      {b.status === 'failed' && (
        <Alert variant="danger">
          <AlertDescription>
            {FAILURE_LABEL[b.failure_reason ?? ''] ?? 'This broadcast stopped.'} Contacts it hadn’t
            reached were skipped.
          </AlertDescription>
        </Alert>
      )}
      {sending && (
        <section
          aria-label="Progress"
          className="flex flex-col gap-2 rounded-xl border bg-card p-5 shadow-card"
        >
          <div className="flex justify-between text-sm">
            <span className="font-bold">Sending…</span>
            <span className="tabular-nums text-muted-foreground">
              {formatCount(Math.max(handled, 0))} of {formatCount(total)}
            </span>
          </div>
          <Progress value={percent(Math.max(handled, 0), total)} aria-label="Sent so far" />
          <span className="text-[13px] text-muted-foreground">
            This page updates on its own. Messages already queued still go if you cancel.
          </span>
        </section>
      )}

      <section aria-label="Results" className="grid gap-3 sm:grid-cols-2 lg:grid-cols-5">
        <StatCard
          icon={<UsersIcon aria-hidden="true" />}
          tone="bg-muted text-foreground"
          value={formatCount(total)}
          label="Recipients"
        />
        <StatCard
          icon={<SendIcon aria-hidden="true" />}
          tone="bg-info-soft text-info"
          value={formatCount(s.sent)}
          label="Sent"
          detail={`${percent(s.sent, total)}%`}
        />
        <StatCard
          icon={<CheckCheckIcon aria-hidden="true" />}
          tone="bg-brand-soft text-brand-text"
          value={formatCount(s.delivered)}
          label="Delivered"
          detail={`${percent(s.delivered, total)}%`}
        />
        <StatCard
          icon={<EyeIcon aria-hidden="true" />}
          tone="bg-brand-soft text-brand-text"
          value={formatCount(s.read)}
          label="Read"
          detail={`${percent(s.read, total)}%`}
        />
        <StatCard
          icon={<MessageCircleReplyIcon aria-hidden="true" />}
          tone="bg-warning-soft text-warning"
          value={formatCount(s.replied)}
          label="Replied"
          detail={`${percent(s.replied, total)}%`}
        />
      </section>

      {b.failures.length > 0 && (
        <section
          aria-labelledby="bc-failures"
          className="flex flex-col gap-3 rounded-xl border bg-card p-5 shadow-card sm:p-6"
        >
          <h2 id="bc-failures" className="text-base font-extrabold">
            Not delivered · {formatCount(s.failed + s.skipped)}
          </h2>
          <ul className="flex flex-col gap-2">
            {b.failures.map((f) => (
              <li key={f.reason} className="flex justify-between gap-3 text-sm">
                <span>{FAILURE_LABEL[f.reason] ?? f.reason}</span>
                <span className="font-bold tabular-nums">{formatCount(f.recipients)}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      <RecipientsTable
        workspaceId={workspaceId}
        broadcastId={b.id}
        filter={filter}
        onFilter={setFilter}
      />
    </main>
  )
}
