import type {
  ButtonDto,
  CountsDto,
  HeaderDto,
  SampleDto,
  TemplateDto,
  WarningDto,
} from '@relay/api-client'

export type TemplateStatus = TemplateDto['status']
export type TemplateCategory = TemplateDto['category']
export type TemplateLanguage = TemplateDto['language']
export type HeaderType = HeaderDto['type']
export type ButtonType = ButtonDto['type']
export type HeaderSample = SampleDto
export type TemplateWarning = WarningDto
export type TemplateCounts = CountsDto

/** The template with the API's nullable lists turned into empty ones. */
export type Template = Omit<TemplateDto, 'components' | 'warnings'> & {
  components: {
    header: HeaderDto
    body: { text: string; examples: string[] }
    footer: { text: string }
    buttons: ButtonDto[]
  }
  warnings: TemplateWarning[]
}

/** The status tabs on the list; drafts, paused and disabled templates show under "All". */
export type StatusFilter = Extract<TemplateStatus, 'approved' | 'pending' | 'rejected'>

export type TemplateFilter = {
  status?: StatusFilter
  category?: TemplateCategory
  q?: string
}

/** What the create/edit form holds; the same shape as the API's components, plus the basics. */
export type TemplateFormValues = {
  name: string
  language: string
  category: TemplateCategory
  header: { type: HeaderType; text: string; example: string }
  body: { text: string; examples: string[] }
  footer: { text: string }
  buttons: { type: ButtonType; text: string; value: string }[]
}
