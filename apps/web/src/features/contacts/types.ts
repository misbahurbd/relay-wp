import type {
  BulkInputBody,
  ColumnMappingDto,
  ContactDetailDto,
  ContactDto,
  CreateContactInputBody,
  ImportColumnDto,
  ImportDto,
  PersonDto,
  PreviewOutputBody,
  PreviewRowDto,
  SegmentConditionDto,
  SegmentCountDto,
  SegmentDto,
  SegmentMatchDto,
  TagDto,
  TagRefDto,
  UpdateContactInputBody,
  UploadOutputBody,
} from '@relay/api-client'

export type OptIn = ContactDto['opt_in']
export type ContactSource = ContactDto['source']
export type LastMessage = '24h' | '7d' | '30d' | 'over_90d'
export type ImportField = ColumnMappingDto['field']

export type TagRef = TagRefDto
export type Tag = TagDto
export type Person = PersonDto

// The generated types drop the nulls the API sends: no tags, nobody assigned.
export type Contact = Omit<ContactDto, 'tags' | 'assigned_to'> & {
  tags: TagRef[]
  assigned_to: Person | null
}
export type ContactDetail = Omit<ContactDetailDto, 'tags' | 'assigned_to' | 'attributes'> & {
  tags: TagRef[]
  assigned_to: Person | null
  attributes: Record<string, string>
}
export type ContactInput = CreateContactInputBody
export type ContactPatch = UpdateContactInputBody
export type BulkInput = Omit<BulkInputBody, 'contact_ids'> & { contact_ids: string[] }

/** The list filters, kept in the URL. */
export type ContactFilter = {
  q?: string
  opt_in?: OptIn
  tag_id?: string
  last_message?: LastMessage
}

export type ContactPage = { items: Contact[]; next: string | null; total: number }
export type ContactsSummary = { total: number; opted_in: number }

export type ImportColumn = Omit<ImportColumnDto, 'samples'> & { samples: string[] }
export type ImportUpload = Omit<UploadOutputBody, 'columns'> & { columns: ImportColumn[] }
export type ColumnMapping = ColumnMappingDto
export type ImportPreview = Omit<PreviewOutputBody, 'sample'> & { sample: PreviewRowDto[] }
export type PreviewRow = PreviewRowDto
export type ContactImport = ImportDto

// ---------- Segments (ADR 0014) ----------

export type SegmentField = SegmentConditionDto['field']
export type SegmentOp = SegmentConditionDto['op']
export type SegmentCondition = { field: SegmentField; op: SegmentOp; value: string; key?: string }
export type SegmentGroup = { match: 'all' | 'any'; conditions: SegmentCondition[] }
export type SegmentRules = { groups: SegmentGroup[] }
export type SegmentCount = SegmentCountDto
export type Segment = Omit<SegmentDto, 'rules'> & { rules: SegmentRules }
export type SegmentPreview = { count: SegmentCount; sample: SegmentMatchDto[] }
