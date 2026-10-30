import { errorMessage, isApiError } from '#/lib/api'

/** Copy for a channel that's gone, as an error toast and on a failed message. */
export const CHANNEL_UNAVAILABLE = 'This channel isn’t connected anymore'

const CHANNEL_ERRORS: Record<string, string> = {
  templates_unsupported: 'This channel doesn’t support templates.',
  reactions_unsupported: 'This channel doesn’t support reactions.',
  channel_unavailable: `${CHANNEL_UNAVAILABLE}, so messages can’t be sent.`,
}

/** An inbox API error as toast text, with the channel errors (ADR 0010) in our own words. */
export function inboxErrorMessage(error: unknown, fallback?: string) {
  if (isApiError(error) && error.code && CHANNEL_ERRORS[error.code]) {
    return CHANNEL_ERRORS[error.code]
  }
  return errorMessage(error, fallback)
}
