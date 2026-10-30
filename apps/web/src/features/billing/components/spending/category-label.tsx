import type { MetaCostCategory } from '../../types'

/** Name and colour of each Meta message category, as the Spending design draws them. */
export const CATEGORY: Record<MetaCostCategory, { label: string; bg: string }> = {
  marketing: { label: 'Marketing', bg: 'bg-cat-marketing' },
  utility: { label: 'Utility', bg: 'bg-cat-utility' },
  authentication: { label: 'Authentication', bg: 'bg-cat-authentication' },
  service: { label: 'Service (customer replies)', bg: 'bg-border' },
}
