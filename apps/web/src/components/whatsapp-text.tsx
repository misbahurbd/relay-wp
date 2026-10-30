import { cn } from '#/lib/utils'

export type Segment = { text: string; bold?: boolean; italic?: boolean; strike?: boolean }

const MARKS = { '*': 'bold', _: 'italic', '~': 'strike' } as const
type Mark = keyof typeof MARKS

/**
 * Splits WhatsApp formatting (*bold*, _italic_, ~strike~) into styled segments. A marker only
 * counts when it wraps non-space text on one line, as in WhatsApp.
 */
export function formatSegments(text: string): Segment[] {
  const out: Segment[] = []
  const pattern = /([*_~])(?=\S)([^\n]*?\S)\1/g
  let last = 0
  for (const m of text.matchAll(pattern)) {
    const at = m.index ?? 0
    if (at > last) out.push({ text: text.slice(last, at) })
    const key = MARKS[m[1] as Mark]
    // One level of nesting covers what the toolbar produces (*_bold italic_*).
    for (const inner of formatSegments(m[2])) out.push({ ...inner, [key]: true })
    last = at + m[0].length
  }
  if (last < text.length) out.push({ text: text.slice(last) })
  return out
}

/** Text as WhatsApp shows it: *bold*, _italic_ and ~strike~ styled, line breaks kept by the parent. */
export function WhatsAppText({ text }: { text: string }) {
  return (
    <>
      {formatSegments(text).map((s, i) => (
        <span
          // biome-ignore lint/suspicious/noArrayIndexKey: segments have no identity of their own
          key={i}
          className={cn(s.bold && 'font-bold', s.italic && 'italic', s.strike && 'line-through')}
        >
          {s.text}
        </span>
      ))}
    </>
  )
}
