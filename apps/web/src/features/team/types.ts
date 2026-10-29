import type {
  AccessDto,
  ChangeDto,
  InvitationDto,
  InviteInputBody,
  LogDto,
  MemberDetailDto,
  MemberDto,
  OverrideDto,
  PermissionDto,
  PreviewDto,
  RoleBody,
  RoleDto,
  SeatsDto,
  TeamBody,
  TeamDto,
} from '@relay/api-client'

// The generated enums drop the null the API sends for custom roles and "use the role's scope".
type SystemKey = NonNullable<RoleDto['system_key']>

export type ChatScope = AccessDto['chat_scope']
export type RoleColor = RoleDto['color']
export type Effect = OverrideDto['effect']

export type Access = Omit<AccessDto, 'permissions' | 'team_ids' | 'number_ids'> & {
  permissions: string[]
  team_ids: string[]
  number_ids: string[]
}
export type Permission = PermissionDto
export type Seats = SeatsDto
export type Role = Omit<RoleDto, 'system_key' | 'permissions'> & {
  system_key: SystemKey | null
  permissions: string[]
}
export type RoleInput = RoleBody
export type RoleChange = Required<{ [K in keyof ChangeDto]: NonNullable<ChangeDto[K]> }>
export type Member = Omit<MemberDto, 'role_key' | 'team_ids'> & {
  role_key: SystemKey | null
  team_ids: string[]
}
export type Override = OverrideDto
export type MemberDetail = Member &
  Omit<
    MemberDetailDto,
    keyof MemberDto | 'chat_scope' | 'effective_permissions' | 'number_ids' | 'overrides'
  > & {
    chat_scope: ChatScope | null
    effective_permissions: string[]
    number_ids: string[]
    overrides: Override[]
  }
export type Team = Omit<TeamDto, 'member_ids' | 'number_id'> & {
  member_ids: string[]
  number_id: string | null
}
export type TeamInput = TeamBody
export type Invitation = Omit<InvitationDto, 'team_ids'> & { team_ids: string[] }
export type InviteInput = InviteInputBody
export type InvitePreview = Omit<
  PreviewDto,
  'can_do' | 'permissions' | 'team_names' | 'number_labels'
> & {
  can_do: string[]
  permissions: string[]
  team_names: string[]
  number_labels: string[]
}
export type LogEntry = LogDto
export type MemberFilter = {
  role_id?: string
  team_id?: string
  status?: Member['status']
  q?: string
}

export type LogFilter = { action?: string; member?: string; since?: string }
