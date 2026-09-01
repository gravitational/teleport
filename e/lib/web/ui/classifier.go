package ui

import (
	"fmt"
	"strings"

	"github.com/gravitational/trace"

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
	Disabled    bool               `json:"disabled,omitempty"`
	Rules       []ClassifierRule   `json:"rules,omitempty"`
}

// ClassifierActions configures the effects of a match. The tri-state action
// modes are exposed as optional booleans (null = unspecified), and the risk
// level as a short string ("low".."critical").
type ClassifierActions struct {
	EmitAuditEvent *bool  `json:"emit_audit_event,omitempty"`
	FlagForReview  *bool  `json:"flag_for_review,omitempty"`
	RiskLevelFloor string `json:"risk_level_floor,omitempty"`
}

// ClassifierRule escalates the response to a match for a subset of sessions.
// Its filter and criteria narrow the top-level ones rather than replacing
// them, and its actions combine with the top-level actions.
type ClassifierRule struct {
	Name     string             `json:"name"`
	Filter   string             `json:"filter,omitempty"`
	Criteria string             `json:"criteria,omitempty"`
	Actions  *ClassifierActions `json:"actions,omitempty"`
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

	return Classifier{
		Name:        c.GetMetadata().GetName(),
		Description: c.GetMetadata().GetDescription(),
		Labels:      c.GetMetadata().GetLabels(),
		Kinds:       c.GetSpec().GetKinds(),
		Filter:      c.GetSpec().GetFilter(),
		Criteria:    c.GetSpec().GetCriteria(),
		Actions:     makeClassifierActions(c.GetSpec().GetActions()),
		Disabled:    c.GetSpec().GetDisabled(),
		Rules:       libslices.Map(c.GetSpec().GetRules(), makeClassifierRule),
	}
}

// MakeClassifiers converts a slice of protobuf Classifiers to UI representation.
func MakeClassifiers(classifiers []*summarizerv1.Classifier) []Classifier {
	return libslices.Map(classifiers, MakeClassifier)
}

// ToProto converts a UI Classifier to protobuf representation. Enum strings
// it does not recognize are rejected rather than mapped to the unspecified
// value, which would silently store a classifier without the constraint the
// user asked for.
func (c *Classifier) ToProto() (*summarizerv1.Classifier, error) {
	actions, err := c.Actions.toProto("spec.actions")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	rules := make([]*summarizerv1.ClassifierRule, 0, len(c.Rules))
	for i, r := range c.Rules {
		rule, err := r.toProto(fmt.Sprintf("spec.rules[%d]", i))
		if err != nil {
			return nil, trace.Wrap(err)
		}
		rules = append(rules, rule)
	}

	return summarizerv1.Classifier_builder{
		Kind:    types.KindClassifier,
		Version: types.V1,
		Metadata: headerv1.Metadata_builder{
			Name:        c.Name,
			Description: c.Description,
			Labels:      c.Labels,
		}.Build(),
		Spec: summarizerv1.ClassifierSpec_builder{
			Kinds:    c.Kinds,
			Filter:   c.Filter,
			Criteria: c.Criteria,
			Actions:  actions,
			Disabled: c.Disabled,
			Rules:    rules,
		}.Build(),
	}.Build(), nil
}

// ListClassifiersResponse is the response for listing classifiers.
type ListClassifiersResponse struct {
	// Items is the list of classifiers in this page.
	Items []Classifier `json:"items"`
	// NextKey is the token to retrieve the next page of results.
	NextKey string `json:"nextKey"`
}

func makeClassifierActions(a *summarizerv1.ClassifierActions) *ClassifierActions {
	if a == nil {
		return nil
	}
	return &ClassifierActions{
		EmitAuditEvent: actionModeToBool(a.GetEmitAuditEvent()),
		FlagForReview:  actionModeToBool(a.GetFlagForReview()),
		RiskLevelFloor: riskLevelToStr[a.GetRiskLevelFloor()],
	}
}

// toProto converts the actions, using path to name them in errors.
func (a *ClassifierActions) toProto(path string) (*summarizerv1.ClassifierActions, error) {
	if a == nil {
		return nil, nil
	}
	riskLevel := summarizerv1.RiskLevel_RISK_LEVEL_UNSPECIFIED
	if a.RiskLevelFloor != "" {
		var ok bool
		if riskLevel, ok = riskLevelFromStr[strings.ToLower(a.RiskLevelFloor)]; !ok {
			return nil, trace.BadParameter(
				`%s.risk_level_floor must be one of "low", "medium", "high", "critical"`, path)
		}
	}
	return summarizerv1.ClassifierActions_builder{
		EmitAuditEvent: boolToActionMode(a.EmitAuditEvent),
		FlagForReview:  boolToActionMode(a.FlagForReview),
		RiskLevelFloor: riskLevel,
	}.Build(), nil
}

func makeClassifierRule(r *summarizerv1.ClassifierRule) ClassifierRule {
	return ClassifierRule{
		Name:     r.GetName(),
		Filter:   r.GetFilter(),
		Criteria: r.GetCriteria(),
		Actions:  makeClassifierActions(r.GetActions()),
	}
}

// toProto converts the rule, using path to name it in errors.
func (r ClassifierRule) toProto(path string) (*summarizerv1.ClassifierRule, error) {
	actions, err := r.Actions.toProto(path + ".actions")
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return summarizerv1.ClassifierRule_builder{
		Name:     r.Name,
		Filter:   r.Filter,
		Criteria: r.Criteria,
		Actions:  actions,
	}.Build(), nil
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
