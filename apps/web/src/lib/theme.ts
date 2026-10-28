export type Theme = 'light' | 'dark' | 'system'

const STORAGE_KEY = 'relay-theme'
const CHANGE_EVENT = 'relay-theme-change'
const DARK_QUERY = '(prefers-color-scheme: dark)'

// Runs in <head> before the page paints, so a dark-mode user never sees a light flash.
export const themeScript = `(function(){try{var t=localStorage.getItem('${STORAGE_KEY}');var d=t==='dark'||(t!=='light'&&matchMedia('${DARK_QUERY}').matches);document.documentElement.classList.toggle('dark',d)}catch(e){}})()`

export function getTheme(): Theme {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'light' || stored === 'dark') return stored
  } catch {
    // Storage blocked (private mode): fall back to the system setting.
  }
  return 'system'
}

export function resolveTheme(theme: Theme): 'light' | 'dark' {
  if (theme !== 'system') return theme
  return matchMedia(DARK_QUERY).matches ? 'dark' : 'light'
}

export function setTheme(theme: Theme) {
  try {
    if (theme === 'system') localStorage.removeItem(STORAGE_KEY)
    else localStorage.setItem(STORAGE_KEY, theme)
  } catch {
    // Not persisted, but still applied for this page.
  }
  applyTheme(theme)
  window.dispatchEvent(new Event(CHANGE_EVENT))
}

function applyTheme(theme: Theme) {
  document.documentElement.classList.toggle('dark', resolveTheme(theme) === 'dark')
}

// Calls onChange when the theme is set here or in another tab, or the OS setting flips.
export function subscribeTheme(onChange: () => void) {
  const media = matchMedia(DARK_QUERY)
  const onSystem = () => {
    if (getTheme() === 'system') applyTheme('system')
    onChange()
  }
  const onStorage = (e: StorageEvent) => {
    if (e.key !== STORAGE_KEY) return
    applyTheme(getTheme())
    onChange()
  }
  window.addEventListener(CHANGE_EVENT, onChange)
  window.addEventListener('storage', onStorage)
  media.addEventListener('change', onSystem)
  return () => {
    window.removeEventListener(CHANGE_EVENT, onChange)
    window.removeEventListener('storage', onStorage)
    media.removeEventListener('change', onSystem)
  }
}
