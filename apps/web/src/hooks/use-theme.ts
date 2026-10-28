import { useSyncExternalStore } from 'react'
import { getTheme, resolveTheme, setTheme, subscribeTheme, type Theme } from '#/lib/theme'

// The server can't know the theme, so it renders 'system'; the client corrects it after hydration.
export function useTheme() {
  const theme = useSyncExternalStore<Theme>(subscribeTheme, getTheme, () => 'system')
  const resolvedTheme = useSyncExternalStore(
    subscribeTheme,
    () => resolveTheme(getTheme()),
    () => 'light' as const,
  )
  return { theme, resolvedTheme, setTheme }
}
