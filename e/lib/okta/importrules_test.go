package okta

import (
	"context"
	"testing"

	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/require"

	"github.com/gravitational/teleport/api/types"
)

func TestBuildOktaImportRuleMappings(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	ap := newTestAccessPoint(t, clockwork.NewRealClock())

	svc := &Service{
		accessPoint: ap,
	}

	addIR(t, ap,
		newIR(t, "ir1", 100,
			newIRMapping(
				map[string]string{
					"label1": "value1",
				},
				newIRMatchGroups("group1"),
				newIRMatchApps("app1"))),
	)
	require.NoError(t, svc.buildImportRuleMappings(ctx))

	require.Equal(t, map[string]prioritizedLabels{
		"group1": {
			newPriorityAndLabel(100, map[string]string{"label1": "value1"}),
		},
	}, svc.groupIRMapping)
	require.Equal(t, map[string]prioritizedLabels{
		"app1": {
			newPriorityAndLabel(100, map[string]string{"label1": "value1"}),
		},
	}, svc.applicationIRMapping)

	addIR(t, ap,
		newIR(t, "ir2", 99,
			newIRMapping(
				map[string]string{
					"label1": "value2",
				},
				newIRMatchGroups("group1"),
				newIRMatchApps("app2")),
			newIRMapping(
				map[string]string{
					"label2": "value1",
				},
				newIRMatchGroups("group2"),
				newIRMatchApps("app1")),
		))

	require.NoError(t, svc.buildImportRuleMappings(ctx))

	require.Equal(t, map[string]prioritizedLabels{
		"group1": {
			newPriorityAndLabel(99, map[string]string{"label1": "value2"}),
			newPriorityAndLabel(100, map[string]string{"label1": "value1"}),
		},
		"group2": {
			newPriorityAndLabel(99, map[string]string{"label2": "value1"}),
		},
	}, svc.groupIRMapping)
	require.Equal(t, map[string]prioritizedLabels{
		"app1": {
			newPriorityAndLabel(99, map[string]string{"label2": "value1"}),
			newPriorityAndLabel(100, map[string]string{"label1": "value1"}),
		},
		"app2": {
			newPriorityAndLabel(99, map[string]string{"label1": "value2"}),
		},
	}, svc.applicationIRMapping)
}

func TestGetLabels(t *testing.T) {
	t.Parallel()

	groupID := "group1"
	groupName := "group1Name"
	appID := "app1"
	appName := "app1Name"

	tests := []struct {
		name                string
		importRules         []types.OktaImportRule
		expectedGroupLabels map[string]string
		expectedAppLabels   map[string]string
	}{
		{
			name: "no overrides",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100, newIRMapping(map[string]string{"label1": "value1"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
				newIR(t, "ir2", 101, newIRMapping(map[string]string{"label2": "value2"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
				newIR(t, "ir3", 101, newIRMapping(map[string]string{"label3": "value3"}, newIRMatchGroupNameRegex("^group.*$"), newIRMatchAppNameRegex("app*"))),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value1",
				"label2": "value2",
				"label3": "value3",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
				"label2": "value2",
				"label3": "value3",
			},
		},
		{
			name: "override group",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100, newIRMapping(map[string]string{"label1": "value1"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
				newIR(t, "ir2", 101, newIRMapping(map[string]string{"label1": "value2"}, newIRMatchGroups(groupID))),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value2",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
			},
		},
		{
			name: "override group (regex)",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100, newIRMapping(map[string]string{"label1": "value1"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
				newIR(t, "ir2", 101, newIRMapping(map[string]string{"label1": "value2"}, newIRMatchGroupNameRegex("^group.*$"))),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value2",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
			},
		},
		{
			name: "shadowed values",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100, newIRMapping(map[string]string{"label1": "value1"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
				newIR(t, "ir2", 99, newIRMapping(map[string]string{"label1": "value0"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value1",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
			},
		},
		{
			name: "shadowed values (regex)",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100, newIRMapping(map[string]string{"label1": "value1"}, newIRMatchGroups(groupID), newIRMatchApps(appID))),
				newIR(t, "ir2", 99, newIRMapping(map[string]string{"label1": "value0"}, newIRMatchGroupNameRegex("^group.*$"), newIRMatchApps(appID))),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value1",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
			},
		},
		{
			name: "interpolated values",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 99, newIRMapping(map[string]string{"label1": "$0"}, newIRMatchGroupNameRegex("^group.*$"), newIRMatchApps(appID))),
				newIR(t, "ir2", 99, newIRMapping(map[string]string{"label2": "$0"}, newIRMatchAppNameRegex("^app.*$"))),
				newIR(t, "ir3", 99, newIRMapping(map[string]string{"label3": "$0 $1 $2"}, newIRMatchAppNameRegex(`^app(\d+)(.*)$`))),
			},
			expectedGroupLabels: map[string]string{
				"label1": groupName,
			},
			expectedAppLabels: map[string]string{
				// Interpolation doesn't work for a direct ID map
				"label1": "$0",
				"label2": appName,
				"label3": "app1Name 1 Name",
			},
		},
		{
			name: "multiple labels",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100,
					newIRMapping(
						map[string]string{
							"label1": "value1",
							"label2": "value1",
							"label3": "value1",
						},
						newIRMatchGroups(groupID), newIRMatchApps(appID),
					),
				),
				newIR(t, "ir2", 101,
					newIRMapping(
						map[string]string{
							"label2": "value2",
							"label3": "value2",
						},
						newIRMatchGroups(groupID), newIRMatchApps(appID),
					),
				),
				newIR(t, "ir3", 103,
					newIRMapping(
						map[string]string{
							"label3": "value3",
						},
						newIRMatchGroups(groupID), newIRMatchApps(appID),
					),
				),
				newIR(t, "ir4", 99,
					newIRMapping(
						map[string]string{
							"label1": "value0",
							"label4": "value0",
						},
						newIRMatchGroups(groupID),
					),
				),
				newIR(t, "ir5", 104,
					newIRMapping(
						map[string]string{
							"label5": "value5",
						},
						newIRMatchAppNameRegex("^app.*$"),
					),
				),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value1",
				"label2": "value2",
				"label3": "value3",
				"label4": "value0",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
				"label2": "value2",
				"label3": "value3",
				"label5": "value5",
			},
		},
		{
			name: "wildcards",
			importRules: []types.OktaImportRule{
				newIR(t, "ir1", 100,
					newIRMapping(
						map[string]string{
							"label1": "value1",
							"label2": "value1",
							"label3": "value1",
						},
						newIRMatchAppNameRegex("*"), newIRMatchGroupNameRegex("*"),
					),
				),
			},
			expectedGroupLabels: map[string]string{
				"label1": "value1",
				"label2": "value1",
				"label3": "value1",
			},
			expectedAppLabels: map[string]string{
				"label1": "value1",
				"label2": "value1",
				"label3": "value1",
			},
		},
		{
			name:                "no rules",
			expectedGroupLabels: map[string]string{},
			expectedAppLabels:   map[string]string{},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ctx := context.Background()

			ap := newTestAccessPoint(t, clockwork.NewRealClock())
			for _, importRule := range test.importRules {
				addIR(t, ap, importRule)
			}

			svc := &Service{accessPoint: ap}
			svc.buildImportRuleMappings(ctx)

			labels, err := svc.getGroupLabels(groupID, groupName)
			require.NoError(t, err)
			require.Equal(t, test.expectedGroupLabels, labels)
			labels, err = svc.getApplicationLabels(appID, appName)
			require.NoError(t, err)
			require.Equal(t, test.expectedAppLabels, labels)
		})
	}
}

func addIR(t *testing.T, ap *testAccessPoint, ir types.OktaImportRule) {
	ctx := context.Background()
	_, err := ap.CreateOktaImportRule(ctx, ir)
	require.NoError(t, err)
}

func newIR(t *testing.T, name string, priority int32, mappings ...*types.OktaImportRuleMappingV1) types.OktaImportRule {
	ir, err := types.NewOktaImportRule(
		types.Metadata{
			Name: name,
		},
		types.OktaImportRuleSpecV1{
			Priority: priority,
			Mappings: mappings,
		},
	)
	require.NoError(t, err)
	return ir
}

func newIRMapping(labels map[string]string, matches ...*types.OktaImportRuleMatchV1) *types.OktaImportRuleMappingV1 {
	return &types.OktaImportRuleMappingV1{
		AddLabels: labels,
		Match:     matches,
	}
}

func newIRMatchApps(appIDs ...string) *types.OktaImportRuleMatchV1 {
	return &types.OktaImportRuleMatchV1{
		AppIDs: appIDs,
	}
}

func newIRMatchGroups(groupIDs ...string) *types.OktaImportRuleMatchV1 {
	return &types.OktaImportRuleMatchV1{
		GroupIDs: groupIDs,
	}
}

func newIRMatchAppNameRegex(regex string) *types.OktaImportRuleMatchV1 {
	return &types.OktaImportRuleMatchV1{
		AppNameRegexes: []string{regex},
	}
}

func newIRMatchGroupNameRegex(regex string) *types.OktaImportRuleMatchV1 {
	return &types.OktaImportRuleMatchV1{
		GroupNameRegexes: []string{regex},
	}
}
