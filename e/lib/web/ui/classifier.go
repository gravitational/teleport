package ui

import (
	"strings"

	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types"
	libslices "github.com/gravitational/teleport/lib/utils/slices"
)

// Classifier is a UI representation of a session summarization classifier.
type Classifier struct {
	Name        string             `json:"name"`
	Description string             `json:"description,omitempty"`
	Labels      map[string]string  `json:"labels,omitempty"`
	Kinds       []string           `json:"kinds"`
	Filter      string             `json:"filter,omitempty"`
	Criteria    string             `json:"criteria"`
	Actions     *ClassifierActions `json:"actions,omitempty"`
}

// ClassifierActions configures the effects of a match. The tri-state action
// modes are exposed as optional booleans (null = unspecified), and the risk
// level as a short string ("low".."critical").
type ClassifierActions struct {
	EmitAuditEvent *bool  `json:"emit_audit_event,omitempty"`
	FlagForReview  *bool  `json:"flag_for_review,omitempty"`
	RiskLevelFloor string `json:"risk_level_floor,omitempty"`
}

var riskLevelToStr = map[summarizerv1.RiskLevel]string{
	summarizerv1.RiskLevel_RISK_LEVEL_LOW:      "low",
	summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM:   "medium",
	summarizerv1.RiskLevel_RISK_LEVEL_HIGH:     "high",
	summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL: "critical",
}

var riskLevelFromStr = map[string]summarizerv1.RiskLevel{
	"low":      summarizerv1.RiskLevel_RISK_LEVEL_LOW,
	"medium":   summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM,
	"high":     summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
	"critical": summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
}

// MakeClassifier converts a protobuf Classifier to UI representation.
func MakeClassifier(c *summarizerv1.Classifier) Classifier {
	if c == nil {
		return Classifier{}
	}

	ui := Classifier{
		Name:        c.GetMetadata().GetName(),
		Description: c.GetMetadata().GetDescription(),
		Labels:      c.GetMetadata().GetLabels(),
		Kinds:       c.GetSpec().GetKinds(),
		Filter:      c.GetSpec().GetFilter(),
		Criteria:    c.GetSpec().GetCriteria(),
	}

	if actions := c.GetSpec().GetActions(); actions != nil {
		ui.Actions = &ClassifierActions{
			EmitAuditEvent: actionModeToBool(actions.GetEmitAuditEvent()),
			FlagForReview:  actionModeToBool(actions.GetFlagForReview()),
			RiskLevelFloor: riskLevelToStr[actions.GetRiskLevelFloor()],
		}
	}

	return ui
}

// MakeClassifiers converts a slice of protobuf Classifiers to UI representation.
func MakeClassifiers(classifiers []*summarizerv1.Classifier) []Classifier {
	return libslices.Map(classifiers, MakeClassifier)
}

// ToProto converts a UI Classifier to protobuf representation.
func (c *Classifier) ToProto() *summarizerv1.Classifier {
	spec := summarizerv1.ClassifierSpec_builder{
		Kinds:    c.Kinds,
		Filter:   c.Filter,
		Criteria: c.Criteria,
	}
	if c.Actions != nil {
		spec.Actions = summarizerv1.ClassifierActions_builder{
			EmitAuditEvent: boolToActionMode(c.Actions.EmitAuditEvent),
			FlagForReview:  boolToActionMode(c.Actions.FlagForReview),
			RiskLevelFloor: riskLevelFromStr[strings.ToLower(c.Actions.RiskLevelFloor)],
		}.Build()
	}

	return summarizerv1.Classifier_builder{
		Kind:    types.KindClassifier,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:        c.Name,
			Description: c.Description,
			Labels:      c.Labels,
		}.Build(),
		Spec: spec.Build(),
	}.Build()
}

// ListClassifiersResponse is the response for listing classifiers.
type ListClassifiersResponse struct {
	// Items is the list of classifiers in this page.
	Items []Classifier `json:"items"`
	// NextKey is the token to retrieve the next page of results.
	NextKey string `json:"nextKey"`
}

func actionModeToBool(m summarizerv1.ClassifierActionMode) *bool {
	switch m {
	case summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED:
		v := true
		return &v
	case summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED:
		v := false
		return &v
	default:
		return nil
	}
}

func boolToActionMode(b *bool) summarizerv1.ClassifierActionMode {
	switch {
	case b == nil:
		return summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_UNSPECIFIED
	case *b:
		return summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED
	default:
		return summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED
	}
}
