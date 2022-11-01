/*
Copyright 2022 Gravitational, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package types

import (
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestProvisionTokenV2_CheckAndSetDefaults(t *testing.T) {
	testcases := []struct {
		desc        string
		token       *ProvisionTokenV2
		expected    *ProvisionTokenV2
		expectedErr error
	}{
		{
			desc:        "empty",
			token:       &ProvisionTokenV2{},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "missing roles",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "invalid role",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles: []SystemRole{RoleNode, "not a role"},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "simple token",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles: []SystemRole{RoleNode},
				},
			},
			expected: &ProvisionTokenV2{
				Kind:    "token",
				Version: "v2",
				Metadata: Metadata{
					Name:      "test",
					Namespace: "default",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "token",
				},
			},
		},
		{
			desc: "implicit ec2 method",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles: []SystemRole{RoleNode},
					Allow: []*TokenRule{
						{
							AWSAccount: "1234",
							AWSRole:    "1234/role",
							AWSRegions: []string{"us-west-2"},
						},
					},
				},
			},
			expected: &ProvisionTokenV2{
				Kind:    "token",
				Version: "v2",
				Metadata: Metadata{
					Name:      "test",
					Namespace: "default",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow: []*TokenRule{
						{
							AWSAccount: "1234",
							AWSRole:    "1234/role",
							AWSRegions: []string{"us-west-2"},
						},
					},
					AWSIIDTTL: Duration(5 * time.Minute),
				},
			},
		},
		{
			desc: "explicit ec2 method",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow:      []*TokenRule{{AWSAccount: "1234"}},
				},
			},
			expected: &ProvisionTokenV2{
				Kind:    "token",
				Version: "v2",
				Metadata: Metadata{
					Name:      "test",
					Namespace: "default",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow:      []*TokenRule{{AWSAccount: "1234"}},
					AWSIIDTTL:  Duration(5 * time.Minute),
				},
			},
		},
		{
			desc: "ec2 method no allow rules",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "ec2 method with aws_arn",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow: []*TokenRule{
						{
							AWSAccount: "1234",
							AWSARN:     "1234",
						},
					},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "ec2 method empty rule",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow:      []*TokenRule{{}},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "iam method",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow:      []*TokenRule{{AWSAccount: "1234"}},
				},
			},
			expected: &ProvisionTokenV2{
				Kind:    "token",
				Version: "v2",
				Metadata: Metadata{
					Name:      "test",
					Namespace: "default",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "ec2",
					Allow:      []*TokenRule{{AWSAccount: "1234"}},
					AWSIIDTTL:  Duration(5 * time.Minute),
				},
			},
		},
		{
			desc: "iam method with aws_role",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "iam",
					Allow: []*TokenRule{
						{
							AWSAccount: "1234",
							AWSRole:    "1234/role",
						},
					},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "iam method with aws_regions",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: "iam",
					Allow: []*TokenRule{
						{
							AWSAccount: "1234",
							AWSRegions: []string{"us-west-2"},
						},
					},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "circleci valid",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: JoinMethodCircleCI,
					CircleCI: &ProvisionTokenSpecV2CircleCI{
						OrganizationID: "foo",
						Allow: []*ProvisionTokenSpecV2CircleCI_Rule{
							{
								ProjectID: "foo",
								ContextID: "bar",
							},
						},
					},
				},
			},
		},
		{
			desc: "circleci and no allow",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: JoinMethodCircleCI,
					CircleCI: &ProvisionTokenSpecV2CircleCI{
						OrganizationID: "foo",
					},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "circleci and no org id",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: JoinMethodCircleCI,
					CircleCI: &ProvisionTokenSpecV2CircleCI{
						Allow: []*ProvisionTokenSpecV2CircleCI_Rule{
							{
								ProjectID: "foo",
							},
						},
					},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
		{
			desc: "circleci allow rule blank",
			token: &ProvisionTokenV2{
				Metadata: Metadata{
					Name: "test",
				},
				Spec: ProvisionTokenSpecV2{
					Roles:      []SystemRole{RoleNode},
					JoinMethod: JoinMethodCircleCI,
					CircleCI: &ProvisionTokenSpecV2CircleCI{
						Allow: []*ProvisionTokenSpecV2CircleCI_Rule{
							{},
						},
					},
				},
			},
			expectedErr: &trace.BadParameterError{},
		},
	}

	for _, tc := range testcases {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.token.CheckAndSetDefaults()
			if tc.expectedErr != nil {
				require.ErrorAs(t, err, &tc.expectedErr)
				return
			}
			require.NoError(t, err)
			if tc.expected != nil {
				require.Equal(t, tc.token, tc.expected)
			}
		})
	}
}
