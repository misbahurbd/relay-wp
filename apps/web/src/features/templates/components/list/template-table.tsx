import { Link } from '@tanstack/react-router'
import { EyeIcon, FileTextIcon, MoreVerticalIcon, PencilIcon, Trash2Icon } from 'lucide-react'
import type { ReactNode } from 'react'
import { Button } from '#/components/ui/button'
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from '#/components/ui/dropdown-menu'
import { Skeleton } from '#/components/ui/skeleton'
import { isFrozen, languageLabel } from '../../schemas/templates.schema'
import type { Template } from '../../types'
import { CategoryBadge } from '../kit/category-badge'
import { formatUpdated } from '../kit/format'
import { StatusBadge } from '../kit/status-badge'

const COLUMNS = 6

export function TemplateTable({
  items,
  loading,
  canEdit,
  canDelete,
  empty,
  onDelete,
}: {
  items: Template[]
  loading: boolean
  canEdit: boolean
  canDelete: boolean
  empty: ReactNode
  onDelete: (t: Template) => void
}) {
  return (
    <div className="-mx-5 overflow-x-auto px-5 sm:-mx-6 sm:px-6">
      <table className="w-full min-w-[880px] border-collapse text-sm">
        <thead>
          <tr className="text-left text-xs text-muted-foreground">
            <th scope="col" className="py-2 pr-2 font-semibold">
              Template
            </th>
            <th scope="col" className="p-2 font-semibold">
              Category
            </th>
            <th scope="col" className="p-2 font-semibold">
              Language
            </th>
            <th scope="col" className="p-2 font-semibold">
              Status
            </th>
            <th scope="col" className="p-2 font-semibold">
              Last updated
            </th>
            <th scope="col" className="py-2 pl-2 font-semibold">
              <span className="sr-only">Actions</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {loading &&
            [0, 1, 2, 3, 4].map((i) => (
              <tr key={i} className="border-t">
                <td colSpan={COLUMNS} className="py-3">
                  <Skeleton className="h-11 w-full" />
                </td>
              </tr>
            ))}
          {items.map((t) => (
            <Row
              key={t.id}
              template={t}
              canEdit={canEdit && !isFrozen(t)}
              canDelete={canDelete}
              onDelete={() => onDelete(t)}
            />
          ))}
          {!loading && items.length === 0 && (
            <tr className="border-t">
              <td colSpan={COLUMNS} className="py-12 text-center">
                {empty}
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  )
}

function Row({
  template: t,
  canEdit,
  canDelete,
  onDelete,
}: {
  template: Template
  canEdit: boolean
  canDelete: boolean
  onDelete: () => void
}) {
  const link = {
    to: '/templates/$templateId',
    params: { templateId: t.id },
  } as const
  return (
    <tr className="border-t align-top hover:bg-muted/60">
      <td className="max-w-[360px] py-3 pr-2">
        <div className="flex items-start gap-3">
          <span className="flex size-9 flex-none items-center justify-center rounded-lg bg-brand-soft text-brand-text">
            <FileTextIcon className="size-[18px]" aria-hidden="true" />
          </span>
          <div className="flex min-w-0 flex-col">
            <Link
              {...link}
              className="truncate font-bold text-foreground no-underline hover:underline"
            >
              {t.name}
            </Link>
            <span className="line-clamp-1 text-[13px] text-muted-foreground">
              {t.components.body.text || 'No message text yet'}
            </span>
          </div>
        </div>
      </td>
      <td className="p-2 py-3">
        <CategoryBadge category={t.category} />
      </td>
      <td className="p-2 py-3 whitespace-nowrap">{languageLabel(t.language)}</td>
      <td className="max-w-[260px] p-2 py-3">
        <div className="flex flex-col items-start gap-1">
          <StatusBadge status={t.status} />
          {t.status === 'rejected' && t.rejected_reason && (
            <span className="text-xs text-destructive">{t.rejected_reason}</span>
          )}
          {t.status === 'pending' && (
            <span className="text-xs text-muted-foreground">
              Meta usually replies within 24 hours
            </span>
          )}
        </div>
      </td>
      <td className="p-2 py-3 whitespace-nowrap text-muted-foreground">
        {formatUpdated(t.updated_at)}
      </td>
      <td className="py-3 pl-2">
        <div className="flex items-center justify-end gap-1.5">
          {t.status === 'rejected' && canEdit && (
            <Link
              {...link}
              className="inline-flex h-9 items-center rounded-md px-3 text-[13px] font-bold whitespace-nowrap text-brand-text no-underline hover:bg-muted"
            >
              Edit and resubmit
            </Link>
          )}
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <Button variant="ghost" size="icon-sm" aria-label={`More actions for ${t.name}`}>
                <MoreVerticalIcon aria-hidden="true" />
              </Button>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              <DropdownMenuItem asChild>
                <Link {...link}>
                  {canEdit ? <PencilIcon aria-hidden="true" /> : <EyeIcon aria-hidden="true" />}
                  {canEdit ? 'Edit' : 'View'}
                </Link>
              </DropdownMenuItem>
              {canDelete && (
                <DropdownMenuItem variant="destructive" onSelect={onDelete}>
                  <Trash2Icon aria-hidden="true" />
                  Delete
                </DropdownMenuItem>
              )}
            </DropdownMenuContent>
          </DropdownMenu>
        </div>
      </td>
    </tr>
  )
}
