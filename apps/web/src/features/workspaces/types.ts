import type { CreateInputBody, SummaryDto, WorkspaceDto } from '@relay/api-client'

export type FirstGoal = CreateInputBody['first_goal']
export type StartMode = CreateInputBody['start_mode']
export type WorkspaceSummary = SummaryDto

// Business fields are null until setup step 1 is saved; the generated enums drop the null.
export type Workspace = Omit<WorkspaceDto, 'industry' | 'team_size' | 'goals'> & {
  industry: WorkspaceDto['industry'] | null
  team_size: WorkspaceDto['team_size'] | null
  goals: NonNullable<WorkspaceDto['goals']>
}
