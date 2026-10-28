package workspaces

import (
	"time"

	"github.com/google/uuid"
)

type summaryDTO struct {
	ID           uuid.UUID `json:"id"`
	Name         string    `json:"name"`
	Role         string    `json:"role" enum:"owner"`
	LastActiveAt time.Time `json:"last_active_at"`
	CurrentStep  string    `json:"current_step" enum:"business,whatsapp,test_message,done"`
	Completed    bool      `json:"completed" doc:"Setup finished or skipped: open the dashboard"`
	Activated    bool      `json:"activated" doc:"A real customer message has been sent"`
}

func toSummaryDTO(s Summary) summaryDTO { return summaryDTO(s) }

type workspaceDTO struct {
	ID          uuid.UUID  `json:"id"`
	Name        string     `json:"name"`
	Industry    *string    `json:"industry" nullable:"true" enum:"retail_ecommerce,food_restaurants,education,health_clinics,travel,real_estate,other"`
	Website     *string    `json:"website"`
	Country     *string    `json:"country" doc:"ISO 3166-1 alpha-2"`
	TeamSize    *string    `json:"team_size" nullable:"true" enum:"just_me,2_5,6_20,20_plus"`
	Goals       []string   `json:"goals" enum:"marketing,support,sales"`
	Plan        string     `json:"plan" enum:"growth"`
	TrialEndsAt *time.Time `json:"trial_ends_at"`
	ActivatedAt *time.Time `json:"activated_at"`
}

func toWorkspaceDTO(w Workspace) workspaceDTO { return workspaceDTO(w) }

type listOutput struct {
	Body struct {
		Workspaces []summaryDTO `json:"workspaces"`
	}
}

type createInput struct {
	Body struct {
		Name      string `json:"name" minLength:"1" maxLength:"100" doc:"Default name, e.g. \"Ada's workspace\"; setup step 1 replaces it"`
		FirstGoal string `json:"first_goal" enum:"offers,support,automate"`
		StartMode string `json:"start_mode" enum:"setup,sample"`
	}
}

type summaryOutput struct {
	Body summaryDTO
}

type workspaceInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
}

type workspaceOutput struct {
	Body workspaceDTO
}

type businessInput struct {
	WorkspaceID uuid.UUID `path:"workspaceId"`
	Body        struct {
		Name     string   `json:"name" minLength:"1" maxLength:"100"`
		Industry string   `json:"industry" enum:"retail_ecommerce,food_restaurants,education,health_clinics,travel,real_estate,other"`
		Website  string   `json:"website,omitempty" maxLength:"200"`
		Country  string   `json:"country" pattern:"^[A-Za-z]{2}$" doc:"ISO 3166-1 alpha-2"`
		TeamSize string   `json:"team_size" enum:"just_me,2_5,6_20,20_plus"`
		Goals    []string `json:"goals" maxItems:"3" enum:"marketing,support,sales"`
	}
}
