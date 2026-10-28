import { MoonIcon, SunIcon } from 'lucide-react'
import { Button } from '#/components/ui/button'
import { useTheme } from '#/hooks/use-theme'

export function ThemeToggle() {
  const { resolvedTheme, setTheme } = useTheme()
  const dark = resolvedTheme === 'dark'

  return (
    <Button
      variant="outline"
      size="icon"
      aria-label={dark ? 'Switch to light mode' : 'Switch to dark mode'}
      onClick={() => setTheme(dark ? 'light' : 'dark')}
    >
      {dark ? <SunIcon /> : <MoonIcon />}
    </Button>
  )
}
