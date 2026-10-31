import type { AutomationDto, SettingsDto, Variable } from '@relay/api-client'

export type Kind = AutomationDto['kind']
export type Automation = AutomationDto
export type AutomationSettings = SettingsDto
export type TemplateVariable = Variable
/** Server validation problems by field, like "rules[1].reply". */
export type FieldErrors = Record<string, string>
