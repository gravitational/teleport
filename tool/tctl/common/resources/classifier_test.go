// Teleport
// Copyright (C) 2026 Gravitational, Inc.
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package resources

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types/summarizer"
	"github.com/gravitational/teleport/lib/services"
)

func makeClassifierWithActions(actions *summarizerv1.ClassifierActions) *summarizerv1.Classifier {
	return summarizer.NewClassifier("test", summarizerv1.ClassifierSpec_builder{
		Kinds:    []string{"ssh"},
		Criteria: "test criteria",
		Actions:  actions,
	}.Build())
}

// "tctl get" renders toggles as booleans and risk_level_floor as a short string.
func TestClassifierActionsMarshalFriendly(t *testing.T) {
	c := makeClassifierWithActions(summarizerv1.ClassifierActions_builder{
		EmitAuditEvent: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
		FlagForReview:  summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED,
		RiskLevelFloor: summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
	}.Build())

	data, err := classifierResource{classifier: c}.MarshalJSON()
	require.NoError(t, err)

	actions := unmarshalActions(t, data)
	require.Equal(t, true, actions["emit_audit_event"])
	require.Equal(t, false, actions["flag_for_review"])
	require.Equal(t, "high", actions["risk_level_floor"])
}

func TestClassifierActionsUnspecifiedOmitted(t *testing.T) {
	c := makeClassifierWithActions(summarizerv1.ClassifierActions_builder{
		EmitAuditEvent: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
		// flag_for_review and risk_level_floor are left unspecified.
	}.Build())

	data, err := classifierResource{classifier: c}.MarshalJSON()
	require.NoError(t, err)

	actions := unmarshalActions(t, data)
	require.Equal(t, true, actions["emit_audit_event"])
	_, hasFlag := actions["flag_for_review"]
	require.False(t, hasFlag, "an unspecified toggle should be omitted, not printed")
	_, hasRisk := actions["risk_level_floor"]
	require.False(t, hasRisk, "an unspecified risk level should be omitted, not printed")
}

// Friendly values survive a get -> create round-trip across every risk level.
func TestClassifierActionsRoundTrip(t *testing.T) {
	for _, level := range []summarizerv1.RiskLevel{
		summarizerv1.RiskLevel_RISK_LEVEL_LOW,
		summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM,
		summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
		summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
	} {
		t.Run(level.String(), func(t *testing.T) {
			c := makeClassifierWithActions(summarizerv1.ClassifierActions_builder{
				EmitAuditEvent: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
				FlagForReview:  summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED,
				RiskLevelFloor: level,
			}.Build())

			data, err := classifierResource{classifier: c}.MarshalJSON()
			require.NoError(t, err)

			back, err := classifierActionsFromFriendly(data)
			require.NoError(t, err)
			got, err := services.UnmarshalProtoResource[*summarizerv1.Classifier](back, services.DisallowUnknown())
			require.NoError(t, err)

			gotActions := got.GetSpec().GetActions()
			require.Equal(t, summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED, gotActions.GetEmitAuditEvent())
			require.Equal(t, summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED, gotActions.GetFlagForReview())
			require.Equal(t, level, gotActions.GetRiskLevelFloor())
		})
	}
}

// Raw enum names, integers, and unknown values are rejected on input.
func TestClassifierActionsFromFriendlyRejectsInvalid(t *testing.T) {
	for _, tt := range []struct {
		name    string
		actions string
	}{
		{"toggle as enum string", `{"emit_audit_event":"CLASSIFIER_ACTION_MODE_ENABLED"}`},
		{"toggle as integer", `{"flag_for_review":1}`},
		{"risk level as enum string", `{"risk_level_floor":"RISK_LEVEL_HIGH"}`},
		{"risk level as integer", `{"risk_level_floor":3}`},
		{"risk level unknown", `{"risk_level_floor":"severe"}`},
	} {
		t.Run(tt.name, func(t *testing.T) {
			in := []byte(`{"kind":"classifier","version":"v1","metadata":{"name":"t"},` +
				`"spec":{"kinds":["ssh"],"criteria":"c","actions":` + tt.actions + `}}`)
			_, err := classifierActionsFromFriendly(in)
			require.Error(t, err)
		})
	}
}

func unmarshalActions(t *testing.T, data []byte) map[string]any {
	t.Helper()
	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	spec, ok := doc["spec"].(map[string]any)
	require.True(t, ok, "spec missing from marshaled classifier")
	actions, ok := spec["actions"].(map[string]any)
	require.True(t, ok, "spec.actions missing from marshaled classifier")
	return actions
}
