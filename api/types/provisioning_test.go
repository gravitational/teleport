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
	"github.com/gogo/protobuf/proto"
	"github.com/gravitational/teleport/api/defaults"
	"testing"
	"time"

	"github.com/gravitational/trace"
	"github.com/stretchr/testify/require"
)

func TestNewProvisionToken(t *testing.T) {
	name := "foo"
	roles := SystemRoles{RoleNode}
	expires := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	tok, err := NewProvisionToken(name, roles, expires)
	require.NoError(t, err)
	require.Equal(t, &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV3{
			JoinMethod: JoinMethodToken,
			Roles:      roles,
		},
	}, tok)
}

func TestNewProvisionTokenFromSpec(t *testing.T) {
	name := "foo"
	expires := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	spec := ProvisionTokenSpecV3{
		Roles:      SystemRoles{RoleNop},
		JoinMethod: JoinMethodToken,
	}
	tok, err := NewProvisionTokenFromSpec(name, expires, spec)
	require.NoError(t, err)
	require.Equal(t, &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: spec,
	}, tok)
}

// ProvisionTokenV1 tests
func TestProvisionTokenV1_V3(t *testing.T) {
	roles := SystemRoles{RoleNop}
	name := "foo-tok"
	expires := time.Date(2000, 1, 1, 1, 1, 1, 1, time.UTC)
	v1 := ProvisionTokenV1{
		Roles:   roles,
		Token:   name,
		Expires: expires,
	}

	v3 := v1.V3()
	require.Equal(t, &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      name,
			Expires:   &expires,
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV3{
			Roles:      roles,
			JoinMethod: JoinMethodToken,
		},
	}, v3)
}

// ProvisionTokenV2 tests
func TestProvisionTokenV2_CheckAndSetDefaults(t *testing.T) {
	tests := []struct {
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
	}

	for _, tc := range tests {
		t.Run(tc.desc, func(t *testing.T) {
			err := tc.token.CheckAndSetDefaults()
			if tc.expectedErr != nil {
				require.ErrorAs(t, err, &tc.expectedErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, tc.token)
		})
	}
}

func validProvisionTokenV3(modifier func(p *ProvisionTokenV3)) *ProvisionTokenV3 {
	token := &ProvisionTokenV3{
		Kind:    KindToken,
		Version: V3,
		Metadata: Metadata{
			Name:      "foo",
			Namespace: defaults.Namespace,
		},
		Spec: ProvisionTokenSpecV3{
			Roles:      SystemRoles{RoleNop},
			JoinMethod: JoinMethodToken,
		},
	}
	if modifier != nil {
		modifier(token)
	}
	return token
}

// ProvisionTokenV3 tests
func TestProvisionTokenV3_CheckAndSetDefaults(t *testing.T) {
	tests := []struct {
		name  string
		token *ProvisionTokenV3
		// want indicates the token that should be present after the validation
		// has been called. If this is not provided, the original value of token
		// is used.
		want    *ProvisionTokenV3
		wantErr error
	}{
		{
			name:  "valid token",
			token: validProvisionTokenV3(nil),
		},
		{
			name: "invalid missing roles",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{}
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "invalid non-existent role",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{"supreme_leader"}
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "valid bot",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{RoleBot}
				p.Spec.BotName = "a_bot"
			}),
		},
		{
			name: "invalid missing bot name",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.Roles = SystemRoles{RoleBot}
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "invalid bot name set but not bot token",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.BotName = "set_by_mistake"
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "missing join method",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = ""
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "invalid join method",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = "ethereal-presence"
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "missing iam configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodIAM
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "missing ec2 confgiuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodEC2
			}),
			wantErr: &trace.BadParameterError{},
		},
		{
			name: "valid iam configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodIAM
				p.Spec.ProviderConfiguration = &ProvisionTokenSpecV3_IAM{
					IAM: &ProvisionTokenSpecV3AWSIAM{
						Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
							{
								Account: "foo",
							},
						},
					},
				}
			}),
		},
		{
			name: "valid ec2 configuration",
			token: validProvisionTokenV3(func(p *ProvisionTokenV3) {
				p.Spec.JoinMethod = JoinMethodEC2
				p.Spec.ProviderConfiguration = &ProvisionTokenSpecV3_EC2{
					EC2: &ProvisionTokenSpecV3AWSEC2{
						Allow: []*ProvisionTokenSpecV3AWSEC2_Rule{
							{
								Account: "foo",
							},
						},
						IIDTTL: NewDuration(time.Minute * 12),
					},
				}
			}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure we run check n set on a clone - so we can check
			// for no changes
			token := proto.Clone(tt.token).(*ProvisionTokenV3)
			err := token.CheckAndSetDefaults()
			if tt.wantErr != nil {
				require.ErrorAs(t, err, &tt.wantErr)
				return
			}
			require.NoError(t, err)

			want := tt.want
			if want == nil {
				want = tt.token
			}
			require.Equal(t, want, token)
		})
	}
}

func TestProvisionTokenV3_GetAllowRules(t *testing.T) {
	tests := []struct {
		name  string
		token ProvisionTokenV3
		want  []*TokenRule
	}{
		{
			name: "ec2",
			token: ProvisionTokenV3{
				Spec: ProvisionTokenSpecV3{
					JoinMethod: JoinMethodEC2,
					ProviderConfiguration: &ProvisionTokenSpecV3_EC2{
						EC2: &ProvisionTokenSpecV3AWSEC2{
							Allow: []*ProvisionTokenSpecV3AWSEC2_Rule{
								{
									Account: "foo",
									Regions: []string{"eu-west-666", "us-coast-612"},
									Role:    "a-role",
								},
								{
									Account: "bar",
									Regions: []string{"a-region"},
									Role:    "b-role",
								},
							},
						},
					},
				},
			},
			want: []*TokenRule{
				{
					AWSAccount: "foo",
					AWSRegions: []string{"eu-west-666", "us-coast-612"},
					AWSRole:    "a-role",
				},
				{
					AWSAccount: "bar",
					AWSRegions: []string{"a-region"},
					AWSRole:    "b-role",
				},
			},
		},
		{
			name: "iam",
			token: ProvisionTokenV3{
				Spec: ProvisionTokenSpecV3{
					JoinMethod: JoinMethodIAM,
					ProviderConfiguration: &ProvisionTokenSpecV3_IAM{
						IAM: &ProvisionTokenSpecV3AWSIAM{
							Allow: []*ProvisionTokenSpecV3AWSIAM_Rule{
								{
									Account: "foo",
									ARN:     "arn-amazon-foo",
								},
								{
									Account: "bar",
									ARN:     "arn-amazon-bar",
								},
							},
						},
					},
				},
			},
			want: []*TokenRule{
				{
					AWSAccount: "foo",
					AWSARN:     "arn-amazon-foo",
				},
				{
					AWSAccount: "bar",
					AWSARN:     "arn-amazon-bar",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules := tt.token.GetAllowRules()
			require.Equal(t, tt.want, rules)
		})
	}
}

func TestProvisionTokenV3_GetAWSIIDTTL(t *testing.T) {
	t.Run("unset", func(t *testing.T) {
		p := ProvisionTokenV3{
			Spec: ProvisionTokenSpecV3{},
		}
		require.Equal(t, Duration(0), p.GetAWSIIDTTL())
	})

	t.Run("set", func(t *testing.T) {
		duration := NewDuration(time.Second * 6)
		p := ProvisionTokenV3{
			Spec: ProvisionTokenSpecV3{
				ProviderConfiguration: &ProvisionTokenSpecV3_EC2{
					EC2: &ProvisionTokenSpecV3AWSEC2{
						IIDTTL: duration,
					},
				},
			},
		}
		got := p.GetAWSIIDTTL()
		require.Equal(t, duration, got)
	})
}
