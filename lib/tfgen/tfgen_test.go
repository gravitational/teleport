/*
 * Teleport
 * Copyright (C) 2025  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package tfgen_test

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"

	accessmonitoringrulesv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/accessmonitoringrules/v1"
	headerv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/header/v1"
	machineidv1 "github.com/gravitational/teleport/api/gen/proto/go/teleport/machineid/v1"
	"github.com/gravitational/teleport/api/types"
	"github.com/gravitational/teleport/api/types/accesslist"
	accesslistconv "github.com/gravitational/teleport/api/types/accesslist/convert/v1"
	"github.com/gravitational/teleport/api/types/header"
	"github.com/gravitational/teleport/api/types/trait"
	"github.com/gravitational/teleport/api/types/wrappers"
	"github.com/gravitational/teleport/lib/tfgen"
	"github.com/gravitational/teleport/lib/tfgen/transform"
	"github.com/gravitational/teleport/lib/utils/testutils/golden"
)

func TestGenerate_Role(t *testing.T) {
	t.Parallel()

	goldenTest(t, &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V8,
		Metadata: types.Metadata{
			Name: "gravitational-teleport-kube-access",
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubernetesLabels: types.Labels{
					"env":    []string{"staging", "dev"},
					"region": []string{"eu-west-1"},
				},
				NodeLabels: types.Labels{
					"foo":  []string{},
					"bar":  []string{"baz"},
					"team": []string{"a", "b", "c"},
				},
				KubeGroups: []string{"{{internal.kubernetes_groups}}"},
				KubeUsers:  []string{"{{internal.kubernetes_users}}"},
			},
		},
	},
		tfgen.WithFieldComment("metadata.labels", "Remember to add some labels!"))
}

func TestGenerate_Bot(t *testing.T) {
	t.Parallel()

	goldenTest(t,
		&machineidv1.Bot{
			Kind:    types.KindBot,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "gha-gravitational-teleport",
			},
			Spec: &machineidv1.BotSpec{
				Roles: []string{"gravitational-teleport-kube-access"},
				Traits: []*machineidv1.Trait{
					{
						Name:   "kubernetes_groups",
						Values: []string{"system:masters", "viewers"},
					},
				},
			},
		},
		tfgen.WithFieldTransform("spec.traits", transform.BotTraits),
	)
}

func TestGenerate_Token(t *testing.T) {
	t.Parallel()

	goldenTest(t,
		&types.ProvisionTokenV2{
			Kind:    types.KindToken,
			Version: types.V2,
			Metadata: types.Metadata{
				Name: "github-gravitational-teleport",
			},
			Spec: types.ProvisionTokenSpecV2{
				BotName:    "gha-gravitational-teleport",
				Roles:      []types.SystemRole{types.RoleBot},
				JoinMethod: types.JoinMethodGitHub,
				GitHub: &types.ProvisionTokenSpecV2GitHub{
					Allow: []*types.ProvisionTokenSpecV2GitHub_Rule{
						{
							Repository:      "graviational/teleport",
							RepositoryOwner: "gravitational",
						},
					},
				},
			},
		},
		tfgen.WithResourceType("teleport_provision_token"),
	)
}

func TestGenerate_AccessMonitoringRule(t *testing.T) {
	t.Parallel()

	goldenTest(t,
		&accessmonitoringrulesv1.AccessMonitoringRule{
			Kind:    types.KindAccessMonitoringRule,
			Version: types.V1,
			Metadata: &headerv1.Metadata{
				Name: "monitoring-rule",
			},
			Spec: &accessmonitoringrulesv1.AccessMonitoringRuleSpec{
				Subjects:     []string{"access_request"},
				Condition:    `access_request.spec.roles.contains("your_role_name")`,
				DesiredState: types.AccessMonitoringRuleStateReviewed,
				Notification: &accessmonitoringrulesv1.Notification{
					Name:       "slack",
					Recipients: []string{"#your-slack-channel"},
				},
				AutomaticReview: &accessmonitoringrulesv1.AutomaticReview{
					Integration: "builtin",
					Decision:    "APPROVED",
				},
				Schedules: map[string]*accessmonitoringrulesv1.Schedule{
					"default": {
						Time: &accessmonitoringrulesv1.TimeSchedule{
							Timezone: "America/Los_Angeles",
							Shifts: []*accessmonitoringrulesv1.TimeSchedule_Shift{
								{
									Weekday: "Monday",
									Start:   "00:00",
									End:     "23:59",
								},
								{
									Weekday: "Tuesday",
									Start:   "00:00",
									End:     "23:59",
								},
							},
						},
					},
				},
			},
		},
	)
}

func TestGenerate_User(t *testing.T) {
	t.Parallel()

	user, err := types.NewUser("bob")
	require.NoError(t, err)

	user.SetTraits(map[string][]string{
		"kubernetes_groups": []string{"viewers"},
	})

	goldenTest(t, user)
}

func TestGenerate_OIDCConnector(t *testing.T) {
	t.Parallel()

	connector, err := types.NewOIDCConnector("connector", types.OIDCConnectorSpecV3{
		ClientID:     "some-client",
		ClientSecret: "some-secret",
		ClaimsToRoles: []types.ClaimMapping{
			{Claim: "foo", Value: "bar", Roles: []string{"baz"}},
		},
		RedirectURLs: wrappers.Strings{"https://some-url"},
		MaxAge:       &types.MaxAge{Value: types.NewDuration(1 * time.Second)},
	})
	require.NoError(t, err)

	goldenTest(t, connector)
}

func TestGenerate_AccessList(t *testing.T) {
	t.Parallel()

	al, err := accesslist.NewAccessList(
		header.Metadata{Name: "some-access-list"},
		accesslist.Spec{
			Title:       "My Access List",
			Description: "An example access list for IaC",
			Owners: []accesslist.Owner{
				{Name: "llama", Description: "some description"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"granted-role"},
				Traits: trait.Traits{
					"trait1": []string{"value1", "value2"},
					"trait2": []string{"value1", "value2"},
				},
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"member-role"},
				Traits: trait.Traits{
					"trait1": []string{"value1", "value2"},
				},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
				Recurrence: accesslist.Recurrence{
					Frequency:  accesslist.ThreeMonths,
					DayOfMonth: accesslist.FirstDayOfMonth,
				},
			},
		},
	)
	require.NoError(t, err)

	alProto := tfgen.WrapHeaderResource(accesslistconv.ToProto(al))
	goldenTest(t, alProto)
}

func TestGenerate_AccessListMember(t *testing.T) {
	t.Parallel()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{Name: "alpaca"},
		accesslist.AccessListMemberSpec{
			AccessList: "some-access-list",
			Name:       "alpaca",
			Reason:     "some reason",
			AddedBy:    "bob",
			Joined:     time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
		},
	)
	require.NoError(t, err)

	memberProto := accesslistconv.ToMemberProto(member)

	goldenTest(t,
		tfgen.WrapHeaderResource(memberProto),
	)
}

func TestGenerate_WithOmitFields(t *testing.T) {
	t.Parallel()

	goldenTest(t, &types.RoleV6{
		Kind:    types.KindRole,
		SubKind: "some-subkind",
		Version: "remove me",
		Metadata: types.Metadata{
			Name:        "example-role",
			Description: "entire metadata should be removed",
		},
		Spec: types.RoleSpecV6{
			Deny: types.RoleConditions{Logins: []string{"remove deny field"}},
			Allow: types.RoleConditions{
				Logins:     []string{"remove me", "remove me"},
				KubeGroups: []string{"system:masters"},
				Request: &types.AccessRequestConditions{
					Roles:              []string{"remove me", "remove me"},
					SuggestedReviewers: []string{"reviewer1", "reviewer2"},
				},
			},
		},
	},
		// single field
		tfgen.WithOmitField("version"),
		// single field object
		tfgen.WithOmitField("metadata"),
		// one level nested field
		tfgen.WithOmitField("spec.deny"),
		// 2nd level nested field
		tfgen.WithOmitField("spec.allow.logins"),
		// multi-level nested field
		tfgen.WithOmitField("spec.allow.request.roles"),
		// no match, does nothing
		tfgen.WithOmitField("i.dont.exist"),
	)
}

func TestGenerate_AccessListMemberOmitField(t *testing.T) {
	t.Parallel()

	member, err := accesslist.NewAccessListMember(
		header.Metadata{Name: "alpaca", Description: "remove me"},
		accesslist.AccessListMemberSpec{
			AccessList:       "some-access-list",
			Name:             "alpaca",
			Joined:           time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
			AddedBy:          "remove me",
			Reason:           "remove me",
			IneligibleStatus: "remove me",
		},
	)
	require.NoError(t, err)

	memberProto := accesslistconv.ToMemberProto(member)
	goldenTest(t,
		tfgen.WrapHeaderResource(memberProto),
		tfgen.WithOmitField("spec.ineligible_status"),
		tfgen.WithOmitField("spec.reason"),
		tfgen.WithOmitField("spec.added_by"),
		tfgen.WithOmitField("header.metadata.description"),
		tfgen.WithOmitField("header.version"),
	)
}

func TestGenerate_OmitFieldFromNestedListItems(t *testing.T) {
	t.Parallel()

	goldenTest(t, &types.OktaImportRuleV1{
		ResourceHeader: types.ResourceHeader{
			Kind:    types.KindOktaImportRule,
			Version: types.V1,
			Metadata: types.Metadata{
				Name: "example-import-rule",
			},
		},
		Spec: types.OktaImportRuleSpecV1{
			Mappings: []*types.OktaImportRuleMappingV1{
				{
					Match: []*types.OktaImportRuleMatchV1{
						{AppIDs: []string{"app1"}, GroupIDs: []string{"remove me"}, AppNameRegexes: []string{"remove me"}},
						{AppIDs: []string{"app2"}, GroupIDs: []string{"remove me"}},
					},
					AddLabels: map[string]string{"remove": "field"},
				},
				{
					Match: []*types.OktaImportRuleMatchV1{
						{AppNameRegexes: []string{"remove me"}, AppIDs: []string{"app3"}, GroupIDs: []string{"remove me"}},
					},
					AddLabels: map[string]string{"remove": "field"},
				},
			},
		},
	},
		tfgen.WithOmitField("spec.mappings.add_labels"),
		// Remove some fields in nested list.
		tfgen.WithOmitField("spec.mappings.match.group_ids"),
		tfgen.WithOmitField("spec.mappings.match.app_name_regexes"),
		// No match, does nothing
		tfgen.WithOmitField("spec.mappings.match.i.dont.exist"),
	)
}

func TestGenerate_AccessListOmitFields(t *testing.T) {
	t.Parallel()

	al, err := accesslist.NewAccessList(
		header.Metadata{Name: "some-access-list"},
		accesslist.Spec{
			Title:       "My Access List",
			Description: "An example access list for IaC",
			Owners: []accesslist.Owner{
				{Name: "llama", Description: "remove me"},
				{Name: "alpaca", Description: "remove me"},
			},
			Grants: accesslist.Grants{
				Roles: []string{"granted-role"},
			},
			MembershipRequires: accesslist.Requires{
				Roles: []string{"member-role"},
			},
			Audit: accesslist.Audit{
				NextAuditDate: time.Date(2023, 02, 02, 0, 0, 0, 0, time.UTC),
				Recurrence: accesslist.Recurrence{
					Frequency:  accesslist.ThreeMonths,
					DayOfMonth: accesslist.FirstDayOfMonth,
				},
			},
		},
	)
	require.NoError(t, err)

	alProto := tfgen.WrapHeaderResource(accesslistconv.ToProto(al))
	goldenTest(t, alProto,
		tfgen.WithOmitField("spec.owners.description"),
		tfgen.WithOmitField("spec.audit.recurrence"),
		tfgen.WithOmitField("spec.owners.ineligible_status"),
		tfgen.WithOmitField("spec.grants"),
	)
}

func TestGenerate_DependsOn_Single(t *testing.T) {
	t.Parallel()

	goldenTest(t, &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "example-role",
		},
		Spec: types.RoleSpecV6{Allow: types.RoleConditions{KubeGroups: []string{"system:masters"}}},
	},
		tfgen.WithDependsOn("teleport_role.some-role-id"),
	)
}

func TestGenerate_DependsOn_Multiple(t *testing.T) {
	t.Parallel()

	goldenTest(t, &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "example-role",
		},
		Spec: types.RoleSpecV6{Allow: types.RoleConditions{KubeGroups: []string{"system:masters"}}},
	},
		tfgen.WithDependsOn("teleport_role.some-role-id-1"),
		tfgen.WithDependsOn("teleport_role.some-role-id-2"),
		tfgen.WithDependsOn("teleport_role.some-role-id-3"),
	)
}

func TestGenerate_ResourceComment(t *testing.T) {
	t.Parallel()

	goldenTest(t, &types.RoleV6{
		Kind:    types.KindRole,
		Version: types.V7,
		Metadata: types.Metadata{
			Name: "example-role",
		},
		Spec: types.RoleSpecV6{
			Allow: types.RoleConditions{
				KubeGroups: []string{"system:masters"},
			},
		},
	},
		tfgen.WithResourceBlockComment("This resource was generated by Teleport."),
	)
}

func TestSanitizeResourceName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "Alpaca123@goteleport.com", want: "Alpaca123_goteleport_com"},
		{input: "simple", want: "simple"},
		{input: "with spaces", want: "with_spaces"},
		{input: "`", want: "_"},
		{input: `\|{}[]<>,.?/~!@#$%^&*()_+='`, want: "___________________________"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			require.Equal(t, tt.want, tfgen.SanitizeResourceName(tt.input))
		})
	}
}

func goldenTest(t *testing.T, resource tfgen.Resource, opts ...tfgen.GenerateOpt) {
	t.Helper()

	tf, err := tfgen.Generate(resource, opts...)
	require.NoError(t, err)

	if golden.ShouldSet() {
		golden.Set(t, tf)
	}
	require.Empty(t,
		cmp.Diff(
			string(golden.Get(t)),
			string(tf),
		),
	)
}
