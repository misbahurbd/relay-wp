import { Link } from '@tanstack/react-router'
import { cn } from '#/lib/utils'

const tab =
  'inline-flex h-10 items-center gap-2 rounded-lg px-3.5 text-sm font-bold no-underline text-muted-foreground hover:bg-muted hover:text-foreground'

/** "All contacts | Segments" under the Contacts title. */
export function ContactsTabs({
  current,
  segments,
}: {
  current: 'all' | 'segments'
  segments?: number
}) {
  return (
    <nav aria-label="Contacts sections" className="flex gap-1 border-b pb-2">
      <Link
        to="/contacts"
        aria-current={current === 'all' ? 'page' : undefined}
        className={cn(tab, current === 'all' && 'bg-brand-soft text-brand-text')}
      >
        All contacts
      </Link>
      <Link
        to="/contacts/segments"
        aria-current={current === 'segments' ? 'page' : undefined}
        className={cn(tab, current === 'segments' && 'bg-brand-soft text-brand-text')}
      >
        Segments
        {segments !== undefined && (
          <span className="rounded-full bg-muted px-2 text-xs text-muted-foreground">
            {segments}
          </span>
        )}
      </Link>
    </nav>
  )
}
