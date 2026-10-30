import { useQuery } from '@tanstack/react-query'
import { FileTextIcon, Loader2Icon, SendIcon, Settings2Icon, ZapIcon } from 'lucide-react'
import { useRef, useState } from 'react'
import { toast } from 'sonner'
import { Button } from '#/components/ui/button'
import { useAccess } from '#/features/team'
import { isApiError } from '#/lib/api'
import { cn } from '#/lib/utils'
import { useAddNote, useSendMessage, useUploadMedia } from '../../api/inbox.mutations'
import { quickRepliesQueryOptions } from '../../api/inbox.queries'
import { useDraft } from '../../hooks/use-draft'
import { useSuggestions } from '../../hooks/use-suggestions'
import {
  type AttachOption,
  attachOptions,
  CHANNEL_COPY,
  uploadProblem,
} from '../../schemas/inbox.schema'
import type { ConversationDetail, Member, Message, Upload } from '../../types'
import { inboxErrorMessage } from '../kit/errors'
import { firstName } from '../kit/format'
import type { WindowState } from '../kit/window'
import { AttachmentBar, ClosedBanner, ReplyBar } from './composer-bars'
import { AttachMenu, EmojiPicker } from './composer-tools'
import { QuickRepliesDialog } from './quick-replies-dialog'
import { SuggestionMenu } from './suggestion-menu'

const REPLY_PLACEHOLDER =
  'Type a message… Enter to send, Shift+Enter for a new line, / for quick replies'
const NOTE_PLACEHOLDER = 'Write a private note. Only your team sees this. Type @ to mention.'

/** Reply or private note (F4, F6, F8): quick replies, mentions, emoji, files and templates. */
export function Composer({
  workspaceId,
  conversation: c,
  window: win,
  me,
  members,
  replyTo,
  onClearReply,
  onOpenTemplate,
}: {
  workspaceId: string
  conversation: ConversationDetail
  window: WindowState
  me: { id: string; name: string }
  members: Member[]
  replyTo: Message | null
  onClearReply: () => void
  onOpenTemplate: () => void
}) {
  const { can, access } = useAccess(workspaceId)
  const canReply = can('inbox.reply')
  const canNote = can('inbox.notes')
  const caps = c.capabilities
  const { metaPricing } = CHANNEL_COPY[c.channel.type]
  // Templates need the permission and a channel that has them (ADR 0010).
  const canTemplate = can('inbox.send_template') && caps.templates
  const attachments = attachOptions(caps.media)
  const [chosen, setMode] = useState<'reply' | 'note'>('reply')
  // A member who can only write notes starts (and stays) in note mode.
  const mode = canReply || canTemplate || !canNote ? chosen : 'note'
  const [text, setText] = useDraft(c.id)
  const [caret, setCaret] = useState(0)
  const [mentioned, setMentioned] = useState<Member[]>([])
  const [attachment, setAttachment] = useState<Upload | null>(null)
  const [managing, setManaging] = useState(false)
  const input = useRef<HTMLTextAreaElement>(null)

  const { data: quickReplies = [] } = useQuery(quickRepliesQueryOptions(workspaceId))
  const send = useSendMessage(workspaceId)
  const note = useAddNote(workspaceId)
  const upload = useUploadMedia(workspaceId)
  const suggest = useSuggestions({ text, caret, mode, quickReplies, members, meId: me.id })

  const isNote = mode === 'note'
  const first = win.first
  const canType = isNote ? canNote : canReply && win.open
  const busy = send.isPending || note.isPending
  const hasContent = text.trim().length > 0 || (!isNote && !!attachment)
  const maxLength = isNote ? 0 : caps.max_text_length
  const length = text.trim().length
  const tooLong = maxLength > 0 && length > maxLength
  // The counter shows once the text nears the channel's limit.
  const showCount = maxLength > 0 && length >= maxLength * 0.9

  if (!access) return null
  if (!canReply && !canNote && !canTemplate) {
    return (
      <p className="border-t bg-card px-4 py-3 text-[13px] text-muted-foreground">
        You can read this chat, but your role can’t reply to it.
      </p>
    )
  }

  const focusAt = (pos: number) =>
    requestAnimationFrame(() => {
      input.current?.focus()
      input.current?.setSelectionRange(pos, pos)
      setCaret(pos)
    })

  const replaceAll = (next: string) => {
    setText(next)
    suggest.reset()
    focusAt(next.length)
  }

  const insertAtCaret = (snippet: string) => {
    const at = input.current?.selectionStart ?? text.length
    const next = text.slice(0, at) + snippet + text.slice(input.current?.selectionEnd ?? at)
    setText(next)
    focusAt(at + snippet.length)
  }

  const pickMention = (member: Member) => {
    if (suggest.state.kind !== 'mention') return
    const { start } = suggest.state
    const token = `@${member.name} `
    const next = text.slice(0, start) + token + text.slice(caret)
    setText(next)
    setMentioned((list) => (list.some((m) => m.id === member.id) ? list : [...list, member]))
    suggest.reset()
    focusAt(start + token.length)
  }

  const onFile = (file: File, option: AttachOption) => {
    const problem = uploadProblem(file, option)
    if (problem) {
      toast.error(problem)
      return
    }
    upload.mutate(file, {
      onSuccess: setAttachment,
      onError: (e) => toast.error(inboxErrorMessage(e, 'The upload failed. Please try again.')),
    })
  }

  const submit = () => {
    if (!canType || busy || !hasContent || tooLong) return
    const body = text.trim()
    if (isNote) {
      const mentions = mentioned.filter((m) => body.includes(`@${m.name}`))
      note.mutate(
        { conversationId: c.id, text: body, mentions: mentions.map((m) => m.id) },
        {
          onSuccess: () => {
            setText('')
            setMentioned([])
            if (mentions.length > 0) {
              toast.success(
                `Note saved · ${mentions.map((m) => firstName(m.name)).join(', ')} notified`,
              )
            }
          },
          onError: (e) => toast.error(inboxErrorMessage(e)),
        },
      )
      return
    }
    send.mutate(
      {
        conversationId: c.id,
        body: attachment
          ? {
              type: 'media',
              media_key: attachment.key,
              caption: body || undefined,
              reply_to_id: replyTo?.id,
            }
          : { type: 'text', text: body, reply_to_id: replyTo?.id },
      },
      {
        onSuccess: () => {
          setText('')
          setAttachment(null)
          onClearReply()
        },
        onError: (e) =>
          toast.error(
            isApiError(e) && e.code === 'window_closed'
              ? `The ${win.label} has closed. ${
                  win.templates
                    ? 'Send an approved template instead.'
                    : `You can reply once ${first} writes again.`
                }`
              : inboxErrorMessage(e),
          ),
      },
    )
  }

  const menu: typeof suggest.state = canType ? suggest.state : { kind: 'none' }
  const menuOpen = menu.kind === 'slash' || (menu.kind === 'mention' && menu.items.length > 0)
  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (e.nativeEvent.isComposing) return
    if (menuOpen && (e.key === 'ArrowDown' || e.key === 'ArrowUp')) {
      e.preventDefault()
      suggest.move(e.key === 'ArrowDown' ? 1 : -1)
      return
    }
    if (menuOpen && e.key === 'Escape') {
      e.preventDefault()
      suggest.dismiss()
      return
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (menu.kind === 'slash') {
        const pick = menu.items[suggest.active]
        if (pick) replaceAll(pick.body)
        return
      }
      if (menu.kind === 'mention' && menu.items[suggest.active]) {
        pickMention(menu.items[suggest.active])
        return
      }
      submit()
    }
  }

  const placeholder = isNote
    ? NOTE_PLACEHOLDER
    : !canReply
      ? 'Your role can send approved templates only.'
      : win.open
        ? REPLY_PLACEHOLDER
        : win.templates
          ? `Window closed. Use Send template to message ${first}.`
          : `Window closed. You can reply once ${first} writes again.`
  // Meta's pricing hints are WhatsApp's; other channels show none.
  const costHint = isNote
    ? 'Private notes are never sent to the customer.'
    : !metaPricing || !win.limited
      ? null
      : win.open
        ? `Free: replies inside the ${win.label} are service messages (no Meta charge).`
        : 'Templates are charged by Meta per delivered message.'
  const listId = `composer-suggestions-${c.id}`

  return (
    <div className="flex flex-col gap-2 border-t bg-card px-3 pt-2.5 pb-3 sm:px-4">
      {!isNote && win.limited && !win.open && (
        <ClosedBanner
          window={win}
          canTemplate={canTemplate}
          metaPricing={metaPricing}
          onOpenTemplate={onOpenTemplate}
        />
      )}

      <div className="flex flex-wrap items-center gap-2">
        <div role="tablist" aria-label="Message type" className="flex rounded-md bg-muted p-0.5">
          {(canReply || canTemplate) && (
            <ModeTab selected={!isNote} onSelect={() => setMode('reply')}>
              Reply
            </ModeTab>
          )}
          {canNote && (
            <ModeTab selected={isNote} onSelect={() => setMode('note')} note>
              Private note
            </ModeTab>
          )}
        </div>
        {!isNote && canType && quickReplies.length > 0 && (
          <>
            <span className="text-xs text-muted-foreground">Quick replies:</span>
            {quickReplies.slice(0, 4).map((r) => (
              <button
                key={r.id}
                type="button"
                onClick={() => replaceAll(r.body)}
                className="h-7 rounded-full border px-2.5 text-xs font-semibold hover:bg-muted"
              >
                {r.title}
              </button>
            ))}
          </>
        )}
        {!isNote && canReply && (
          <Button
            variant="ghost"
            size="xs"
            className="text-muted-foreground"
            onClick={() => setManaging(true)}
          >
            <Settings2Icon aria-hidden="true" />
            Manage quick replies
          </Button>
        )}
        {isNote && members.some((m) => m.id !== me.id) && (
          <>
            <span className="text-xs text-muted-foreground">Mention:</span>
            {members
              .filter((m) => m.id !== me.id)
              .slice(0, 3)
              .map((m) => (
                <button
                  key={m.id}
                  type="button"
                  onClick={() => {
                    const token = `${text && !/\s$/.test(text) ? ' ' : ''}@${m.name} `
                    setMentioned((l) => (l.some((x) => x.id === m.id) ? l : [...l, m]))
                    replaceAll(text + token)
                  }}
                  className="h-7 rounded-full border border-warning/40 px-2.5 text-xs font-semibold text-warning hover:bg-warning-soft"
                >
                  @{firstName(m.name)}
                </button>
              ))}
          </>
        )}
      </div>

      {!isNote && replyTo && <ReplyBar message={replyTo} first={first} onClear={onClearReply} />}

      {!isNote && (attachment || upload.isPending) && (
        <AttachmentBar
          attachment={attachment}
          uploading={upload.isPending}
          onRemove={() => setAttachment(null)}
        />
      )}

      <div className="relative flex items-end gap-1.5">
        {menu.kind === 'slash' && (
          <SuggestionMenu
            id={listId}
            label="Quick replies"
            heading="Quick replies · keep typing to filter"
            empty="No quick reply matches."
            items={menu.items.map((r) => ({
              id: r.id,
              primary: `/${r.shortcut}`,
              secondary: r.body,
            }))}
            active={suggest.active}
            onPick={(i) => menu.items[i] && replaceAll(menu.items[i].body)}
          />
        )}
        {menu.kind === 'mention' && menu.items.length > 0 && (
          <SuggestionMenu
            id={listId}
            label="Mention a teammate"
            heading="Mention a teammate"
            empty=""
            items={menu.items.map((m) => ({ id: m.id, primary: m.name }))}
            active={suggest.active}
            onPick={(i) => menu.items[i] && pickMention(menu.items[i])}
          />
        )}
        <EmojiPicker disabled={!canType} onPick={insertAtCaret} />
        {!isNote && (
          <AttachMenu
            options={attachments}
            disabled={!canType || upload.isPending}
            onFile={onFile}
          />
        )}
        <label
          className={cn(
            'flex min-w-0 flex-1 rounded-lg border px-3 py-2',
            isNote ? 'border-warning bg-warning-soft' : 'border-input bg-muted',
            !canType && 'opacity-60',
          )}
        >
          <span className="sr-only">{isNote ? 'Private note' : 'Message'}</span>
          <textarea
            ref={input}
            rows={2}
            value={text}
            disabled={!canType}
            placeholder={placeholder}
            onChange={(e) => {
              setText(e.target.value)
              setCaret(e.target.selectionStart)
            }}
            onSelect={(e) => setCaret(e.currentTarget.selectionStart)}
            onKeyDown={onKeyDown}
            role="combobox"
            aria-expanded={menuOpen}
            aria-controls={menuOpen ? listId : undefined}
            aria-activedescendant={menuOpen ? `${listId}-${suggest.active}` : undefined}
            aria-autocomplete="list"
            aria-invalid={tooLong || undefined}
            aria-describedby={showCount ? `${listId}-count` : undefined}
            className="max-h-40 min-h-11 w-full resize-none bg-transparent text-base outline-none placeholder:text-muted-foreground disabled:cursor-not-allowed md:text-sm"
          />
        </label>
        {!isNote && canReply && (
          <Button
            variant="ghost"
            size="icon-sm"
            disabled={!canType}
            aria-label="Insert quick reply"
            onClick={() => replaceAll('/')}
          >
            <ZapIcon aria-hidden="true" />
          </Button>
        )}
        {!isNote && canTemplate && (
          <Button variant="outline" size="sm" className="h-10" onClick={onOpenTemplate}>
            <FileTextIcon aria-hidden="true" />
            Template
          </Button>
        )}
        <Button
          size="icon-sm"
          className={cn('size-10', isNote && 'bg-warning text-card hover:bg-warning/90')}
          disabled={!canType || !hasContent || tooLong || busy || upload.isPending}
          aria-label={isNote ? 'Save private note' : 'Send message'}
          onClick={submit}
        >
          {busy ? (
            <Loader2Icon className="animate-spin" aria-hidden="true" />
          ) : (
            <SendIcon aria-hidden="true" />
          )}
        </Button>
      </div>
      {(costHint || showCount) && (
        <span className="flex items-center gap-3 text-xs text-muted-foreground">
          {costHint && <span className="min-w-0 flex-1">{costHint}</span>}
          {showCount && (
            <span
              id={`${listId}-count`}
              className={cn('ml-auto tabular-nums', tooLong && 'font-bold text-destructive')}
            >
              {length.toLocaleString()} / {maxLength.toLocaleString()}
              <span className="sr-only">
                {tooLong ? ' characters, over the limit' : ' characters used'}
              </span>
            </span>
          )}
        </span>
      )}

      {canReply && (
        <QuickRepliesDialog workspaceId={workspaceId} open={managing} onOpenChange={setManaging} />
      )}
    </div>
  )
}

function ModeTab({
  selected,
  onSelect,
  note,
  children,
}: {
  selected: boolean
  onSelect: () => void
  note?: boolean
  children: React.ReactNode
}) {
  return (
    <button
      type="button"
      role="tab"
      aria-selected={selected}
      onClick={onSelect}
      className={cn(
        'h-8 rounded-sm px-3 text-[13px] font-bold',
        selected ? 'bg-card shadow-card' : 'text-muted-foreground hover:text-foreground',
        selected && note && 'text-warning',
      )}
    >
      {children}
    </button>
  )
}
