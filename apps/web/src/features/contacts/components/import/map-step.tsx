import { useId } from 'react'
import { Checkbox } from '#/components/ui/checkbox'
import { Input } from '#/components/ui/input'
import { Label } from '#/components/ui/label'
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select'
import { IMPORT_FIELDS, type MappingValues } from '../../schemas/contacts.schema'
import type { ImportField, ImportUpload } from '../../types'

export type MappingErrors = { columns?: string; tag_name?: string; opt_in_confirmed?: string }

/** The server's guesses become the starting mapping. */
export function initialMapping(upload: ImportUpload): MappingValues {
  return {
    columns: upload.columns.map((c) => ({
      name: c.name,
      field: c.guess,
      attribute_key: c.guess === 'attribute' ? c.name : undefined,
    })),
    tag_name: '',
    opt_in_confirmed: false,
  }
}

/** Step 2: what each column becomes, an optional tag for everyone, and the opt-in promise. */
export function MapStep({
  upload,
  value,
  errors,
  onChange,
}: {
  upload: ImportUpload
  value: MappingValues
  errors: MappingErrors
  onChange: (next: MappingValues) => void
}) {
  const id = useId()
  const setColumn = (i: number, patch: Partial<MappingValues['columns'][number]>) =>
    onChange({
      ...value,
      columns: value.columns.map((c, j) => (j === i ? { ...c, ...patch } : c)),
    })

  return (
    <>
      <p className="text-muted-foreground">
        Match each column in your file to a contact field. We guessed most of them for you.
      </p>
      <div
        className="flex flex-col overflow-hidden rounded-lg border"
        aria-invalid={!!errors.columns}
        aria-describedby={errors.columns ? `${id}-columns-error` : undefined}
      >
        <div className="hidden grid-cols-3 gap-3 bg-muted px-3.5 py-2.5 text-xs font-bold text-muted-foreground sm:grid">
          <span>Column in your file</span>
          <span>Sample</span>
          <span>Contact field</span>
        </div>
        {value.columns.map((col, i) => {
          const samples = upload.columns.find((c) => c.name === col.name)?.samples ?? []
          return (
            <div
              key={col.name}
              className="grid grid-cols-1 items-center gap-x-3 gap-y-1.5 border-t px-3.5 py-2 text-[13px] first:border-t-0 sm:grid-cols-3 sm:first:border-t"
            >
              <span className="font-bold break-words">{col.name}</span>
              <span className="truncate text-muted-foreground" title={samples.join(', ')}>
                {samples.filter(Boolean).join(', ') || '—'}
              </span>
              <div className="flex flex-col gap-1.5">
                <Select
                  value={col.field}
                  onValueChange={(v) =>
                    setColumn(i, {
                      field: v as ImportField,
                      attribute_key:
                        v === 'attribute' ? (col.attribute_key ?? col.name) : undefined,
                    })
                  }
                >
                  <SelectTrigger
                    size="sm"
                    className="h-10 w-full rounded-[10px]"
                    aria-label={`Field for ${col.name}`}
                  >
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {IMPORT_FIELDS.map((f) => (
                      <SelectItem key={f.value} value={f.value}>
                        {f.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                {col.field === 'attribute' && (
                  <Input
                    value={col.attribute_key ?? ''}
                    onChange={(e) => setColumn(i, { attribute_key: e.target.value })}
                    aria-label={`Custom field name for ${col.name}`}
                    placeholder="Custom field name"
                    maxLength={64}
                    className="h-10 rounded-[10px]"
                  />
                )}
              </div>
            </div>
          )
        })}
      </div>
      {errors.columns && (
        <p id={`${id}-columns-error`} role="alert" className="text-[13px] text-destructive">
          {errors.columns}
        </p>
      )}

      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${id}-tag`}>Add a tag to everyone imported (optional)</Label>
        <Input
          id={`${id}-tag`}
          value={value.tag_name}
          onChange={(e) => onChange({ ...value, tag_name: e.target.value })}
          placeholder="e.g. Winter leads"
          maxLength={50}
          className="sm:max-w-xs"
          aria-invalid={!!errors.tag_name}
          aria-describedby={errors.tag_name ? `${id}-tag-error` : undefined}
        />
        {errors.tag_name && (
          <p id={`${id}-tag-error`} role="alert" className="text-[13px] text-destructive">
            {errors.tag_name}
          </p>
        )}
      </div>

      <div className="flex flex-col gap-1.5">
        <Label htmlFor={`${id}-optin`} className="items-start gap-2.5 font-normal">
          <Checkbox
            id={`${id}-optin`}
            className="mt-0.5"
            checked={value.opt_in_confirmed === true}
            onCheckedChange={(v) => onChange({ ...value, opt_in_confirmed: v === true })}
            aria-invalid={!!errors.opt_in_confirmed}
            aria-describedby={errors.opt_in_confirmed ? `${id}-optin-error` : undefined}
          />
          <span>
            These contacts agreed to receive WhatsApp messages from us (required by Meta). Anyone
            who opted out stays opted out.
          </span>
        </Label>
        {errors.opt_in_confirmed && (
          <p id={`${id}-optin-error`} role="alert" className="text-[13px] text-destructive">
            {errors.opt_in_confirmed}
          </p>
        )}
      </div>
    </>
  )
}
