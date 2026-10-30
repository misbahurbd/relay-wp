import { Badge } from '#/components/ui/badge'

export function ActiveBadge({ active }: { active: boolean }) {
  return <Badge variant={active ? 'success' : 'neutral'}>{active ? 'Active' : 'Inactive'}</Badge>
}
