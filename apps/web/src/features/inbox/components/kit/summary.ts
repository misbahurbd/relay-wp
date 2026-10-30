import type { Message } from '../../types'

/** One line for quotes, copies and announcements: the text, or what kind of message it was. */
export function messageSummaryText(m: Pick<Message, 'type' | 'body'>) {
  if (m.body) return m.body
  const label: Record<string, string> = {
    image: 'Photo',
    video: 'Video',
    audio: 'Voice message',
    document: 'Document',
    sticker: 'Sticker',
    location: 'Location',
    contacts: 'Contact card',
  }
  return label[m.type] ?? 'Message'
}
