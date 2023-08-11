package servicenow

import (
	"time"

	"github.com/gravitational/teleport/api/types"
)

// Incident represents a servicenow incident.
type Incident struct {
	ShortDescription string `json:"short_description"`
	Description      string `json:"description"`
	CloseCode        string `json:"close_code"`
	CloseNotes       string `json:"close_notes"`
	IncidentState    string `json:"incident_state"`
	WorkNotes        string `json:"work_notes"`
}

// Resolution stores the resolution state and the servicenow close code.
type Resolution struct {
	State     string
	CloseCode string
	Reason    string
}

// RequestData stores a slice of some request fields in a convenient format.
type RequestData struct {
	User              string
	Roles             []string
	Created           time.Time
	RequestReason     string
	ReviewsCount      int
	Resolution        Resolution
	SystemAnnotations types.Labels
}

type onCallResult struct {
	Result []struct {
		UserID string `json:"userId"`
	} `json:"result"`
}

type userResult struct {
	Result []struct {
		Email string `json:"email"`
	} `json:"result"`
}
