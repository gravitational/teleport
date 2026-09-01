package ui

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/testing/protocmp"

	summarizerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/summarizer/v1"
	"github.com/gravitational/teleport/api/types/summarizer"
)

func TestClassifierJSONRoundTrip(t *testing.T) {
	in := summarizer.NewClassifier("destructive", summarizerv1.ClassifierSpec_builder{
		Kinds:    []string{"ssh", "k8s"},
		Filter:   `equals(resource.metadata.labels["env"], "prod")`,
		Criteria: "The session destroyed persistent data.",
		Actions: summarizerv1.ClassifierActions_builder{
			RiskLevelFloor: summarizerv1.RiskLevel_RISK_LEVEL_MEDIUM,
			FlagForReview:  summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_DISABLED,
		}.Build(),
		Disabled: true,
		Rules: []*summarizerv1.ClassifierRule{
			summarizerv1.ClassifierRule_builder{
				Name:     "backups",
				Criteria: "The destroyed resource was a backup or snapshot.",
				Actions: summarizerv1.ClassifierActions_builder{
					RiskLevelFloor: summarizerv1.RiskLevel_RISK_LEVEL_CRITICAL,
					EmitAuditEvent: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
				}.Build(),
			}.Build(),
			summarizerv1.ClassifierRule_builder{
				Name:   "unjustified",
				Filter: `equals(user.metadata.name, "alice")`,
				Actions: summarizerv1.ClassifierActions_builder{
					FlagForReview: summarizerv1.ClassifierActionMode_CLASSIFIER_ACTION_MODE_ENABLED,
				}.Build(),
			}.Build(),
		},
	}.Build())

	// Go through JSON rather than comparing structs, since the JSON is what the frontend sees and sends back.
	data, err := json.Marshal(MakeClassifier(in))
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	require.Equal(t, true, doc["disabled"])
	require.Equal(t, map[string]any{"risk_level_floor": "medium", "flag_for_review": false}, doc["actions"])
	rules := doc["rules"].([]any)
	require.Len(t, rules, 2)
	require.Equal(t, map[string]any{"risk_level_floor": "critical", "emit_audit_event": true},
		rules[0].(map[string]any)["actions"])
	require.Equal(t, `equals(user.metadata.name, "alice")`, rules[1].(map[string]any)["filter"])
	require.Equal(t, map[string]any{"flag_for_review": true}, rules[1].(map[string]any)["actions"])

	var decoded Classifier
	require.NoError(t, json.Unmarshal(data, &decoded))
	out, err := decoded.ToProto()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(in, out, protocmp.Transform()))
}

// Unknown enum strings are rejected instead of silently becoming "unspecified", which would drop the constraint.
func TestClassifierToProtoRejectsUnknownEnums(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		err  string
	}{
		{
			name: "risk level",
			in:   `{"name":"c","kinds":["ssh"],"criteria":"x","actions":{"risk_level_floor":"severe"}}`,
			err:  `spec.actions.risk_level_floor must be one of "low", "medium", "high", "critical"`,
		},
		{
			name: "rule risk level",
			in:   `{"name":"c","kinds":["ssh"],"criteria":"x","rules":[{"name":"r"},{"name":"s","actions":{"risk_level_floor":"RISK_LEVEL_HIGH"}}]}`,
			err:  `spec.rules[1].actions.risk_level_floor must be one of`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var decoded Classifier
			require.NoError(t, json.Unmarshal([]byte(tc.in), &decoded))
			_, err := decoded.ToProto()
			require.ErrorContains(t, err, tc.err)
		})
	}
}

// Absent optional blocks stay absent instead of turning into empty objects.
func TestClassifierOmitsUnsetBlocks(t *testing.T) {
	in := summarizer.NewClassifier("plain", summarizerv1.ClassifierSpec_builder{
		Kinds:    []string{"ssh"},
		Criteria: "Anything at all.",
	}.Build())

	data, err := json.Marshal(MakeClassifier(in))
	require.NoError(t, err)

	var doc map[string]any
	require.NoError(t, json.Unmarshal(data, &doc))
	for _, key := range []string{"actions", "rules", "disabled"} {
		require.NotContains(t, doc, key)
	}

	var decoded Classifier
	require.NoError(t, json.Unmarshal(data, &decoded))
	out, err := decoded.ToProto()
	require.NoError(t, err)
	require.Empty(t, cmp.Diff(in, out, protocmp.Transform()))
}
