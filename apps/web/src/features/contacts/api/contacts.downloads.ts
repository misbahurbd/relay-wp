import { downloadContactImportSkipped, exportContacts } from '@relay/api-client'
import { inWorkspace } from '#/lib/api'
import type { Contact, ContactFilter } from '../types'

// Downloads go through the generated client, so the session cookie and the refresh-on-401
// in #/lib/api apply. The API's Content-Disposition isn't exposed over CORS, so the file
// name is set here.

function saveBlob(blob: Blob, fileName: string) {
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = fileName
  document.body.append(a)
  a.click()
  a.remove()
  setTimeout(() => URL.revokeObjectURL(url), 1_000)
}

const today = () => new Date().toISOString().slice(0, 10)

/** Downloads every contact matching the filter as CSV (AD-4 export). */
export async function downloadContactsExport(workspaceId: string, filter: ContactFilter) {
  const { data } = await exportContacts({
    headers: inWorkspace(workspaceId),
    query: filter,
    parseAs: 'blob',
    throwOnError: true,
  })
  saveBlob(data as unknown as Blob, `contacts-${today()}.csv`)
}

/** Downloads the rows an import skipped, with the reason for each. */
export async function downloadSkippedRows(workspaceId: string, importId: string) {
  const { data } = await downloadContactImportSkipped({
    headers: inWorkspace(workspaceId),
    path: { importId },
    parseAs: 'blob',
    throwOnError: true,
  })
  saveBlob(data as unknown as Blob, `skipped-rows-${today()}.csv`)
}

const CSV_COLUMNS = ['name', 'phone', 'email', 'opt_in', 'tags', 'source', 'language', 'created_at']

function csvCell(raw: string) {
  // A leading = or @ would run as a spreadsheet formula. (+ stays: phones start with it.)
  const value = /^[=@]/.test(raw) ? `'${raw}` : raw
  return /[",\n\r]/.test(value) ? `"${value.replaceAll('"', '""')}"` : value
}

/**
 * Saves the selected rows as CSV with the export's leading columns. The export endpoint
 * filters but can't take a list of ids, and the selected rows are already loaded.
 */
export function downloadSelectedContacts(rows: Contact[]) {
  const lines = [
    CSV_COLUMNS.join(','),
    ...rows.map((r) =>
      [
        r.name,
        r.phone,
        r.email ?? '',
        r.opt_in,
        r.tags.map((t) => t.name).join('; '),
        r.source,
        r.language ?? '',
        r.created_at,
      ]
        .map(csvCell)
        .join(','),
    ),
  ]
  saveBlob(new Blob([`${lines.join('\r\n')}\r\n`], { type: 'text/csv' }), `contacts-${today()}.csv`)
}

/** A small file showing the columns the importer understands. */
export function downloadSampleFile() {
  const csv =
    'name,phone,email,city,tags\r\nHannah Becker,+49 151 2345 6789,hannah@example.com,Berlin,VIP\r\n'
  saveBlob(new Blob([csv], { type: 'text/csv' }), 'contacts-sample.csv')
}
