import type {
  BusinessInputBody,
  ChecklistItemDto,
  NumberDto,
  OnboardingDto,
  SetupTasksDto,
  SignupConfigOutputBody,
  StepDto,
  TestMessageOutputBody,
} from '@relay/api-client'

export type Industry = BusinessInputBody['industry']
export type TeamSize = BusinessInputBody['team_size']
export type BusinessGoal = NonNullable<BusinessInputBody['goals']>[number]
export type BusinessInput = Omit<BusinessInputBody, 'goals'> & { goals: BusinessGoal[] }

export type ChecklistItem = ChecklistItemDto
export type Onboarding = Omit<OnboardingDto, 'steps' | 'checklist'> & {
  steps: StepDto[]
  checklist: ChecklistItemDto[]
}

export type NumberType = NumberDto['number_type']
export type SetupTaskState = SetupTasksDto['register']
export type WhatsAppNumber = Omit<NumberDto, 'quality_rating'> & {
  quality_rating: NumberDto['quality_rating'] | null
}
export type TestMessage = TestMessageOutputBody
export type SignupConfig = SignupConfigOutputBody
