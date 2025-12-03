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
