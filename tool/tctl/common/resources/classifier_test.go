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

// TestClassifierActionsMarshalBooleans verifies that "tctl get" renders the
// tri-state ClassifierActionMode toggles as booleans, leaves risk_level_floor as
// its enum, and omits unspecified toggles entirely.
func TestClassifierActionsMarshalBooleans(t *testing.T) {
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
	// risk_level_floor is a level, not a toggle, so it stays an enum string.
	require.Equal(t, "RISK_LEVEL_HIGH", actions["risk_level_floor"])
}

func TestClassifierActionsUnspecifiedOmitted(t *testing.T) {
	c := makeClassifierWithActions(summarizerv1.ClassifierActions_builder{
		EmitAuditEvent: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
		// FlagForReview is left unspecified.
	}.Build())

	data, err := classifierResource{classifier: c}.MarshalJSON()
	require.NoError(t, err)

	actions := unmarshalActions(t, data)
	require.Equal(t, true, actions["emit_audit_event"])
	_, ok := actions["flag_for_review"]
	require.False(t, ok, "an unspecified action toggle should be omitted, not printed")
}

// TestClassifierActionsRoundTrip verifies booleans survive a get -> create
// round-trip, and that the legacy enum-string form is still accepted on input.
func TestClassifierActionsRoundTrip(t *testing.T) {
	c := makeClassifierWithActions(summarizerv1.ClassifierActions_builder{
		EmitAuditEvent: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
		FlagForReview:  summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED,
		RiskLevelFloor: summarizerv1.RiskLevel_RISK_LEVEL_HIGH,
	}.Build())

	// "get" renders booleans...
	data, err := classifierResource{classifier: c}.MarshalJSON()
	require.NoError(t, err)

	// ...and "create" accepts them back.
	back, err := classifierActionsFromBool(data)
	require.NoError(t, err)
	got, err := services.UnmarshalProtoResource[*summarizerv1.Classifier](back, services.DisallowUnknown())
	require.NoError(t, err)

	gotActions := got.GetSpec().GetActions()
	require.Equal(t, summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED, gotActions.GetEmitAuditEvent())
	require.Equal(t, summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED, gotActions.GetFlagForReview())
	require.Equal(t, summarizerv1.RiskLevel_RISK_LEVEL_HIGH, gotActions.GetRiskLevelFloor())
}

// TestClassifierActionsFromBoolAcceptsEnumStrings verifies backward
// compatibility: a manifest still using the enum string form decodes unchanged.
func TestClassifierActionsFromBoolAcceptsEnumStrings(t *testing.T) {
	in := []byte(`{"kind":"classifier","version":"v1","metadata":{"name":"test"},` +
		`"spec":{"kinds":["ssh"],"criteria":"c",` +
		`"actions":{"emit_audit_event":"CLASSIFIER_ACTION_MODE_ENABLED"}}}`)

	out, err := classifierActionsFromBool(in)
	require.NoError(t, err)
	got, err := services.UnmarshalProtoResource[*summarizerv1.Classifier](out, services.DisallowUnknown())
	require.NoError(t, err)
	require.Equal(t,
		summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
		got.GetSpec().GetActions().GetEmitAuditEvent())
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
